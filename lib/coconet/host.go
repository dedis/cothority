package coconet

import (
	"sync"

	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// Host is an abstract node on the Host tree.
// Representing this tree, it has the ability to PutUp and PutDown
// to its parent and children respectively.
// It also can Get from all its connections.
// It is up to the caller to de-multiplex the messages sent back from Get.
//
// A Host also implements multiple views of this tree.
// Each view is a unique tree (parent, children) configuration.
// Applications using Hosts can specify which view to use for operations.
type Host interface {
	// Name returns the name of this host.
	Name() string

	Parent(view int) string

	// Peers returns a mapping from peer names to Connections.
	Peers() map[string]Conn

	// AddPeers adds new peers to the host, but does not make it either child or parent.
	AddPeers(hostnames ...string)

	// NewView creates a NewView to operate on.
	// It creates a view with the given view number, which corresponds to the tree
	// with the specified parent and children.
	NewView(view int, parent string, children []string, hostlist []string)

	NewViewFromPrev(view int, parent string)

	// Returns map of this host's views
	Views() *Views

	// AddParent adds a parent to the view specified.
	// Used for building a view incrementally.
	AddParent(view int, hostname string)
	// AddChildren adds children to the view specified.
	// Used for building a view incrementally.
	AddChildren(view int, hostname ...string)

	// NChildren returns the number of children for the given view.
	NChildren(view int) int
	// Children returns the children for a given view.
	Children(view int) map[string]Conn
	// Returns list of hosts available on a specific view
	HostListOn(view int) []string
	// Set the hoslist on a specific view
	SetHostList(view int, hostlist []string)

	// IsRoot returns true if this host is the root for the given view.
	IsRoot(view int) bool
	// IsParent returns true if the given peer is the parent for a specified view.
	IsParent(view int, peer string) bool
	// IsChild returns true if the given peer is the child for a specified view.
	IsChild(view int, peer string) bool

	// PutUp puts the given data up to the parent in the specified view.
	// The context is used to timeout the request.
	PutUp(ctx context.Context, view int, data BinaryMarshaler) error
	// PutDown puts the given data down to the children in the specified view.
	// The context is used to timeout the request.
	PutDown(ctx context.Context, view int, data []BinaryMarshaler) error

	PutTo(ctx context.Context, host string, data BinaryMarshaler) error

	// Get returns a channel on which all received messages will be put.
	// It always returns a reference to the same channel.
	// Multiple listeners will receive disjoint sets of messages.
	// When receiving from the channels always recieve from both the network
	// messages channel as well as the error channel.
	Get() chan NetworkMessg

	// Connect connects to the parent in the given view.
	Connect(view int) error
	ConnectTo(host string) error
	// Listen listens for incoming connections.
	Listen() error

	// Close closes all the connections in the Host.
	Close()

	// SetSuite sets the suite to use for the Host.
	SetSuite(abstract.Suite)
	// PubKey returns the public key of the Host.
	PubKey() abstract.Point
	// SetPubKey sets the public key of the Host.
	SetPubKey(abstract.Point)

	// Pool is a pool of BinaryUnmarshallers to use when generating NetworkMessg's.
	Pool() *sync.Pool
	// SetPool sets the pool of the Host.
	SetPool(*sync.Pool)

	// Functions to allow group evolution
	AddPeerToPending(h string)
	AddPendingPeer(view int, name string) error
	RemovePeer(view int, name string) bool
	Pending() map[string]bool

	AddPeerToHostlist(view int, name string)
	RemovePeerFromHostlist(view int, name string)
}
