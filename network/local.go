package network

import (
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"reflect"
	"sync"
	"time"
)

// NewLocalRouter returns a fresh router which uses local connections. It takes
// the default context.
func NewLocalRouter(sid *ServerIdentity) (*Router, error) {
	return NewLocalRouterWithContext(defaultLocalContext, sid)
}

// NewLocalRouterWithContext is the same as NewLocalRouter but takes a specific
// LocalContext. This is useful to run parallel different local overlays.
func NewLocalRouterWithContext(ctx *LocalContext, sid *ServerIdentity) (*Router, error) {
	h, err := NewLocalHostWithContext(ctx, sid.Address)
	if err != nil {
		return nil, err
	}
	return NewRouter(sid, h), nil
}

// LocalContext keeps reference to all opened local connections
// It also keeps tracks of who is "listening", so it's possible to mimics
// Conn & Listener.
type LocalContext struct {
	// queues maps a remote endpoint to its packet queue. It's the main
	//stucture used to communicate.
	queues map[endpoint]*connQueue
	sync.Mutex
	listening map[Address]func(Conn)

	baseUID uint64
}

// NewLocalContext returns a fresh new context that can be used by LocalConn,
// LocalListener & LocalHost.
func NewLocalContext() *LocalContext {
	return &LocalContext{
		queues:    make(map[endpoint]*connQueue),
		listening: make(map[Address]func(Conn)),
	}
}

var defaultLocalContext = NewLocalContext()

// endpoint represents one endpoint of a connection.
type endpoint struct {
	addr Address
	// uid is a unique identifier of the remote endpoint
	// it's unique  for each direction:
	// 127.0.0.1:2000 -> 127.0.0.1:2000 => 14
	// 127.0.0.1:2000 <- 127.0.0.1:2000 => 15
	uid uint64
}

// LocalReset reset the whole map of connections + listener so it is like
// a fresh defaultLocalContext.
func LocalReset() {
	defaultLocalContext = NewLocalContext()

}

// IsListening returns true if the remote address is listening "virtually"
func (ccc *LocalContext) IsListening(remote Address) bool {
	ccc.Lock()
	defer ccc.Unlock()
	_, ok := ccc.listening[remote]
	return ok
}

// Listening put the address as "listening" mode. If a user connects to this
// addr, this function will be called.
func (ccc *LocalContext) Listening(addr Address, fn func(Conn)) {
	ccc.Lock()
	defer ccc.Unlock()
	ccc.listening[addr] = fn
}

// StopListening remove the address from the "listening" mode
func (ccc *LocalContext) StopListening(addr Address) {
	ccc.Lock()
	defer ccc.Unlock()
	delete(ccc.listening, addr)
}

// Connect will check if the remote address is listening, if yes it creates
// the two connections, and launch the listening function in a go routine.
// It returns the outgoing connection with any error.
func (ccc *LocalContext) Connect(local, remote Address) (*LocalConn, error) {
	ccc.Lock()
	defer ccc.Unlock()

	fn, ok := ccc.listening[remote]
	if !ok {
		return nil, fmt.Errorf("%s can't connect to %s: it's not listening", local, remote)
	}

	outEndpoint := endpoint{local, ccc.baseUID}
	ccc.baseUID++

	incEndpoint := endpoint{remote, ccc.baseUID}
	ccc.baseUID++

	outgoing := newLocalConn(ccc, outEndpoint, incEndpoint)
	incoming := newLocalConn(ccc, incEndpoint, outEndpoint)

	// outgoing knows how to store packet into the incoming's queue
	ccc.queues[outEndpoint] = outgoing.connQueue
	// incoming knows how to store packet into the outgoing's queue
	ccc.queues[incEndpoint] = incoming.connQueue

	go fn(incoming)
	return outgoing, nil
}

// Send will get the connection denoted by this endpoint and will call queueMsg
// with the packet as argument on it. It returns ErrClosed if it does not find
// the connection.
func (ccc *LocalContext) Send(e endpoint, nm Packet) error {
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
func (ccc *LocalContext) Close(conn *LocalConn) {
	ccc.Lock()
	defer ccc.Unlock()
	// delete this conn
	delete(ccc.queues, conn.local)
	// and delete the remote one + close it
	remote, ok := ccc.queues[conn.remote]
	if !ok {
		return
	}
	delete(ccc.queues, conn.remote)
	remote.Close()
}

// Len returns how many local connections is there
func (ccc *LocalContext) Len() int {
	ccc.Lock()
	defer ccc.Unlock()
	return len(ccc.queues)
}

// LocalConn is a connection that send and receive messages through channels
type LocalConn struct {
	local  endpoint
	remote endpoint

	// connQueue is the part that is accesible from the LocalContext (i.e. is
	// shared). Reason why we can't directly share LocalConn is because go test
	// -race detects as data race (while it's *protected*)
	*connQueue

	ctx *LocalContext
}

// newLocalConn simply init the fields of a LocalConn but do not try to
// connect. It should not be used as-is, most user wants to call NewLocalConn.
func newLocalConn(ctx *LocalContext, local, remote endpoint) *LocalConn {
	return &LocalConn{
		remote:    remote,
		local:     local,
		connQueue: newConnQueue(),
		ctx:       ctx,
	}
}

// NewLocalConn returns a new channel connection from local to remote
// Mimics the behavior of NewTCPConn => tries connecting right away.
// It uses the default local context.
func NewLocalConn(local, remote Address) (*LocalConn, error) {
	return NewLocalConnWithContext(defaultLocalContext, local, remote)
}

// NewLocalConnWithContext is similar to NewLocalConn but takes a specific
// LocalContext.
func NewLocalConnWithContext(ctx *LocalContext, local, remote Address) (*LocalConn, error) {
	return ctx.Connect(local, remote)
}

// Send implements the Conn interface.
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
	return cc.ctx.Send(cc.remote, nm)
}

