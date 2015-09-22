package coconet

// TCPHost is a simple implementation of Host that does not specify the
import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// Ensure that TCPHost satisfies the Host interface.
var _ Host = &TCPHost{}

type TCPHost struct {
	name     string
	listener net.Listener

	views *Views

	PeerLock sync.RWMutex
	peers    map[string]Conn
	Ready    map[string]bool

	// Peers asking to join overall tree structure of nodes
	// via connection to current host node
	PendingPeers map[string]bool

	pkLock sync.RWMutex
	Pubkey abstract.Point // own public key

	pool  *sync.Pool
	suite abstract.Suite

	// channels to send on Get() and update
	msgchan chan NetworkMessg

	// 1 if closed, 0 if not closed
	closed int64
}

// NewTCPHost creates a new TCPHost with a given hostname.
func NewTCPHost(hostname string) *TCPHost {
	h := &TCPHost{name: hostname,
		views:        NewViews(),
		msgchan:      make(chan NetworkMessg, 1),
		PendingPeers: make(map[string]bool)}
	h.peers = make(map[string]Conn)
	h.Ready = make(map[string]bool)
	return h
}

func (h *TCPHost) Views() *Views {
	return h.views
}

// SetSuite sets the suite of the TCPHost to use.
func (h *TCPHost) SetSuite(s abstract.Suite) {
	h.suite = s
}

// PubKey returns the public key of the host.
func (h *TCPHost) PubKey() abstract.Point {
	h.pkLock.RLock()
	pk := h.Pubkey
	h.pkLock.RUnlock()
	return pk
}

// SetPubKey sets the public key of the host.
func (h *TCPHost) SetPubKey(pk abstract.Point) {
	h.pkLock.Lock()
	h.Pubkey = pk
	h.pkLock.Unlock()
}

// StringMarshaler is a wrapper type to allow strings to be marshalled and unmarshalled.
type StringMarshaler string

// MarshalBinary implements the BinaryMarshaler interface for the StringMarshaler.
func (s *StringMarshaler) MarshalBinary() ([]byte, error) {
	return []byte(*s), nil
}

// UnmarshalBinary implements the BinaryUnmarshaler interface for the StringMarshaler.
func (s *StringMarshaler) UnmarshalBinary(b []byte) error {
	*s = StringMarshaler(b)
	return nil
}

// Listen listens for incoming TCP connections.
// It is a non-blocking call that runs in the background.
// It accepts incoming connections and establishes Peers.
// When a peer attempts to connect it must send over its name (as a StringMarshaler),
// as well as its public key.
// Only after that point can be communicated with.
func (h *TCPHost) Listen() error {
	var err error
	dbg.Lvl3("Starting to listen on", h.name)
	ln, err := net.Listen("tcp4", h.name)
	if err != nil {
		log.Println("failed to listen:", err)
		return err
	}
	h.listener = ln
	go func() {
		for {
			var err error
			dbg.Lvl3(h.Name(), "Accepting incoming")
			conn, err := ln.Accept()
			dbg.Lvl3(h.Name(), "Connection request - handling")
			if err != nil {
				log.Errorln("failed to accept connection: ", err)
				// if the host has been closed then stop listening
				if atomic.LoadInt64(&h.closed) == 1 {
					return
				}
				continue
			}

			// Read in name of client
			tp := NewTCPConnFromNet(conn)
			var mname StringMarshaler
			err = tp.Get(&mname)
			if err != nil {
				log.Errorln("failed to establish connection: getting name: ", err)
				tp.Close()
				continue
			}
			name := string(mname)

			// create connection
			tp.SetName(name)

			// get and set public key
			suite := h.suite
			pubkey := suite.Point()
			err = tp.Get(pubkey)
			if err != nil {
				log.Errorln("failed to establish connection: getting pubkey:", err)
				tp.Close()
				continue
			}
			tp.SetPubKey(pubkey)

			// give child the public key
			err = tp.Put(h.Pubkey)
			if err != nil {
				log.Errorln("failed to send public key:", err)
				continue
			}

			// the connection is now Ready to use
			h.PeerLock.Lock()
			h.Ready[name] = true
			h.peers[name] = tp
			dbg.Lvl3("Connected to child:", tp.Name())
			h.PeerLock.Unlock()

			go func() {
				for {
					data := h.pool.Get().(BinaryUnmarshaler)
					err := tp.Get(data)

					h.msgchan <- NetworkMessg{Data: data, From: tp.Name(), Err: err}
				}
			}()
		}
	}()
	return nil
}

