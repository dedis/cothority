package tree

import (
	"bytes"
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"hash"
	"net"
)

/*
The tree library holds all hosts and allows for different ways to
retrieve the hosts - either in a n-ary tree, broadcast or just
 a selection.
*/

type TreeId struct {
	Depth    int
	TreeHash hashid.HashId
}

// TreeEntry is one entry in the tree and linked to it's parent and
// the tree-structure it's in.
type Node struct {
	// TreeId represents the ID of the tree that is computed from the list of
	// treenodes.
	TreeId *TreeId `toml:"-"`
	// A list of all child-nodes - the indexes are relative to the PeerList
	Children []*Node
	// The parent node - or nil if this is the root
	Parent *Node `toml:"-"`
	// The actual Peer stored in this node.
	*Peer
}

// Init a tree node from the ID and a peer
func (tn *Node) init(p *Peer) *Node {
	tn = &Node{
		Children: make([]*Node, 0),
		Peer:     p,
	}
	return tn
}

// Returns a fresh new TreeNode
func NewTreeNode(p *Peer) *Node {
	return new(Node).init(p)
}

// AddChild appends a node into the child list of this treenode. It also updates
// the Parent pointer of the child.
func (te *Node) AddChild(tn *Node) {
	te.Children = append(te.Children, tn)
	tn.Parent = te
}

// VisistsBFS will visits the tree BFS style calling the given function for each
// node encountered from the root.
func VisitsBFS(root *Node, fn func(*Node)) {
	fn(root)
	for _, child := range root.Children {
		VisitsBFS(child, fn)
	}
}

// CountRec counts the number of children recursively
func (te *Node) Count() int {
	nbr := 0
	VisitsBFS(te, func(tn *Node) {
		nbr += 1
	})
	return nbr
}

// Write simply write the peer representation into the writer. Used for hashing.
func (tn *Node) Bytes() []byte {
	buf := tn.Peer.Bytes()
	// if we have a parent
	if tn.Parent != nil {
		// we include the link from the parent to us in the hash
		buf = append(buf, tn.Parent.Peer.Bytes()...)
	}
	return buf
}

// Id will hash its whole topology to produce an TreeId. It will set the treeId
// field for each nodes in its topology
func (tn *Node) GenId(hashFunc hash.Hash) hashid.HashId {
	tid := &TreeId{}
	depth := 0
	// Visits the whole tree
	VisitsBFS(tn, func(node *Node) {
		// The node write itselfs
		hashFunc.Write(node.Bytes())
		// then sets the right fields
		node.TreeId = tid
		depth++
	})
	// Set the hashid
	tid.TreeHash = hashid.HashId(hashFunc.Sum(nil))
	tid.Depth = depth
	return tid.TreeHash
}

// Id() returns the id
func (tn *Node) Id() hashid.HashId {
	return tn.TreeId.TreeHash
}

// How many children does this node has
func (tn *Node) NChildren() int {
	return len(tn.Children)
}

// Name of the underlying peer
func (tn *Node) Name() string {
	return tn.Peer.Name
}

// ROot returns true if this node is the root of the tree
func (tn *Node) Root() bool {
	return tn.Parent == nil
}

func (tn *Node) Leaf() bool {
	return len(tn.Children) == 0
}

// returns true if this node is the child of the given node
func (tn *Node) ChildOf(parent string) bool {
	return tn.Parent.Name() == parent
}

// returns true if this node is the parent of the given node
func (tn *Node) ParentOf(child string) bool {
	for _, c := range tn.Children {
		if c.Name() == child {
			return true
		}
	}
	return false
}

// Config Tree is used to write in and from a config file. All keys are encoded
// as strings (in hex or b64). This is needed because of the lack of custom
// decoding from the TOML library.
// When you want you have decoded the config file, you will have this tree
// structure. From it you can call  NewViewFromConfigTree that will generate a
// local view for a specified host (View = PeerList + Node)
type ConfigTree struct {
	Name string
	// hex encoded public and private keys
	PriKey   string
	PubKey   string
	Children []*ConfigTree
}

