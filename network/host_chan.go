// host_chan includes all the functionalities to emulate a network using only
// channels and go routines. The main purpose here is to be able to emulate
// connections for all other tests than those related to network (sda/,
// protocols/ etc).
package network

import (
	"errors"
	"reflect"
	"sync"

	"github.com/dedis/cothority/log"
)

// chanHost is a struct that implements the Host interface locally using
// channels and go routines.
type chanHost struct {
	Dispatcher
	identity *ServerIdentity
	// msgQueue is the channel where other chanHost communicate messages to
	// this chanHost.
	msgChan chan *Packet
	conns   *connsStore
}

// NewLocalHost will return a fresh router using native go channels to communicate
// to others chanHost. Its purpose is mainly for easy testing without any
// trouble of opening / closing / waiting for the network socket ...
func NewLocalHost(identity *ServerIdentity) *chanHost {
	r := &chanHost{
		Dispatcher: NewBlockingDispatcher(),
		identity:   identity,
		msgChan:    make(chan *Packet, 100),
		conns:      newConnsStore(),
	}
	chanHosts.Put(r)
	return r
}

func (l *chanHost) serverIdentity() *ServerIdentity {
	return l.identity
}

// Send implements the Host interface
func (l *chanHost) Send(e *ServerIdentity, msg Body) error {
	r := chanHosts.Get(e)
	if r == nil {
		return errors.New("No mock routers at this entity")
	}

	l.conns.Put(e.String())

	var body Body
	var val = reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	body = val.Interface()

	var typ = TypeFromData(body)
	nm := Packet{
		MsgType:        typ,
		Msg:            body,
		ServerIdentity: l.identity,
	}
	r.receive(&nm)
	return nil
}

func (l *chanHost) receive(msg *Packet) {
	l.msgChan <- msg
}

// Run will make the chanHost start listening on its incoming channel. It's a
// blocking call.
func (l *chanHost) Start() {
	for msg := range l.msgChan {
		l.conns.Put(msg.ServerIdentity.String())
		log.Lvl5(l.Address, "Received message", msg.MsgType, "from", msg.ServerIdentity.Address)
		if err := l.Dispatch(msg); err != nil {
			log.Lvl4(l.Address(), "Error dispatching:", err)
		}
	}
}

// Close implements the Host interface. It will stop the dispatching of
// incoming messages.
func (l *chanHost) Stop() error {
	close(l.msgChan)
	return nil
}

// Tx implements the Host interface (mainly for compatibility reason with
// monitor.CounterIO which is needed for TcpHost simulations)
func (l *chanHost) Tx() uint64 {
	return 0
}

// Rx implements the Host interface (mainly for compatibility reason with
// monitor.CounterIO which is needed for TcpHost simulations)
func (l *chanHost) Rx() uint64 {
	return 0
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

// Address implements the Host interface
func (l *chanHost) Address() string {
	return string(l.identity.Address)
}

// chanHostStore keeps tracks of all the mock routers
type chanHostStore struct {
	chanHosts map[ServerIdentityID]*chanHost
	mut       sync.Mutex
}

// chanHosts is the store that keeps tracks of all opened local routers in a
// thread safe manner
var chanHosts = chanHostStore{
	chanHosts: make(map[ServerIdentityID]*chanHost),
}

func (lrs *chanHostStore) Put(r *chanHost) {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	lrs.chanHosts[r.serverIdentity().ID] = r
}

// Get returns the router associated with this ServerIdentity. It returns nil if
// there is no chanHost associated with this ServerIdentity
func (lrs *chanHostStore) Get(id *ServerIdentity) *chanHost {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	r, ok := lrs.chanHosts[id.ID]
	if !ok {
		return nil
	}
	return r
}

func (lrs *chanHostStore) Len() int {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	return len(lrs.chanHosts)
}

type connsStore struct {
	// conns keep tracks of to whom this local router sent something so it can
	// have a reasonable loooking report status in GetStatus
	conns map[string]bool
	sync.Mutex
}

func (cs *connsStore) Put(name string) {
	cs.Lock()
	defer cs.Unlock()
	cs.conns[name] = true
}
func (cs *connsStore) Get() []string {
	cs.Lock()
	defer cs.Unlock()
	var names []string
	for k := range cs.conns {
		names = append(names, k)
	}
	return names
}
func (cs *connsStore) Len() int {
	cs.Lock()
	defer cs.Unlock()
	return len(cs.conns)
}

func newConnsStore() *connsStore {
	return &connsStore{
		conns: make(map[string]bool),
	}
}