func (h *TCPHost) ConnectTo(parent string) error {
	// If we have alReady set up this connection don't do anything
	h.PeerLock.Lock()
	if h.Ready[parent] {
		log.Println("ConnectTo: node already ready")
		h.PeerLock.RUnlock()
		return nil
	}
	h.PeerLock.Unlock()

	// connect to the parent
	conn, err := net.Dial("tcp4", parent)
	if err != nil {
		dbg.Lvl3("tcphost:", h.Name(), "failed to connect to parent:", err)
		return err
	}
	tp := NewTCPConnFromNet(conn)

	mname := StringMarshaler(h.Name())
	err = tp.Put(&mname)
	if err != nil {
		log.Errorln(err)
		return err
	}
	tp.SetName(parent)

	// give parent the public key
	err = tp.Put(h.Pubkey)
	if err != nil {
		log.Errorln("failed to send public key")
		return err
	}

	// get and set the parents public key
	suite := h.suite
	pubkey := suite.Point()
	err = tp.Get(pubkey)
	if err != nil {
		log.Errorln("failed to establish connection: getting pubkey:", err)
		tp.Close()
		return err
	}
	tp.SetPubKey(pubkey)

	h.PeerLock.Lock()
	h.Ready[tp.Name()] = true
	h.peers[parent] = tp
	// h.PendingPeers[parent] = true
	h.PeerLock.Unlock()
	dbg.Lvl4("CONNECTED TO PARENT:", parent)

	go func() {
		for {
			data := h.pool.Get().(BinaryUnmarshaler)
			err := tp.Get(data)

			h.msgchan <- NetworkMessg{Data: data, From: tp.Name(), Err: err}
		}
	}()

	return nil
}

func (h *TCPHost) Pending() map[string]bool {
	return h.PendingPeers
}

func (h *TCPHost) AddPeerToPending(p string) {
	h.PeerLock.Lock()
	h.PendingPeers[p] = true
	h.PeerLock.Unlock()
	log.Println("added peer to pending:", p)
}

// Connect connects to the parent in the given view.
// It connects to the parent by establishing a TCPConn.
// It then sends its name and public key to initialize the connection.
func (h *TCPHost) Connect(view int) error {
	// Get the parent of the given view.
	v := h.views.Views[view]
	parent := v.Parent
	if parent == "" {
		return nil
	}
	h.PeerLock.Lock()
	delete(h.PendingPeers, parent)
	h.PeerLock.Unlock()
	return h.ConnectTo(parent)
}

// NewView creates a new view with the given view number, parent and children.
func (h *TCPHost) NewView(view int, parent string, children []string, hostlist []string) {
	h.views.NewView(view, parent, children, hostlist)
}

func (h *TCPHost) NewViewFromPrev(view int, parent string) {
	h.views.NewViewFromPrev(view, parent)
}

// Close closes all the connections currently open.
func (h *TCPHost) Close() {
	log.Println("tcphost: closing")
	// stop accepting new connections
	atomic.StoreInt64(&h.closed, 1)
	h.listener.Close()

	// close peer connections
	h.PeerLock.Lock()
	for _, p := range h.peers {
		if p != nil {
			p.Close()
		}
	}
	h.PeerLock.Unlock()

}

func (h *TCPHost) Closed() bool {
	return atomic.LoadInt64(&h.closed) == 1
}

