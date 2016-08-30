package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

func NewLocalRouter(sid *ServerIdentity) (*Router, error) {
	h, err := NewLocalHost(sid.Address)
	if err != nil {
		return nil, err
	}
	return NewRouter(sid, h), nil
}

// localConnStore_ keeps reference to all opened local connections
// It also keeps tracks of who is "listening", so it's possible to mimics
// Conn & Listener.
type localConnStore_ struct {
	queues map[endpoints]*connQueue
	sync.Mutex
	listening map[Address]func(Conn)
}

// endpoints represents the two end points of a connection
type endpoints struct {
	local  Address
	remote Address
}

// reverse simply reverse the endpoints from local <-> remote
func (e *endpoints) reverse() endpoints {
	return endpoints{
		local:  e.remote,
		remote: e.local,
	}
}

var localConnStore = &localConnStore_{
	queues:    make(map[endpoints]*connQueue),
	listening: make(map[Address]func(Conn)),
}

// String returns the string representation of this local store.
func (ccc *localConnStore_) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "(LocalConnStore) Listening: ")
	for a := range ccc.listening {
		fmt.Fprintf(&b, "%s ", a)
	}
	fmt.Fprintf(&b, "\n(localConnStore) Connections: ")
	for a := range ccc.queues {
		fmt.Fprintf(&b, "[ %s -> %s ] ", a.local, a.remote)
	}
	return b.String()
}

// LocalDump returns a view of the local connections + listener present at
// this time
func LocalDump() string {
	return localConnStore.String()
}

// LocalReset reset the whole map of connections + listener so it is like
// a fresh localConnStore.
func LocalReset() {
	localConnStore = &localConnStore_{
		queues:    make(map[endpoints]*connQueue),
		listening: make(map[Address]func(Conn)),
	}

}

// Put takes a new local connection object and stores it
func (ccc *localConnStore_) Put(e endpoints, c *connQueue) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.queues[e] = c
}

// Del removes the local connection object
func (ccc *localConnStore_) Del(e endpoints, c *connQueue) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.queues, e)
}

// Islistening returns true if the remote address is listening "virtually"
func (ccc *localConnStore_) IsListening(remote Address) bool {
	ccc.Lock()
	defer ccc.Unlock()
	_, ok := ccc.listening[remote]
	return ok
}

// Listening put the address as "listening" mode. If a user connects to this
// addr, this function will be called.
func (ccc *localConnStore_) Listening(addr Address, fn func(Conn)) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.listening[addr] = fn
}

// StopListening remove the address from the "listening" mode
func (ccc *localConnStore_) StopListening(addr Address) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.listening, addr)
}

// Connect will check if the remote address is listening, if yes it creates
// the two connections, and launch the listening function in a go routine.
// It returns the outgoing connection with any error.
func (ccc *localConnStore_) Connect(local, remote Address) (*LocalConn, error) {
	ccc.Lock()
	defer ccc.Unlock()
	if local == remote {
		return nil, errors.New("Don't connect to yourself using local conn")
	}

	fn, ok := ccc.listening[remote]
	if !ok {
		return nil, fmt.Errorf("%s can't connect to %s: it's not listening", local, remote)
	}

	outgoing := newLocalConn(local, remote)
	incoming := newLocalConn(remote, local)

	ccc.queues[outgoing.endpoints] = outgoing.connQueue
	ccc.queues[incoming.endpoints] = incoming.connQueue

	go fn(incoming)
	return outgoing, nil
}

// Send will get the connection denoted by this endpoint and will call queueMsg
// with the packet as argument on it. It returns ErrClosed if it does not find
// the connection.
func (ccc *localConnStore_) Send(e endpoints, nm Packet) error {
	ccc.Lock()
	defer ccc.Unlock()
	q, ok := ccc.queues[e]
	if !ok {
		return ErrClosed
	}

	q.Push(nm)
	return nil
}

// Close will get the connection denoted by this endpoint and will Close it if
// present.
func (ccc *localConnStore_) Close(local *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	// delete this conn
	delete(ccc.queues, local.endpoints)
	// and delete the remote one + close it
	remote, ok := ccc.queues[local.reverse()]
	if !ok {
		return
	}
	delete(ccc.queues, local.reverse())
	remote.Close()
}

// Len returns how many local connections is there
func (ccc *localConnStore_) Len() int {
	ccc.Lock()
	defer ccc.Unlock()
	return len(ccc.queues)
}

