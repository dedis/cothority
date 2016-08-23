package network

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// localConnStore_ keeps reference to all opened local connections
// It also keeps tracks of who is "listening", so it's possible to mimics
// Conn & Listener.
type localConnStore_ struct {
	conns map[string]*LocalConn
	sync.Mutex
	listening map[string]bool
	listMut   sync.Mutex
}

var localConnStore = &localConnStore_{
	conns: make(map[string]*LocalConn),
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

// Len returns how many global channel connections is there
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
}

// Returns a new channel connection from local to remote
func NewLocalConn(local, remote string) Conn {
	c := &LocalConn{
		remote: remote,
		local:  local,
		cond:   sync.NewCond(&sync.Mutex{}),
	}
	localConnStore.Put(c)
	return c
}

func (cc *LocalConn) Send(ctx context.Context, msg Body) error {
	c, ok := localConnStore.Get(cc.remote)
	if !ok {
		return fmt.Errorf("No connection opened at this address", cc.remote)
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

	c.queueMsg(nm)
	return nil
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
		cc.cond.Wait()
	}
	nm := cc.queue[0]
	cc.queue = cc.queue[1:]
	cc.cond.L.Unlock()
	return nm, nil
}

func (cc *LocalConn) Local() string {
	return cc.local
}

func (cc *LocalConn) Remote() string {
	return cc.remote
}

func (cc *LocalConn) Close() error {
	localConnStore.Del(cc)
	return nil
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
