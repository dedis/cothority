package coconet

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

func init() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT)
	go func() {
		<-sigc
		panic("CTRL-C")
	}()
}

// a GoHost must satisfy the host interface
var _ Host = &GoHost{}

// GoHost is an implementation of the Host interface,
// that uses GoConns as its underlying connection type.
type GoHost struct {
	name string // the hostname

	views    *Views
	dir      *GoDirectory
	PeerLock sync.RWMutex
	peers    map[string]Conn
	Ready    map[string]bool
	// Peers asking to join overall tree structure of nodes
	// via connection to current host node
	PendingPeers map[string]bool

	suite abstract.Suite

	pkLock sync.RWMutex
	Pubkey abstract.Point // own public key

	pool *sync.Pool

	msgchan chan NetworkMessg
	closed  int64
}

// GetDirectory returns the underlying directory used for GoHosts.
func (h *GoHost) GetDirectory() *GoDirectory {
	return h.dir
}

// NewGoHost creates a new GoHost with the given hostname,
// and registers it in the given directory.
func NewGoHost(hostname string, dir *GoDirectory) *GoHost {
	h := &GoHost{name: hostname,
		views:   NewViews(),
		dir:     dir,
		msgchan: make(chan NetworkMessg, 0)}
	h.peers = make(map[string]Conn)
	h.PeerLock = sync.RWMutex{}
	h.Ready = make(map[string]bool)
	return h
}

func (h *GoHost) Views() *Views {
	return h.views
}

// SetSuite sets the crypto suite which this Host is using.
func (h *GoHost) SetSuite(s abstract.Suite) {
	h.suite = s
}

// PubKey returns the public key of the Host.
func (h *GoHost) PubKey() abstract.Point {
	h.pkLock.RLock()
	pk := h.Pubkey
	h.pkLock.RUnlock()
	return pk
}

// SetPubKey sets the publick key of the Host.
func (h *GoHost) SetPubKey(pk abstract.Point) {
	h.pkLock.Lock()
	h.Pubkey = pk
	h.pkLock.Unlock()
}

func (h *GoHost) ConnectTo(parent string) error {
	// if the connection has been established skip it
	h.PeerLock.Lock()
	if h.Ready[parent] {
		dbg.Warn("peer is alReady Ready")
		h.PeerLock.Unlock()
		return nil
	}
	h.PeerLock.Unlock()

	// get the connection to the parent
	conn := h.peers[parent]

	// send the hostname to the destination
	mname := StringMarshaler(h.Name())
	err := conn.PutData(&mname)
	if err != nil {
		dbg.Fatal("failed to connect: putting name:", err)
	}

	// give the parent the public key
	err = conn.PutData(h.Pubkey)
	if err != nil {
		dbg.Fatal("failed to send public key:", err)
	}

	// get the public key of the parent
	suite := h.suite
	pubkey := suite.Point()
	err = conn.GetData(pubkey)
	if err != nil {
		dbg.Fatal("failed to establish connection: getting pubkey:", err)
	}
	conn.SetPubKey(pubkey)

	h.PeerLock.Lock()
	h.Ready[conn.Name()] = true
	h.peers[parent] = conn
	h.PeerLock.Unlock()

	go func() {
		for {
			data := h.pool.Get().(BinaryUnmarshaler)
			err := conn.GetData(data)

			h.msgchan <- NetworkMessg{Data: data, From: conn.Name(), Err: err}
		}
	}()

	return nil
}

// Connect connects to the parent of the host.
// For GoHosts this is a noop.
func (h *GoHost) Connect(view int) error {
	parent := h.views.Parent(view)
	if parent == "" {
		return nil
	}
	return h.ConnectTo(parent)
}