// AddParent adds a parent node to the TCPHost, for the given view.
func (h *TCPHost) AddParent(view int, c string) {
	h.PeerLock.Lock()
	if _, ok := h.peers[c]; !ok {
		h.peers[c] = NewTCPConn(c)
	}
	// remove from pending peers list
	delete(h.PendingPeers, c)
	h.PeerLock.Unlock()
	dbg.Lvl4("Adding parent to views on", h.Name(), "for", c)
	h.views.AddParent(view, c)
}

// AddChildren adds children to the specified view.
func (h *TCPHost) AddChildren(view int, cs ...string) {
	for _, c := range cs {
		// if the peer doesn't exist add it to Peers
		h.PeerLock.Lock()
		if _, ok := h.peers[c]; !ok {
			h.peers[c] = NewTCPConn(c)
		}
		delete(h.PendingPeers, c)
		h.PeerLock.Unlock()

		h.views.AddChildren(view, c)
	}
}

func (h *TCPHost) AddPeerToHostlist(view int, name string) {
	h.views.AddPeerToHostlist(view, name)
}

func (h *TCPHost) RemovePeerFromHostlist(view int, name string) {
	h.views.RemovePeerFromHostlist(view, name)
}

func (h *TCPHost) AddPendingPeer(view int, name string) error {
	h.PeerLock.Lock()
	if _, ok := h.PendingPeers[name]; !ok {
		h.PeerLock.Unlock()
		return errors.New("error adding pending peer: not in pending peers")
	}
	delete(h.PendingPeers, name)

	h.PeerLock.Unlock()

	// we have already connected to the name
	// h.ConnectTo(name)

	h.AddChildren(view, name)
	return nil
}

func (h *TCPHost) RemovePendingPeer(peer string) {
	h.PeerLock.Lock()
	delete(h.PendingPeers, peer)
	h.PeerLock.Unlock()
}

func (h *TCPHost) RemovePeer(view int, name string) bool {
	return h.views.RemovePeer(view, name)
}

// NChildren returns the number of children for the specified view.
func (h *TCPHost) NChildren(view int) int {
	return h.views.NChildren(view)
}

func (h *TCPHost) HostListOn(view int) []string {
	return h.views.HostList(view)
}

func (h *TCPHost) SetHostList(view int, hostlist []string) {
	h.views.SetHostList(view, hostlist)
}

// Name returns the hostname of the TCPHost.
func (h *TCPHost) Name() string {
	return h.name
}

// IsRoot returns true if the TCPHost is the root of it's tree for the given view..
func (h *TCPHost) IsRoot(view int) bool {
	return h.views.Parent(view) == ""
}

// IsParent returns true if the given peer is the parent for the specified view.
func (h *TCPHost) IsParent(view int, peer string) bool {
	return h.views.Parent(view) == peer
}

func (h *TCPHost) Parent(view int) string {
	return h.views.Parent(view)
}

// IsChild returns true f the given peer is the child for the specified view.
func (h *TCPHost) IsChild(view int, peer string) bool {
	h.PeerLock.Lock()
	_, ok := h.peers[peer]
	h.PeerLock.Unlock()
	return h.views.Parent(view) != peer && ok
}

// Peers returns the list of Peers as a mapping from hostname to Conn.
func (h *TCPHost) Peers() map[string]Conn {
	return h.peers
}

// Children returns a map of childname to Conn for the given view.
func (h *TCPHost) Children(view int) map[string]Conn {
	h.PeerLock.Lock()

	childrenMap := make(map[string]Conn, 0)
	children := h.views.Children(view)
	for _, c := range children {
		if !h.Ready[c] {
			continue
		}
		childrenMap[c] = h.peers[c]
	}

	h.PeerLock.Unlock()

	return childrenMap
}

// AddPeers adds the list of Peers.
func (h *TCPHost) AddPeers(cs ...string) {
	// XXX does it make sense to add Peers that are not children or parents
	h.PeerLock.Lock()
	for _, c := range cs {
		h.peers[c] = NewTCPConn(c)
	}
	h.PeerLock.Unlock()
}

