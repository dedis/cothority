package sda

import (
	"errors"
	"strconv"

	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
)

// LocalTest represents all that is needed for a local test-run
type LocalTest struct {
	// A map of Entity.Id to Hosts
	Hosts map[network.EntityID]*Host
	// A map of Entity.Id to Overlays
	Overlays map[network.EntityID]*Overlay
	// A map of Entity.Id to Services
	Services map[network.EntityID]map[ServiceID]Service
	// A map of EntityList.Id to EntityLists
	EntityLists map[EntityListID]*EntityList
	// A map of Tree.Id to Trees
	Trees map[TreeID]*Tree
	// All single nodes
	Nodes []*TreeNodeInstance
}

// NewLocalTest creates a new Local handler that can be used to test protocols
// locally
func NewLocalTest() *LocalTest {
	return &LocalTest{
		Hosts:       make(map[network.EntityID]*Host),
		Overlays:    make(map[network.EntityID]*Overlay),
		Services:    make(map[network.EntityID]map[ServiceID]Service),
		EntityLists: make(map[EntityListID]*EntityList),
		Trees:       make(map[TreeID]*Tree),
		Nodes:       make([]*TreeNodeInstance, 0, 1),
	}
}

// StartProtocol takes a name and a tree and will create a
// new Node with the protocol 'name' running from the tree-root
func (l *LocalTest) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	rootEntityId := t.Root.Entity.ID
	for _, h := range l.Hosts {
		if h.Entity.ID.Equals(rootEntityId) {
			// XXX do we really need multiples overlays ? Can't we just use the
			// Node, since it is already dispatched as like a TreeNode ?
			return l.Overlays[h.Entity.ID].StartProtocol(t, name)
		}
	}
	return nil, errors.New("Didn't find host for tree-root")
}

// CreateNewNodeName takes a name and a tree and will create a
// new Node with the protocol 'name' without running it
func (l *LocalTest) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	rootEntityId := t.Root.Entity.ID
	for _, h := range l.Hosts {
		if h.Entity.ID.Equals(rootEntityId) {
			// XXX do we really need multiples overlays ? Can't we just use the
			// Node, since it is already dispatched as like a TreeNode ?
			return l.Overlays[h.Entity.ID].CreateProtocol(t, name)
		}
	}
	return nil, errors.New("Didn't find host for tree-root")
}

// GenLocalHost returns a slice of 'n' Hosts. If 'connect' is true, the
// hosts will be connected between each other. If 'processMsg' is true,
// the ProcessMsg-method will be called.
func (l *LocalTest) GenLocalHosts(n int, connect, processMsg bool) []*Host {
	hosts := GenLocalHosts(n, connect, processMsg)
	for _, host := range hosts {
		l.Hosts[host.Entity.ID] = host
		l.Overlays[host.Entity.ID] = host.overlay
		l.Services[host.Entity.ID] = host.serviceStore.services
	}
	return hosts
}

// GenTree will create a tree of n hosts. If connect is true, they will
// be connected to the root host. If register is true, the EntityList and Tree
// will be registered with the overlay.
func (l *LocalTest) GenTree(n int, connect, processMsg, register bool) ([]*Host, *EntityList, *Tree) {
	hosts := l.GenLocalHosts(n, connect, processMsg)

	list := l.GenEntityListFromHost(hosts...)
	tree := list.GenerateBinaryTree()
	l.Trees[tree.Id] = tree
	if register {
		hosts[0].overlay.RegisterEntityList(list)
		hosts[0].overlay.RegisterTree(tree)
	}
	return hosts, list, tree
}

// GenBigTree will create a tree of n hosts. If connect is true, they will
// be connected to the root host. If register is true, the EntityList and Tree
// will be registered with the overlay.
// 'nbrHosts' is how many hosts are created
// 'nbrTreeNodes' is how many TreeNodes are created
// nbrHosts can be smaller than nbrTreeNodes, in which case a given host will
// be used more than once in the tree.
func (l *LocalTest) GenBigTree(nbrTreeNodes, nbrHosts, bf int, connect bool, register bool) ([]*Host, *EntityList, *Tree) {
	hosts := l.GenLocalHosts(nbrHosts, connect, true)

	list := l.GenEntityListFromHost(hosts...)
	tree := list.GenerateBigNaryTree(bf, nbrTreeNodes)
	l.Trees[tree.Id] = tree
	if register {
		hosts[0].overlay.RegisterEntityList(list)
		hosts[0].overlay.RegisterTree(tree)
	}
	return hosts, list, tree
}

// GenEntityListFromHosts takes a number of hosts as arguments and creates
// an EntityList.
func (l *LocalTest) GenEntityListFromHost(hosts ...*Host) *EntityList {
	var entities []*network.Entity
	for i := range hosts {
		entities = append(entities, hosts[i].Entity)
	}
	list := NewEntityList(entities)
	l.EntityLists[list.Id] = list
	return list
}