func (c *ConfigTree) Count() int {
	var count int
	fn := func(ct *ConfigTree) {
		count += 1
	}
	c.Visit(fn)
	return count
}

func (c *ConfigTree) Visit(fn func(*ConfigTree)) {
	fn(c)
	for i := range c.Children {
		c.Children[i].Visit(fn)
	}
}

// NewNaryTree creates a regular config tree with a branching factor bf from the list
// of peers "peers". It returns the root.
// Usually used for localhost testing purposes
func NewNaryTree(s abstract.Suite, peerList *PeerList, bf int) *ConfigTree {
	if len(peerList.Peers) < 1 {
		return nil
	}
	peers := peerList.Peers
	dbg.Lvl3("NewNaryTree Called with", len(peers), "peers and bf =", bf)
	root := NewConfigTree(s, peers[0])
	var index int = 1
	bfs := make([]*ConfigTree, 1)
	bfs[0] = root
	for len(bfs) > 0 && index < len(peers) {
		t := bfs[0]
		t.Children = make([]*ConfigTree, 0)
		lbf := 0
		// create space for enough children
		// init them
		for lbf < bf && index < len(peers) {
			child := NewConfigTree(s, peers[index])
			// append the children to the list of trees to visit
			bfs = append(bfs, child)
			t.Children = append(t.Children, child)
			index += 1
			lbf += 1
		}
		bfs = bfs[1:]
	}
	return root
}

func GenNaryTree(s abstract.Suite, names []string, bf int) *ConfigTree {
	pl := GenPeerList(s, names)
	return NewNaryTree(s, pl, bf)
}

func NewConfigTree(suite abstract.Suite, p *Peer) *ConfigTree {
	var bpriv bytes.Buffer
	if err := cliutils.WriteSecret64(suite, &bpriv, p.Secret); err != nil {
		dbg.Fatal("Error while writing secret key of peer", p.Name, " for NewConfigTree")
	}
	var bpub bytes.Buffer
	if err := cliutils.WritePub64(suite, &bpub, p.Public); err != nil {
		dbg.Fatal("Error while writing public key of peer", p.Name, "for NewConfigTree")
	}
	return &ConfigTree{
		Name:     p.Name,
		PriKey:   bpriv.String(),
		PubKey:   bpub.String(),
		Children: make([]*ConfigTree, 0),
	}
}

func (ct *ConfigTree) AddChild(child *ConfigTree) {
	ct.Children = append(ct.Children, child)
}

// NewColorTree creates a tree such that given machines ip addresses, it creates
// a tree with hpn nodes per machines and such that no nodes on the same
// physical machines are directly link (father-child) in the tree.
// peerList is the list of peers we want to run a co-tree on
// peerPerMachine = # of process to run per machine
// bf = branching factor of the tree
// startPort at which point do
// This function is mostly used in controlled testbed environments such as
// emulabs. You should call this function in order to create your test Tree,
// then you can write that tree to the config file.
func NewColorConfigTree(peerList *PeerList, peerPerMachine int, bf int) (
	*ConfigTree, error) {

	// Map from nodes to their hosts
	mp := make(map[string][]*Peer)

	// split by ip address
	for _, peer := range peerList.Peers {
		// look at the machine it is in
		baseIp, _, err := net.SplitHostPort(peer.Name)
		if err != nil {
			return nil, err
		}
		mp[baseIp] = append(mp[baseIp], peer)
	}
	// which machine is supposed to be the root
	startMIp, _, _ := net.SplitHostPort(peerList.Peers[0].Name)
	root, peers := ColorTree(peerList.Suite, peerList, peerPerMachine, bf, startMIp, mp)
	if len(peerList.Peers) != len(peers) {
		return nil, fmt.Errorf("Error could not create enough peers on the colored tree peerlist= %d vs Tree.Count %d vs peers returned %d", len(peerList.Peers), root.Count(), len(peers))
	}
	return root, nil
}

