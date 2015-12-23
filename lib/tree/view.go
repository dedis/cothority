package tree

import (
	"strings"
	"sync"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
)

// View represents  the set on what to run a cothority /or another protocol)
// It is higher level than the lib/tree and peerlist
type View struct {
	sync.RWMutex
	// The number of the view (XXX still relevant ? since we have tree.id() +
	// peeers.Id()
	Num int

	*Node
	*PeerList
}

func (v *View) Id() hashid.HashId {
	return append(v.Node.Id(), v.PeerList.Id()...)
}

func NewView(view int, node *Node, peerList *PeerList) *View {
	dbg.Lvl3("Creating new view", view, " with", len(peerList.Peers), " hosts")
	vi := &View{Num: view, Node: node, PeerList: peerList}
	return vi
}

// Generate a new view from this Views struct
func (v *Views) NewView(view int, node *Node, peerList *PeerList) {
	v.Lock()
	v.Views[view] = NewView(view, node, peerList)
	v.Unlock()
}

// XXX The following function are commented because I think they might be
// irrelevant with our new architecture (Tree + PeerList). A View is basically a
// Tree + PeerList, this pair is supposed to be unique. If we want to remove a a
// node in the tree we make a NewView out of this new tree and it will have a
// new Unique Id. We should not change the inners of a view...
/*// Remove a child from this view. Returns true if succeeded otherwise false*/
//func (v *View) RemoveChild(child string) bool {
//v.Lock()
//defer v.Unlock()

//if !v.Node.ParentOf(child) {
//return false
//}
//var i int
//for i, c := range v.Node.Children {
//if c.Name() == child {
//break
//}
//}

//v.Node.Children = append(v.Node.Children[:i], v.Node.Children[i+1:])
//// XXX should we remove also from the peer list ?? I think not because the
//// peer list is more constant within a view than a the actual tree (quickly
//// remove one malicious peer or whatever but keep the same peer list)
//return true
//}

//func (v *View) AddPeerToHostlist(name string) {
//m := make(map[string]bool)
//for _, h := range v.HostList {
//if h != name {
//m[h] = true
//}
//}
//m[name] = true
//hostlist := make([]string, 0, len(m))

//for h := range m {
//hostlist = append(hostlist, h)
//}

//sortable := sort.StringSlice(hostlist)
//sortable.Sort()
//v.HostList = []string(sortable)

//}

//func (v *View) RemovePeerFromHostlist(name string) {
//m := make(map[string]bool)
//for _, h := range v.HostList {
//if h != name {
//m[h] = true
//}
//}
//hostlist := make([]string, 0, len(m))

//for h := range m {
//hostlist = append(hostlist, h)
//}

//sortable := sort.StringSlice(hostlist)
//sortable.Sort()
//v.HostList = []string(sortable)
//}

//func (v *View) RemovePeer(name string) bool {
//dbg.Print("LOOKING FOR", name, "in HOSTLIST", v.HostList)
//v.Lock()
//// make sure we don't remove our parent
//if v.Parent == name {
//v.Unlock()
//return false
//}
//v.Unlock()

//removed := v.RemoveChild(name)

//v.Lock()
//defer v.Unlock()
//if len(v.HostList) == 0 {
//return false
//}

//v.RemovePeerFromHostlist(name)
//return removed
//}

type Views struct {
	sync.RWMutex
	Views map[int]*View
}

func NewViews() *Views {
	vs := &Views{Views: make(map[int]*View)}
	return vs
}

func (v *Views) AddView(viewNbr int, view *View) {
	v.Views[viewNbr] = view
}
func (v *Views) View(view int) *View {
	return v.Views[view]
}

// Returns the parent for this view
func (v *Views) Parent(view int) *Node {
	v.RLock()
	defer v.RUnlock()
	if vi := v.Views[view]; vi != nil {
		return vi.Parent
	}
	return nil
}

// Returns the peer list for this view
func (v *Views) PeerList(view int) *PeerList {
	v.RLock()
	defer v.RUnlock()
	if v.Views[view] == nil {
		return nil
	}
	return v.Views[view].PeerList
}

func (v *Views) Children(view int) []*Node {
	v.RLock()
	defer v.RUnlock()
	if view < len(v.Views) {
		return v.Views[view].Children
	} else {
		return nil
	}
}

func (v *Views) NChildren(view int) int {
	v.RLock()
	defer v.RUnlock()
	return v.Views[view].NChildren()
}

// Generate a peer list + Tree = View from the host list and the tree.
// The hostname is needed sa each view is "unique" depending on the host, i.e.
// the node will always be the node of the host, not the root, with its
// respective children and parent.
// You generally call that when launching a new cothority node since the
// graph.Tree comes from the config file.
// As it goes along the tree, it looks if it have a private or not.
// If not it generates a new  key pair for a host.
// Otherwise it justs the peer with the right public  private key pair
// It returns a View that consists of the tree and the peer list
func NewViewFromConfigTree(suite abstract.Suite, tree *ConfigTree, hostname string) *View {
	peers := make([]*Peer, 0)
	rootNode, hostNode := convertTree(suite, peers, tree, hostname)
	pl := NewPeerList(suite, peers)
	rootNode.GenId(suite.Hash())
	// here we create the local view !
	return NewView(0, hostNode, pl)
}

// convertTree is a recursive function that parse the ConfigTree and returns the
// Root node and the node corresponding to *host* in the tree.
func convertTree(suite abstract.Suite, peers []*Peer, t *ConfigTree, host string) (*Node, *Node) {
	if t.Name == "" {
		return nil, nil
	}
	var node *Node
	var hostNode *Node // the node corresponding to *host*
	// because we need  a local view from this host instead of the full tree.
	var secret abstract.Secret = suite.Secret()
	var public abstract.Point = suite.Point()
	var err error
	if t.Name == host && t.PriKey != "" {
		// decode private key
		secret, err = cliutils.ReadSecret64(suite, strings.NewReader(t.PriKey))
		if err != nil {
			dbg.Fatal("Can not decode private key for node", t.Name)
		}

	}
	if t.PubKey != "" {
		// decode the public and  create a Peer with theses keys
		public, err = cliutils.ReadPub64(suite, strings.NewReader(t.PubKey))
		if err != nil {
			dbg.Fatal("Can not decode public key for node", t.Name)
		}
	} else if t.PriKey != "" {
		// we can generate the corresponding public key anyway
		public = public.Mul(nil, secret)
	} else {
		// We don't have any public key for this host ??
		dbg.Fatal("No public key for this host")
	}
	// Create the peer + node (tree)
	p := NewPeer(t.Name, public, secret)
	peers = append(peers, p)
	node = NewTreeNode(p)
	if t.Name == host {
		hostNode = node
	}
	for i, _ := range t.Children {
		// create the children node
		if child, hostN := convertTree(suite, peers, t.Children[i], host); child != nil {
			// add them if we are not leaf
			node.AddChild(child)
			// if we still dont know where our host is, that the returned node
			// is not null and it corresponds
			if hostNode == nil && hostN != nil && hostN.Name() == host {
				// we have found our node
				hostNode = hostN
			}
		}
	}
	return node, hostNode
}