// CloseAll takes a list of hosts that will be closed
func (l *LocalTest) CloseAll() {
	for _, host := range l.Hosts {
		err := host.Close()
		if err != nil {
			dbg.Error("Closing host", host.Entity.First(),
				"gives error", err)
		}
	}
	for _, node := range l.Nodes {
		node.Close()
	}
}

// GetTree returns the tree of the given TreeNode
func (l *LocalTest) GetTree(tn *TreeNode) *Tree {
	var tree *Tree
	for _, t := range l.Trees {
		if tn.IsInTree(t) {
			tree = t
			break
		}
	}
	return tree
}

// NewNode creates a new node on a TreeNode
func (l *LocalTest) NewTreeNodeInstance(tn *TreeNode, protName string) (*TreeNodeInstance, error) {
	o := l.Overlays[tn.Entity.ID]
	if o == nil {
		return nil, errors.New("Didn't find corresponding overlay")
	}
	tree := l.GetTree(tn)
	if tree == nil {
		return nil, errors.New("Didn't find tree corresponding to TreeNode")
	}
	protId := ProtocolNameToID(protName)
	if !ProtocolExists(protId) {
		return nil, errors.New("Didn't find protocol: " + protName)
	}
	tok := &Token{
		ProtoID:      protId,
		EntityListID: tree.EntityList.Id,
		TreeID:       tree.Id,
		TreeNodeID:   tn.Id,
		RoundID:      RoundID(uuid.NewV4()),
	}
	node := newTreeNodeInstance(o, tok, tn)
	l.Nodes = append(l.Nodes, node)
	return node, nil
}

// GetNodes returns all Nodes that belong to a treeNode
func (l *LocalTest) GetNodes(tn *TreeNode) []*TreeNodeInstance {
	nodes := make([]*TreeNodeInstance, 0)
	for _, n := range l.Overlays[tn.Entity.ID].instances {
		nodes = append(nodes, n)
	}
	return nodes
}

// SendTreeNode injects a message directly in the Overlay-layer, bypassing
// Host and Network
func (l *LocalTest) SendTreeNode(proto string, from, to *TreeNodeInstance, msg network.ProtocolMessage) error {
	if from.Tree().Id != to.Tree().Id {
		return errors.New("Can't send from one tree to another")
	}
	b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}
	sdaMsg := &Data{
		MsgSlice: b,
		MsgType:  network.TypeToMessageTypeID(msg),
		From:     from.token,
		To:       to.token,
	}
	return to.overlay.TransmitMsg(sdaMsg)
}

// AddPendingTreeMarshal takes a treeMarshal and adds it to the list of the
// known trees, also triggering dispatching of SDA-messages waiting for that
// tree
func (l *LocalTest) AddPendingTreeMarshal(h *Host, tm *TreeMarshal) {
	h.addPendingTreeMarshal(tm)
}

// CheckPendingTreeMarshal looks whether there are any treeMarshals to be
// called
func (l *LocalTest) CheckPendingTreeMarshal(h *Host, el *EntityList) {
	h.checkPendingTreeMarshal(el)
}

// GetPrivate returns the private key of a host
func (l *LocalTest) GetPrivate(h *Host) abstract.Secret {
	return h.private
}

// NewLocalHost creates a new host with the given address and registers it.
func NewLocalHost(port int) *Host {
	address := "localhost:" + strconv.Itoa(port)
	priv, pub := PrivPub()
	id := network.NewEntity(pub, address)
	return NewHost(id, priv)
}

// GenLocalHosts will create n hosts with the first one being connected to each of
// the other nodes if connect is true.
func GenLocalHosts(n int, connect bool, processMessages bool) []*Host {

	hosts := make([]*Host, n)
	for i := 0; i < n; i++ {
		host := NewLocalHost(2000 + i*10)
		hosts[i] = host
	}
	root := hosts[0]
	for _, host := range hosts {
		host.ListenAndBind()
		dbg.Lvl3("Listening on", host.Entity.First(), host.Entity.ID)
		if processMessages {
			host.StartProcessMessages()
		}
		if connect && root != host {
			dbg.Lvl4("Connecting", host.Entity.First(), host.Entity.ID, "to",
				root.Entity.First(), root.Entity.ID)
			if _, err := host.Connect(root.Entity); err != nil {
				dbg.Fatal(host.Entity.Addresses, "Could not connect hosts", root.Entity.Addresses, err)
			}
			// Wait for connection accepted in root
			connected := false
			for !connected {
				time.Sleep(time.Millisecond * 10)
				root.entityListsLock.RLock()
				for id, _ := range root.entities {
					if id.Equals(host.Entity.ID) {
						connected = true
						break
					}
				}
				root.entityListsLock.RUnlock()
			}
			dbg.Lvl4(host.Entity.First(), "is connected to root")
		}
	}
	return hosts
}

// PrivPub creates a private/public key pair.
func PrivPub() (abstract.Secret, abstract.Point) {
	keypair := config.NewKeyPair(network.Suite)
	return keypair.Secret, keypair.Public
}