func GenColorConfigTree(suite abstract.Suite, names []string, ppm, bf int) (*ConfigTree, error) {
	pl := GenPeerList(suite, names)
	return NewColorConfigTree(pl, ppm, bf)
}

// ColorTree takes a peerList with already public / private key generated,
// a Host Per Node or Process Per Machines, a branching factor and the map
// between machines and nodes.
// It returns a tree if possible such that each node do not have a direct link
// between a process on the same machine.
func ColorTree(suite abstract.Suite, peerList *PeerList, hostsPerNode int, bf int, startM string, mp map[string][]*Peer) (
	*ConfigTree, []*Peer) {

	nodesTouched := make([]string, 0)
	nodesTouched = append(nodesTouched, startM)

	rootHost := mp[startM][0]
	mp[startM] = mp[startM][1:]

	hostsCreated := make([]*Peer, 0)
	hostsCreated = append(hostsCreated, rootHost)
	depth := make([]int, 0)
	depth = append(depth, 1)

	hostTNodes := make([]*ConfigTree, 0)
	rootTNode := NewConfigTree(suite, rootHost)
	hostTNodes = append(hostTNodes, rootTNode)

	for i := 0; i < len(hostsCreated); i++ {
		curHost := hostsCreated[i]
		curDepth := depth[i]
		curTNode := hostTNodes[i]
		curNode, _, _ := net.SplitHostPort(curHost.Name)

		for c := 0; c < bf; c++ {
			var newHost *Peer
			nodesTouched, newHost = GetFirstFreeNode(nodesTouched, mp, curNode)
			// Finished
			if newHost == nil {
				return rootTNode, hostsCreated
				// break
			}

			// create Tree Node for the new host
			newHostTNode := NewConfigTree(suite, newHost)
			curTNode.AddChild(newHostTNode)

			// keep track of created hosts and nodes
			hostsCreated = append(hostsCreated, newHost)
			depth = append(depth, curDepth+1)
			hostTNodes = append(hostTNodes, newHostTNode)

			// keep track of machines used in FIFO order
			node, _, _ := net.SplitHostPort(newHost.Name)
			nodesTouched = append(nodesTouched, node)
		}
	}
	return rootTNode, hostsCreated
}

// Go through list of nodes(machines) and choose a hostName on the first node that
// still has room for more hosts on it and is != curNode
// If such a machine does not exist, loop through the map from machine names
// to their available host names, and choose a name on the first free machine != curNode
// Return updated nodes and the chosen hostName
func GetFirstFreeNode(nodes []string, mp map[string][]*Peer, curNode string) (
	[]string, *Peer) {
	var chosen *Peer
	uNodes := make([]string, 0)

	// loop through recently selected machines that already have hosts
	var i int
	var node string
	for i, node = range nodes {
		if node != curNode {
			if len(mp[node]) > 0 {
				// choose hostname on this node
				chosen = mp[node][0]
				mp[node] = mp[node][1:]

				// if still not fully used, add to updated nodes
				if len(mp[node]) > 0 {
					uNodes = append(uNodes, node)
				}
				break
			} else {
				// remove full node
				delete(mp, node)
			}
		}
	}

	// keep in list nodes after chosen node
	for ; i < len(nodes); i++ {
		uNodes = append(uNodes, node)
	}

	if chosen != nil {
		// we were able to make a choice
		// but all recently seen nodes before 'i' were fully used
		return uNodes, chosen
	}

	// all recently seen nodes were fully used or == curNode
	// must choose free machine from map
	for node := range mp {
		if node != curNode {
			if len(mp[node]) > 0 {
				chosen = mp[node][0]
				mp[node] = mp[node][1:]
				break
			}
		}
	}

	return uNodes, chosen
}