// ChannConn is a connection that send and receive messages through channels
type LocalConn struct {
	endpoints

	*connQueue

	mut sync.Mutex
}

// newLocalConn simply init the fields of a LocalConn but do not try to
// connect. It should not be used as-is, most user wants to call NewLocalConn.
func newLocalConn(local, remote Address) *LocalConn {
	return &LocalConn{
		endpoints: endpoints{
			remote: remote,
			local:  local,
		},
		connQueue: newConnQueue(),
	}
}

// Returns a new channel connection from local to remote
// Mimics the behavior of NewTCPConn => tries connecting right away.
func NewLocalConn(local, remote Address) (*LocalConn, error) {
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
	return localConnStore.Send(cc.reverse(), nm)
}

func (cc *LocalConn) Receive(ctx context.Context) (Packet, error) {
	return cc.Pop()
}

func (cc *LocalConn) Local() Address {
	return cc.local
}

func (cc *LocalConn) Remote() Address {
	return cc.remote
}

func (cc *LocalConn) Close() error {
	cc.connQueue.Close()
	// close the remote conn also
	localConnStore.Close(cc)
	return nil
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

type connQueue struct {
	*sync.Cond
	queue    []Packet
	isClosed bool
}

func newConnQueue() *connQueue {
	return &connQueue{
		Cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (c *connQueue) Push(p Packet) {
	c.L.Lock()
	defer c.L.Unlock()
	if c.isClosed {
		return
	}
	c.queue = append(c.queue, p)
	c.Signal()
}

func (c *connQueue) Pop() (Packet, error) {
	c.L.Lock()
	defer c.L.Unlock()
	for len(c.queue) == 0 {
		if c.isClosed {
			return EmptyApplicationMessage, ErrClosed
		}
		c.Wait()
	}
	if c.isClosed {
		return EmptyApplicationMessage, ErrClosed
	}
	nm := c.queue[0]
	c.queue = c.queue[1:]
	return nm, nil
}

func (c *connQueue) Close() {
	c.L.Lock()
	defer c.L.Unlock()
	c.isClosed = true
	c.Signal()
}

type LocalListener struct {
	// addr is the addr we're listening to + mut
	addr Address
	// are we listening or not
	listening bool

	sync.Mutex

	// quit is used to stop the listening routine
	quit chan bool
}

func NewLocalListener(addr Address) (*LocalListener, error) {
	l := &LocalListener{
		quit: make(chan bool),
	}
	return l, l.bind(addr)
}

func (ll *LocalListener) bind(addr Address) error {
	ll.Lock()
	defer ll.Unlock()
	if addr.ConnType() != Local {
		return errors.New("Wrong address type for local listener")
	}
	ll.addr = addr
	return nil
}

func (ll *LocalListener) Listen(fn func(Conn)) error {
	ll.Lock()
	ll.quit = make(chan bool)
	ll.listening = true
	localConnStore.Listening(ll.addr, fn)
	ll.Unlock()

	<-ll.quit
	return nil
}

func (ll *LocalListener) Stop() error {
	ll.Lock()
	defer ll.Unlock()
	localConnStore.StopListening(ll.addr)
	close(ll.quit)
	return nil
}

func (ll *LocalListener) Address() Address {
	ll.Lock()
	defer ll.Unlock()
	return ll.addr
}

func (ll *LocalListener) Listening() bool {
	ll.Lock()
	defer ll.Unlock()
	return ll.listening
}

type LocalHost struct {
	addr Address
	*LocalListener
}

func NewLocalHost(addr Address) (*LocalHost, error) {
	lh := &LocalHost{
		addr: addr,
	}
	var err error
	lh.LocalListener, err = NewLocalListener(addr)
	return lh, err
}

func (lh *LocalHost) Connect(addr Address) (Conn, error) {
	if addr.ConnType() != Local {
		return nil, errors.New("Can't connect to non-Local address")
	}
	var finalErr error
	for i := 0; i < MaxRetry; i++ {
		c, err := NewLocalConn(lh.addr, addr)
		if err == nil {
			return c, nil
		}
		finalErr = err
		time.Sleep(WaitRetry)
	}
	return nil, finalErr

}

func NewLocalClient() *Client {
	fn := func(own, remote *ServerIdentity) (Conn, error) {
		return NewLocalConn(own.Address, remote.Address)
	}
	return newClient(fn)
}

func NewLocalAddress(addr string) Address {
	return NewAddress(Local, addr)
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
