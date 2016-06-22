package sda

import (
	"errors"
	"strconv"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
)

// LocalTest represents all that is needed for a local test-run
type LocalTest struct {
	// A map of ServerIdentity.Id to Hosts
	Hosts map[network.ServerIdentityID]*Host
	// A map of ServerIdentity.Id to Overlays
	Overlays map[network.ServerIdentityID]*Overlay
	// A map of ServerIdentity.Id to Services
	Services map[network.ServerIdentityID]map[ServiceID]Service
	// A map of Roster.Id to Rosters
	Rosters map[RosterID]*Roster
	// A map of Tree.Id to Trees
	Trees map[TreeID]*Tree
	// All single nodes
	Nodes []*TreeNodeInstance
}

// NewLocalTest creates a new Local handler that can be used to test protocols
// locally
func NewLocalTest() *LocalTest {
	return &LocalTest{
		Hosts:    make(map[network.ServerIdentityID]*Host),
		Overlays: make(map[network.ServerIdentityID]*Overlay),
		Services: make(map[network.ServerIdentityID]map[ServiceID]Service),
		Rosters:  make(map[RosterID]*Roster),
		Trees:    make(map[TreeID]*Tree),
		Nodes:    make([]*TreeNodeInstance, 0, 1),
	}
}

// StartProtocol takes a name and a tree and will create a
// new Node with the protocol 'name' running from the tree-root
func (l *LocalTest) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	rootServerIdentityID := t.Root.ServerIdentity.ID
	for _, h := range l.Hosts {
		if h.ServerIdentity.ID.Equal(rootServerIdentityID) {
			// XXX do we really need multiples overlays ? Can't we just use the
			// Node, since it is already dispatched as like a TreeNode ?
			return l.Overlays[h.ServerIdentity.ID].StartProtocol(t, name)
		}
	}
	return nil, errors.New("Didn't find host for tree-root")
}

// CreateProtocol takes a name and a tree and will create a
// new Node with the protocol 'name' without running it
func (l *LocalTest) CreateProtocol(t *Tree, name string) (ProtocolInstance, error) {
	rootServerIdentityID := t.Root.ServerIdentity.ID
	for _, h := range l.Hosts {
		if h.ServerIdentity.ID.Equal(rootServerIdentityID) {
			// XXX do we really need multiples overlays ? Can't we just use the
			// Node, since it is already dispatched as like a TreeNode ?
			return l.Overlays[h.ServerIdentity.ID].CreateProtocolSDA(t, name)
		}
	}
	return nil, errors.New("Didn't find host for tree-root")
}

// GenLocalHosts returns a slice of 'n' Hosts. If 'connect' is true, the
// hosts will be connected between each other. If 'processMsg' is true,
// the ProcessMsg-method will be called.
func (l *LocalTest) GenLocalHosts(n int, connect, processMsg bool) []*Host {
	hosts := GenLocalHosts(n, connect, processMsg)
	for _, host := range hosts {
		l.Hosts[host.ServerIdentity.ID] = host
		l.Overlays[host.ServerIdentity.ID] = host.overlay
		l.Services[host.ServerIdentity.ID] = host.serviceStore.services
	}
	return hosts
}

// GenTree will create a tree of n hosts. If connect is true, they will
// be connected to the root host. If register is true, the Roster and Tree
// will be registered with the overlay.
func (l *LocalTest) GenTree(n int, connect, processMsg, register bool) ([]*Host, *Roster, *Tree) {
	hosts := l.GenLocalHosts(n, connect, processMsg)

	list := l.GenRosterFromHost(hosts...)
	tree := list.GenerateBinaryTree()
	l.Trees[tree.ID] = tree
	if register {
		hosts[0].overlay.RegisterRoster(list)
		hosts[0].overlay.RegisterTree(tree)
	}
	return hosts, list, tree
}

// GenBigTree will create a tree of n hosts. If connect is true, they will
// be connected to the root host. If register is true, the Roster and Tree
// will be registered with the overlay.
// 'nbrHosts' is how many hosts are created
// 'nbrTreeNodes' is how many TreeNodes are created
// nbrHosts can be smaller than nbrTreeNodes, in which case a given host will
// be used more than once in the tree.
func (l *LocalTest) GenBigTree(nbrTreeNodes, nbrHosts, bf int, connect bool, register bool) ([]*Host, *Roster, *Tree) {
	hosts := l.GenLocalHosts(nbrHosts, connect, true)

	list := l.GenRosterFromHost(hosts...)
	tree := list.GenerateBigNaryTree(bf, nbrTreeNodes)
	l.Trees[tree.ID] = tree
	if register {
		hosts[0].overlay.RegisterRoster(list)
		hosts[0].overlay.RegisterTree(tree)
	}
	return hosts, list, tree
}