// Receive implements the Conn interface.
func (cc *LocalConn) Receive(ctx context.Context) (Packet, error) {
	return cc.Pop()
}

// Local implements the Conn interface
func (cc *LocalConn) Local() Address {
	return cc.local.addr
}

// Remote implements the Conn interface
func (cc *LocalConn) Remote() Address {
	return cc.remote.addr
}

// Close implements the Conn interface
func (cc *LocalConn) Close() error {
	cc.connQueue.Close()
	// close the remote conn also
	cc.ctx.Close(cc)
	return nil
}

// Rx implements the Conn interface
func (cc *LocalConn) Rx() uint64 {
	return 0
}

// Tx implements the Conn interface
func (cc *LocalConn) Tx() uint64 {
	return 0
}

// Type implements the Conn interface
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
			return EmptyApplicationPacket, ErrClosed
		}
		c.Wait()
	}
	if c.isClosed {
		return EmptyApplicationPacket, ErrClosed
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

// LocalListener is a Listener that uses LocalConn to communicate. It tries to
// behave as much as possible as a real golang net.Listener but using LocalConn
// as the underlying communication layer.
type LocalListener struct {
	// addr is the addr we're listening to + mut
	addr Address
	// are we listening or not
	listening bool

	sync.Mutex

	// quit is used to stop the listening routine
	quit chan bool

	ctx *LocalContext
}

// NewLocalListener returns a fresh LocalListener which implements the Listener
// interface.
func NewLocalListener(addr Address) (*LocalListener, error) {
	return NewLocalListenerWithContext(defaultLocalContext, addr)
}

// NewLocalListenerWithContext is similar to NewLocalListener but taking a
// specific LocalContext to use to communicate.
func NewLocalListenerWithContext(ctx *LocalContext, addr Address) (*LocalListener, error) {
	l := &LocalListener{
		quit: make(chan bool),
		ctx:  ctx,
	}
	return l, l.bind(addr)
}

func (ll *LocalListener) bind(addr Address) error {
	ll.Lock()
	defer ll.Unlock()
	if addr.ConnType() != Local {
		return errors.New("Wrong address type for local listener")
	}
	if ll.ctx.IsListening(addr) {
		return fmt.Errorf("%s is already listening: can't listen again", addr)
	}
	ll.addr = addr
	return nil
}

// Listen implements the Listener interface
func (ll *LocalListener) Listen(fn func(Conn)) error {
	ll.Lock()
	ll.quit = make(chan bool)
	ll.ctx.Listening(ll.addr, fn)
	ll.listening = true
	ll.Unlock()

	<-ll.quit
	return nil
}

// Stop implements the Listener interface
func (ll *LocalListener) Stop() error {
	ll.Lock()
	defer ll.Unlock()
	ll.ctx.StopListening(ll.addr)
	if ll.listening {
		close(ll.quit)
	}
	ll.listening = false
	return nil
}

// Address returns the listening address used.
func (ll *LocalListener) Address() Address {
	ll.Lock()
	defer ll.Unlock()
	return ll.addr
}

// Listening returns true if this Listener is actually listening for any
// incoming connections.
func (ll *LocalListener) Listening() bool {
	ll.Lock()
	defer ll.Unlock()
	return ll.listening
}

// LocalHost is a Host implementation using LocalConn and LocalListener as
// the underlying means of communication. It is a implementation of the Host
// interface.
type LocalHost struct {
	addr Address
	*LocalListener
	ctx *LocalContext
}

// NewLocalHost returns a fresh Host using Local communication that will listen
// on the given addr.
func NewLocalHost(addr Address) (*LocalHost, error) {
	return NewLocalHostWithContext(defaultLocalContext, addr)
}

// NewLocalHostWithContext is similar to NewLocalHost but specify which
// LocalContext to use for communicating.
func NewLocalHostWithContext(ctx *LocalContext, addr Address) (*LocalHost, error) {
	lh := &LocalHost{
		addr: addr,
		ctx:  ctx,
	}
	var err error
	lh.LocalListener, err = NewLocalListenerWithContext(ctx, addr)
	return lh, err

}

// Connect implements the Host interface.
func (lh *LocalHost) Connect(addr Address) (Conn, error) {
	if addr.ConnType() != Local {
		return nil, errors.New("Can't connect to non-Local address")
	}
	var finalErr error
	for i := 0; i < MaxRetryConnect; i++ {
		c, err := NewLocalConnWithContext(lh.ctx, lh.addr, addr)
		if err == nil {
			return c, nil
		}
		finalErr = err
		time.Sleep(WaitRetry)
	}
	return nil, finalErr

}

// NewLocalClient returns Client that uses the Local communication layer.
func NewLocalClient() *Client {
	return NewLocalClientWithContext(defaultLocalContext)
}

// NewLocalClientWithContext is similar to NewLocalClient but takes a specific
// LocalContext to communicate.
func NewLocalClientWithContext(ctx *LocalContext) *Client {
	fn := func(own, remote *ServerIdentity) (Conn, error) {
		return NewLocalConnWithContext(ctx, own.Address, remote.Address)
	}
	return newClient(fn)

}

// NewLocalAddress returns an Address of type Local with the given raw addr.
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
