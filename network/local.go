package network

import (
	"context"
	"errors"
	"log"
	"reflect"
	"sync"
)

// localConnStore_ keeps reference to all opened local connections
// It also keeps tracks of who is "listening", so it's possible to mimics
// Conn & Listener.
type localConnStore_ struct {
	conns map[string]*LocalConn
	sync.Mutex
	listening map[string]func(Conn)
	listMut   sync.Mutex
}

var localConnStore = &localConnStore_{
	conns:     make(map[string]*LocalConn),
	listening: make(map[string]func(Conn)),
}

// Get return the remote connection object
func (ccc *localConnStore_) Get(remote string) (*LocalConn, bool) {
	ccc.Lock()
	defer ccc.Unlock()
	c, ok := ccc.conns[remote]
	return c, ok
}

// Put takes a new local connection object and stores it
func (ccc *localConnStore_) Put(c *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.conns[c.local] = c
}

// Del removes the local connection object
func (ccc *localConnStore_) Del(c *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.conns, c.local)
}

// Islistening returns true if the remote address is listening "virtually"
func (ccc *localConnStore_) IsListening(remote string) bool {
	ccc.listMut.Lock()
	defer ccc.listMut.Unlock()
	_, ok := ccc.listening[remote]
	return ok
}

// Listening put the address as "listening" mode. If a user connects to this
// addr, this function will be called.
func (ccc *localConnStore_) Listening(addr string, fn func(Conn)) {
	ccc.listMut.Lock()
	defer ccc.listMut.Unlock()
	ccc.listening[addr] = fn
}

// StopListening remove the address from the "listening" mode
func (ccc *localConnStore_) StopListening(addr string) {
	ccc.listMut.Lock()
	defer ccc.listMut.Unlock()
	delete(ccc.listening, addr)
}

// Connect will check if the remote address is listening, if yes it creates
// the two connections, and launch the listening function in a go routine.
// It returns the outgoing connection with any error.
func (ccc *localConnStore_) Connect(local, remote string) (*LocalConn, error) {
	ccc.listMut.Lock()
	fn, ok := ccc.listening[remote]
	ccc.listMut.Unlock()
	if !ok {
		return nil, errors.New("Remote address is not listening")
	}

	outgoing := newLocalConn(local, remote)
	incoming := newLocalConn(remote, local)

	ccc.Put(outgoing)
	ccc.Put(incoming)

	go fn(incoming)
	return outgoing, nil

}

// Len returns how many local connections is there
func (ccc *localConnStore_) Len() int {
	ccc.Lock()
	defer ccc.Unlock()
	return len(ccc.conns)
}

// ChannConn is a connection that send and receive messages through channels
type LocalConn struct {
	// remote is the string representing the other end of the connection
	remote string
	local  string

	// contains all pending messages to be retrievied by Receive
	queue []Packet
	// synchronize operations for queuing and retrieving messages
	cond *sync.Cond

	// to signal we are quitting
	closed    bool
	closedMut sync.Mutex
}

// newLocalConn simply init the fields of a LocalConn but do not try to
// connect. It should not be used as-is, most user wants to call NewLocalConn.
func newLocalConn(local, remote string) *LocalConn {
	return &LocalConn{
		remote: remote,
		local:  local,
		cond:   sync.NewCond(&sync.Mutex{}),
	}
}

// Returns a new channel connection from local to remote
// Mimics the behavior of NewTCPConn => tries connecting right away.
func NewLocalConn(local, remote string) (*LocalConn, error) {
	return localConnStore.Connect(local, remote)
}

func (cc *LocalConn) Send(ctx context.Context, msg Body) error {
	c, ok := localConnStore.Get(cc.remote)
	if !ok {
		return ErrClosed
	}

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

	log.Print("cc.local=", cc.local, " cc.remote=", cc.remote, " => c.local=", c.local, " c.remote=", c.remote)
	c.queueMsg(nm)
	return nil
}

func (cc *LocalConn) queueMsg(p Packet) {
	cc.cond.L.Lock()
	cc.queue = append(cc.queue, p)
	cc.cond.L.Unlock()
	cc.cond.Signal()
	log.Print(cc.local, " Signal() done for ", cc.remote)
}

func (cc *LocalConn) Receive(ctx context.Context) (Packet, error) {
	cc.cond.L.Lock()
	defer cc.cond.L.Unlock()
	for len(cc.queue) == 0 {
		if cc.isClosed() {
			return EmptyApplicationMessage, ErrClosed
		}
		log.Print(cc.local, " Before Wait() from ", cc.remote)
		cc.cond.Wait()
		log.Print(cc.local, " After Wait() from ", cc.remote)
	}
	nm := cc.queue[0]
	cc.queue = cc.queue[1:]
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
	localConnStore.Del(cc)
	cc.closing()
	// signal so Receive() is waken up
	cc.cond.Signal()
	c, ok := localConnStore.Get(cc.remote)
	if !ok {
		return nil
	}
	return c.Close()
}

func (cc *LocalConn) Rx() uint64 {
	return 0
}

func (cc *LocalConn) Tx() uint64 {
	return 0
}

func (cc *LocalConn) Type() ConnType {
	return Chan
}

type LocalListener struct {
	// addr is the addr we're listening to + mut
	addr string
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
	localConnStore.Listening(addr, fn)
	ll.Unlock()
	<-ll.quit
	return nil
}

func (ll *LocalListener) Stop() error {
	ll.quit <- true
	return nil
}

func (ll *LocalListener) IncomingType() ConnType {
	return Chan
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
	return NewLocalConn(lh.id.Address.NetworkAddress(), sid.Address.NetworkAddress())
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
