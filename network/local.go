package network

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"
)

func NewLocalRouter(sid *ServerIdentity) *Router {
	return NewRouter(sid, NewLocalHost(sid))
}

// localConnStore_ keeps reference to all opened local connections
// It also keeps tracks of who is "listening", so it's possible to mimics
// Conn & Listener.
type localConnStore_ struct {
	conns map[endpoints]*LocalConn
	sync.Mutex
	listening map[string]func(Conn)
}

// endpoints represents the two end points of a connection
type endpoints struct {
	local  string
	remote string
}

// reverse simply reverse the endpoints from local <-> remote
func (e *endpoints) reverse() endpoints {
	return endpoints{
		local:  e.remote,
		remote: e.local,
	}
}

var localConnStore = &localConnStore_{
	conns:     make(map[endpoints]*LocalConn),
	listening: make(map[string]func(Conn)),
}

// Put takes a new local connection object and stores it
func (ccc *localConnStore_) Put(c *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.conns[c.endpoints] = c
}

// Del removes the local connection object
func (ccc *localConnStore_) Del(c *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.conns, c.endpoints)
}

// Islistening returns true if the remote address is listening "virtually"
func (ccc *localConnStore_) IsListening(remote string) bool {
	ccc.Lock()
	defer ccc.Unlock()
	_, ok := ccc.listening[remote]
	return ok
}

// Listening put the address as "listening" mode. If a user connects to this
// addr, this function will be called.
func (ccc *localConnStore_) Listening(addr string, fn func(Conn)) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.listening[addr] = fn
}

// StopListening remove the address from the "listening" mode
func (ccc *localConnStore_) StopListening(addr string) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.listening, addr)
}

var errNotListening = errors.New("Remote address is not listening")

// Connect will check if the remote address is listening, if yes it creates
// the two connections, and launch the listening function in a go routine.
// It returns the outgoing connection with any error.
func (ccc *localConnStore_) Connect(local, remote string) (*LocalConn, error) {
	ccc.Lock()
	defer ccc.Unlock()

	fn, ok := ccc.listening[remote]
	if !ok {
		return nil, errNotListening
	}

	outgoing := newLocalConn(local, remote)
	incoming := newLocalConn(remote, local)

	ccc.conns[outgoing.endpoints] = outgoing
	ccc.conns[incoming.endpoints] = incoming

	go fn(incoming)
	return outgoing, nil
}

// Send will get the connection denoted by this endpoint and will call queueMsg
// with the packet as argument on it. It returns ErrClosed if it does not find
// the connection.
func (ccc *localConnStore_) Send(e endpoints, nm Packet) error {
	ccc.Lock()
	defer ccc.Unlock()
	c, ok := ccc.conns[e]
	if !ok {
		return ErrClosed
	}

	c.queueMsg(nm)
	return nil
}

// Close will get the connection denoted by this endpoint and will Close it if
// present.
func (ccc *localConnStore_) Close(local *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	// delete this conn
	delete(ccc.conns, local.endpoints)
	// and delete the remote one + close it
	remote, ok := ccc.conns[local.reverse()]
	if !ok {
		return
	}
	delete(ccc.conns, local.reverse())
	remote.closeLocal()
}

// Len returns how many local connections is there
func (ccc *localConnStore_) Len() int {
	ccc.Lock()
	defer ccc.Unlock()
	return len(ccc.conns)
}

// ChannConn is a connection that send and receive messages through channels
type LocalConn struct {
	endpoints

	// contains all pending messages to be retrievied by Receive
	queue []Packet
	// synchronize operations for queuing and retrieving messages
	cond *sync.Cond

	// to signal we are quitting
	closed    bool
	closedMut sync.Mutex

	mut sync.Mutex
}