// GenRosterFromHost takes a number of hosts as arguments and creates
// an Roster.
func (l *LocalTest) GenRosterFromHost(hosts ...*Host) *Roster {
	var entities []*network.ServerIdentity
	for i := range hosts {
		entities = append(entities, hosts[i].ServerIdentity)
	}
	list := NewRoster(entities)
	l.Rosters[list.ID] = list
	return list
}

// CloseAll takes a list of hosts that will be closed
func (l *LocalTest) CloseAll() {
	for _, host := range l.Hosts {
		err := host.Close()
		if err != nil {
			log.Error("Closing host", host.ServerIdentity.First(),
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

// NewTreeNodeInstance creates a new node on a TreeNode
func (l *LocalTest) NewTreeNodeInstance(tn *TreeNode, protName string) (*TreeNodeInstance, error) {
	o := l.Overlays[tn.ServerIdentity.ID]
	if o == nil {
		return nil, errors.New("Didn't find corresponding overlay")
	}
	tree := l.GetTree(tn)
	if tree == nil {
		return nil, errors.New("Didn't find tree corresponding to TreeNode")
	}
	protID := ProtocolNameToID(protName)
	if !ProtocolExists(protID) {
		return nil, errors.New("Didn't find protocol: " + protName)
	}
	tok := &Token{
		ProtoID:    protID,
		RosterID:   tree.Roster.ID,
		TreeID:     tree.ID,
		TreeNodeID: tn.ID,
		RoundID:    RoundID(uuid.NewV4()),
	}
	node := newTreeNodeInstance(o, tok, tn)
	l.Nodes = append(l.Nodes, node)
	return node, nil
}

// GetNodes returns all Nodes that belong to a treeNode
func (l *LocalTest) GetNodes(tn *TreeNode) []*TreeNodeInstance {
	var nodes []*TreeNodeInstance
	for _, n := range l.Overlays[tn.ServerIdentity.ID].instances {
		nodes = append(nodes, n)
	}
	return nodes
}

// SendTreeNode injects a message directly in the Overlay-layer, bypassing
// Host and Network
func (l *LocalTest) SendTreeNode(proto string, from, to *TreeNodeInstance, msg network.Body) error {
	if from.Tree().ID != to.Tree().ID {
		return errors.New("Can't send from one tree to another")
	}
	b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}
	sdaMsg := &ProtocolMsg{
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
func (l *LocalTest) CheckPendingTreeMarshal(h *Host, el *Roster) {
	h.checkPendingTreeMarshal(el)
}

// GetPrivate returns the private key of a host
func (l *LocalTest) GetPrivate(h *Host) abstract.Scalar {
	return h.private
}

// GetServices returns a slice of all services asked for.
// The sid is the id of the service that will be collected.
func (l *LocalTest) GetServices(hosts []*Host, sid ServiceID) []Service {
	services := make([]Service, len(hosts))
	for i, h := range hosts {
		services[i] = l.Services[h.ServerIdentity.ID][sid]
	}
	return services
}

// MakeHELS is an abbreviation to make a Host, an Roster, and a service.
// It returns the service of the first host in the slice.
func (l *LocalTest) MakeHELS(nbr int, sid ServiceID) ([]*Host, *Roster, Service) {
	hosts := l.GenLocalHosts(nbr, false, true)
	el := l.GenRosterFromHost(hosts...)
	return hosts, el, l.Services[hosts[0].ServerIdentity.ID][sid]
}

// NewLocalHost creates a new host with the given address and registers it.
func NewLocalHost(port int) *Host {
	address := "localhost:" + strconv.Itoa(port)
	priv, pub := PrivPub()
	id := network.NewServerIdentity(pub, address)
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
		log.Lvlf3("Listening on %s %x", host.ServerIdentity.First(), host.ServerIdentity.ID)
		if processMessages {
			host.StartProcessMessages()
		}
		if connect && root != host {
			log.Lvl4("Connecting", host.ServerIdentity.First(), host.ServerIdentity.ID, "to",
				root.ServerIdentity.First(), root.ServerIdentity.ID)
			if _, err := host.Connect(root.ServerIdentity); err != nil {
				log.Fatal(host.ServerIdentity.Addresses, "Could not connect hosts", root.ServerIdentity.Addresses, err)
			}
			// Wait for connection accepted in root
			connected := false
			for !connected {
				time.Sleep(time.Millisecond * 10)
				root.networkLock.Lock()
				for id := range root.connections {
					if id.Equal(host.ServerIdentity.ID) {
						connected = true
						break
					}
				}
				root.networkLock.Unlock()
			}
			log.Lvl4(host.ServerIdentity.First(), "is connected to root")
		}
	}
	return hosts
}

// PrivPub creates a private/public key pair.
func PrivPub() (abstract.Scalar, abstract.Point) {
	keypair := config.NewKeyPair(network.Suite)
	return keypair.Secret, keypair.Public
}