// It shares the public keys and names of the hosts.
func (h *GoHost) Listen() error {
	children := h.views.Children(0)
	// listen for connection attempts from each of the children
	for _, c := range children {
		go func(c string) {

			if h.Ready[c] {
				dbg.Fatal("listening: connection alReady established")
			}

			h.PeerLock.Lock()
			conn := h.peers[c]
			h.PeerLock.Unlock()

			var mname StringMarshaler
			err := conn.GetData(&mname)
			if err != nil {
				dbg.Fatal("failed to establish connection: getting name:", err)
			}

			suite := h.suite
			pubkey := suite.Point()

			e := conn.GetData(pubkey)
			if e != nil {
				dbg.Fatal("unable to get pubkey from child")
			}
			conn.SetPubKey(pubkey)

			err = conn.PutData(h.Pubkey)
			if err != nil {
				dbg.Fatal("failed to send public key:", err)
			}

			h.PeerLock.Lock()
			h.Ready[c] = true
			h.peers[c] = conn
			h.PeerLock.Unlock()

			go func() {
				for {
					data := h.pool.Get().(BinaryUnmarshaler)
					err := conn.GetData(data)

					h.msgchan <- NetworkMessg{Data: data, From: conn.Name(), Err: err}
				}
			}()
		}(c)
	}
	return nil
}

// NewView creates a new view with the given view number, parent, and children.
func (h *GoHost) NewView(view int, parent string, children []string, hoslist []string) {
	h.views.NewView(view, parent, children, hoslist)
}

func (h *GoHost) NewViewFromPrev(view int, parent string) {
	h.views.NewViewFromPrev(view, parent)
}

func (h *GoHost) Parent(view int) string {
	return h.views.Parent(view)
}

// AddParent adds a parent node to the specified view.
func (h *GoHost) AddParent(view int, c string) {
	h.PeerLock.Lock()
	if _, ok := h.peers[c]; !ok {
		h.peers[c], _ = NewGoConn(h.dir, h.name, c)
	}
	h.PeerLock.Unlock()

	h.views.AddParent(view, c)
}

// AddChildren adds children to the specified view.
func (h *GoHost) AddChildren(view int, cs ...string) {
	for _, c := range cs {
		h.PeerLock.Lock()
		if _, ok := h.peers[c]; !ok {
			h.peers[c], _ = NewGoConn(h.dir, h.name, c)
		}
		h.PeerLock.Unlock()
		h.views.AddChildren(view, c)
	}
}

func (h *GoHost) AddPeerToPending(p string) {
	h.PendingPeers[p] = true
}

func (h *GoHost) AddPeerToHostlist(view int, name string) {
	h.views.AddPeerToHostlist(view, name)
}

func (h *GoHost) RemovePeerFromHostlist(view int, name string) {
	h.views.RemovePeerFromHostlist(view, name)
}

func (h *GoHost) AddPendingPeer(view int, name string) error {
	h.PeerLock.Lock()
	if _, ok := h.PendingPeers[name]; !ok {
		dbg.Error("Attempt to add peer not present in pending Peers")
		h.PeerLock.Unlock()
		return errors.New("attempted to add peer not present in pending peers")
	}

	h.PeerLock.Unlock()
	h.ConnectTo(name)

	h.views.AddChildren(view, name)
	return nil
}

func (h *GoHost) RemovePendingPeer(peer string) {
	h.PeerLock.Lock()
	delete(h.PendingPeers, peer)
	h.PeerLock.Unlock()
}

func (h *GoHost) RemovePeer(view int, name string) bool {
	return h.views.RemovePeer(view, name)
}

// Close closes the connections.
func (h *GoHost) Close() {
	dbg.Printf("closing gohost: %p", h)
	h.dir.Close()

	h.PeerLock.Lock()
	for _, c := range h.peers {
		c.Close()
	}
	h.PeerLock.Unlock()

	atomic.SwapInt64(&h.closed, 1)
}

func (h *GoHost) Closed() bool {
	return atomic.LoadInt64(&h.closed) == 1
}

// NChildren returns the number of children specified by the given view.
func (h *GoHost) NChildren(view int) int {
	return h.views.NChildren(view)
}

func (h *GoHost) HostListOn(view int) []string {
	return h.views.HostList(view)
}

func (h *GoHost) SetHostList(view int, hostlist []string) {
	h.views.SetHostList(view, hostlist)
}

// Name returns the hostname of the Host.
func (h *GoHost) Name() string {
	return h.name
}