// newLocalConn simply init the fields of a LocalConn but do not try to
// connect. It should not be used as-is, most user wants to call NewLocalConn.
func newLocalConn(local, remote string) *LocalConn {
	return &LocalConn{
		endpoints: endpoints{
			remote: remote,
			local:  local,
		},
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

// Returns a new channel connection from local to remote
// Mimics the behavior of NewTCPConn => tries connecting right away.
func NewLocalConn(local, remote string) (*LocalConn, error) {
	c, err := localConnStore.Connect(local, remote)
	return c, err
}

func (cc LocalConn) Send(ctx context.Context, msg Body) error {

	var body Body
	var val = reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	body = val.Interface()

	var typ = TypeFromData(body)
	nm := Packet{
		MsgType: typ,
		Msg:     body,
	}
	err := localConnStore.Send(cc.reverse(), nm)
	return err
}

func (cc *LocalConn) queueMsg(p Packet) {
	cc.cond.L.Lock()
	cc.queue = append(cc.queue, p)
	cc.cond.L.Unlock()
	cc.cond.Signal()
}

func (cc *LocalConn) Receive(ctx context.Context) (Packet, error) {
	cc.cond.L.Lock()
	for len(cc.queue) == 0 {
		if cc.isClosed() {
			cc.cond.L.Unlock()
			return EmptyApplicationMessage, ErrClosed
		}
		cc.cond.Wait()
	}
	nm := cc.queue[0]
	cc.queue = cc.queue[1:]
	cc.cond.L.Unlock()
	return nm, nil
}

func (cc *LocalConn) isClosed() bool {
	cc.closedMut.Lock()
	defer cc.closedMut.Unlock()
	return cc.closed
}

// closing sets "closed" field to true
func (cc *LocalConn) closing() {
	cc.closedMut.Lock()
	defer cc.closedMut.Unlock()
	cc.closed = true
}

func (cc *LocalConn) Local() string {
	return cc.local
}

func (cc *LocalConn) Remote() string {
	return cc.remote
}

// Close will remove this connection from the store, will signal to futur
// use of Receive to return with an error
// and will call the Close on the other side.
func (cc *LocalConn) Close() error {
	cc.closeLocal()
	// close the remote conn also
	localConnStore.Close(cc)
	return nil
}

func (cc *LocalConn) closeLocal() {
	cc.closing()
	// signal so Receive() is waken up (if it's called)
	cc.cond.Signal()
}

func (cc *LocalConn) Rx() uint64 {
	return 0
}

func (cc *LocalConn) Tx() uint64 {
	return 0
}

func (cc *LocalConn) Type() ConnType {
	return Local
}

type LocalListener struct {
	// addr is the addr we're listening to + mut
	addr string
	// are we listening or not
	listening bool

	sync.Mutex

	// quit is used to stop the listening routine
	quit chan bool
}

func NewLocalListener() *LocalListener {
	return &LocalListener{
		quit: make(chan bool),
	}
}

func (ll *LocalListener) Listen(addr string, fn func(Conn)) error {
	ll.Lock()
	ll.addr = addr
	ll.listening = true
	ll.Unlock()
	localConnStore.Listening(addr, fn)
	<-ll.quit
	return nil
}

func (ll *LocalListener) Stop() error {
	ll.Lock()
	defer ll.Unlock()
	addr := ll.addr
	localConnStore.StopListening(addr)
	if ll.listening {
		ll.quit <- true
	}
	return nil
}

func (ll *LocalListener) IncomingType() ConnType {
	return Local
}

type LocalHost struct {
	id *ServerIdentity
	*LocalListener
}

func NewLocalHost(sid *ServerIdentity) *LocalHost {
	return &LocalHost{
		id:            sid,
		LocalListener: NewLocalListener(),
	}
}

func (lh *LocalHost) Connect(sid *ServerIdentity) (Conn, error) {
	for i := 0; i < MaxRetry; i++ {
		c, err := NewLocalConn(lh.id.Address.NetworkAddress(), sid.Address.NetworkAddress())
		if err == nil {
			return c, nil
		}
		time.Sleep(WaitRetry)
	}
	return nil, errors.New("Could not connect...")

}

func NewLocalClient() *Client {
	fn := func(own, remote *ServerIdentity) (Conn, error) {
		return NewLocalConn(own.Address.NetworkAddress(), remote.Address.NetworkAddress())
	}
	return newClient(fn)
}

/*// GetStatus implements the Host interface*/
//func (l *chanHost) GetStatus() Status {
//m := make(map[string]string)
//m["Connections"] = strings.Join(l.conns.Get(), "\n")
//m["Host"] = l.Address()
//m["Total"] = strconv.Itoa(l.conns.Len())
//m["Packets_Received"] = strconv.FormatUint(0, 10)
//m["Packets_Sent"] = strconv.FormatUint(0, 10)
//return m
//}