// ErrClosed indicates that the connection has been closed.
var ErrClosed = errors.New("connection closed")

func (h *TCPHost) PutTo(ctx context.Context, host string, data BinaryMarshaler) error {
	pname := host
	done := make(chan error)
	canceled := int64(0)
	go func() {
		// try until this is canceled, closed, or successful
		for {
			if atomic.LoadInt64(&canceled) == 1 {
				return
			}

			h.PeerLock.Lock()
			isReady, ok := h.Ready[pname]
			parent, ok := h.peers[pname]
			h.PeerLock.Unlock()
			if !ok {
				done <- errors.New("not connected to peer")
				return
			}
			if !isReady {
				time.Sleep(250 * time.Millisecond)
				continue
			}

			if parent.Closed() {
				done <- ErrClosed
				return
			}
			// if the connection has been closed put will fail
			done <- parent.Put(data)
			return
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		atomic.StoreInt64(&canceled, 1)
		return ctx.Err()
	}

}

// PutUp sends a message to the parent in the specified view.
func (h *TCPHost) PutUp(ctx context.Context, view int, data BinaryMarshaler) error {
	pname := h.views.Parent(view)
	done := make(chan error)
	canceled := int64(0)
	go func() {
		// try until this is canceled, closed, or successful
		for {
			if atomic.LoadInt64(&canceled) == 1 {
				return
			}

			h.PeerLock.Lock()
			isReady := h.Ready[pname]
			parent := h.peers[pname]
			h.PeerLock.Unlock()
			if !isReady {
				time.Sleep(250 * time.Millisecond)
				continue
			}

			if parent.Closed() {
				done <- ErrClosed
				return
			}
			// if the connection has been closed put will fail
			done <- parent.Put(data)
			return
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		atomic.StoreInt64(&canceled, 1)
		return ctx.Err()
	}
}

// PutDown sends a message (an interface{} value) up to all children through
// whatever 'network' interface each child Peer implements.
func (h *TCPHost) PutDown(ctx context.Context, view int, data []BinaryMarshaler) error {
	// Try to send the message to all children
	// If at least one of the attempts fails, return a non-nil error
	var err error
	var errLock sync.Mutex
	children := h.views.Children(view)
	if len(data) != len(children) {
		panic("number of messages passed down != number of children")
	}
	var canceled int64
	var wg sync.WaitGroup
	dbg.LLvl4(h.Name(), "sending to", len(children), "children")
	for i, c := range children {
		dbg.LLvl4("Sending to child", c)
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			// try until it is canceled, successful, or timed-out
			for {
				// check to see if it has been canceled
				if atomic.LoadInt64(&canceled) == 1 {
					return
				}

				// if it is not Ready try again later
				h.PeerLock.Lock()
				Ready := h.Ready[c]
				conn := h.peers[c]
				h.PeerLock.Unlock()
				if Ready {
					if e := conn.Put(data[i]); e != nil {
						errLock.Lock()
						err = e
						errLock.Unlock()
					}
					dbg.LLvl4("Informed child", c)
					return
				}
				dbg.LLvl4("Re-trying, waiting to put down msg from", h.Name(), "to", c)
				time.Sleep(250 * time.Millisecond)
			}

		}(i, c)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
		atomic.StoreInt64(&canceled, 1)
	}

	return err
}

// Get gets from all of the Peers and sends the responses to a channel of
// NetworkMessg and errors that it returns.
//
// TODO: each of these goroutines could be spawned when we initally connect to
// them instead.
func (h *TCPHost) Get() chan NetworkMessg {
	return h.msgchan
}

// Pool is the underlying pool of BinaryUnmarshallers to use when getting.
func (h *TCPHost) Pool() *sync.Pool {
	return h.pool
}

// SetPool sets the pool of BinaryUnmarshallers when getting from channels
func (h *TCPHost) SetPool(p *sync.Pool) {
	h.pool = p
}