// IsRoot returns true if this Host is the root of the specified view.
func (h *GoHost) IsRoot(view int) bool {
	return h.views.Parent(view) == ""
}

// IsParent returns true if the peer is the Parent of the specifired view.
func (h *GoHost) IsParent(view int, peer string) bool {
	return h.views.Parent(view) == peer
}

// IsChild returns true if the peer is a Child for the specified view.
func (h *GoHost) IsChild(view int, peer string) bool {
	h.PeerLock.Lock()
	_, ok := h.peers[peer]
	h.PeerLock.Unlock()
	return !h.IsParent(view, peer) && ok
}

// Peers returns the list of Peers as a mapping from hostname to Conn
func (h *GoHost) Peers() map[string]Conn {
	return h.peers
}

// Children returns the children in the specified view.
func (h *GoHost) Children(view int) map[string]Conn {
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

// AddPeers adds the list of Peers to the host.
func (h *GoHost) AddPeers(cs ...string) {
	h.PeerLock.Lock()
	for _, c := range cs {
		h.peers[c], _ = NewGoConn(h.dir, h.name, c)
	}
	h.PeerLock.Unlock()
}

func (h *GoHost) PutTo(ctx context.Context, host string, data BinaryMarshaler) error {
	done := make(chan error)
	var canceled int64
	go func() {
		for {
			if atomic.LoadInt64(&canceled) == 1 {
				return
			}
			h.PeerLock.Lock()
			Ready := h.Ready[host]
			parent := h.peers[host]
			h.PeerLock.Unlock()

			if Ready {
				// if closed put will return ErrClosed
				done <- parent.PutData(data)
				return
			}
			time.Sleep(250 * time.Millisecond)
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

// PutUp sends a message to the parent on the given view, potentially timing out.
func (h *GoHost) PutUp(ctx context.Context, view int, data BinaryMarshaler) error {
	// defer fmt.Println(h.Name(), "done put up", h.parent)
	pname := h.views.Parent(view)
	done := make(chan error)
	var canceled int64
	go func() {
		for {
			if atomic.LoadInt64(&canceled) == 1 {
				return
			}
			h.PeerLock.Lock()
			Ready := h.Ready[pname]
			parent := h.peers[pname]
			h.PeerLock.Unlock()

			if Ready {
				// if closed put will return ErrClosed
				done <- parent.PutData(data)
				return
			}
			time.Sleep(250 * time.Millisecond)
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

// PutDown sends messages to its children on the given view, potentially timing out.
func (h *GoHost) PutDown(ctx context.Context, view int, data []BinaryMarshaler) error {
	var err error
	var errLock sync.Mutex
	children := h.views.Children(view)
	if len(data) != len(children) {
		panic("number of messages passed down != number of children")
	}
	var canceled int64
	var wg sync.WaitGroup
	for i, c := range children {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			for {
				if atomic.LoadInt64(&canceled) == 1 {
					return
				}
				h.PeerLock.Lock()
				Ready := h.Ready[c]
				conn := h.peers[c]
				h.PeerLock.Unlock()

				if Ready {
					e := conn.PutData(data[i])
					if e != nil {
						errLock.Lock()
						err = e
						errLock.Unlock()
					}
					return
				}
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
		dbg.Error("DEADLINE EXCEEDED")
		err = ctx.Err()
		atomic.StoreInt64(&canceled, 1)
	}
	return err
}

// Get returns two channels. One of messages that are received, and another of errors
// associated with each message.
func (h *GoHost) GetNetworkMessg() chan NetworkMessg {
	return h.msgchan
}

// Pool returns the underlying pool of objects for creating new BinaryUnmarshalers,
// when Getting from network connections.
func (h *GoHost) Pool() *sync.Pool {
	return h.pool
}

func (h *GoHost) Pending() map[string]bool {
	return h.PendingPeers
}

// SetPool sets the pool of underlying objects for creating new BinaryUnmarshalers,
// when Getting from network connections.
func (h *GoHost) SetPool(p *sync.Pool) {
	h.pool = p
}
