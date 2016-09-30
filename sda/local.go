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
	// are we running tcp or local layer
	mode string
	// the context for the local connections
	// it enables to have multiple local test running simultaneously
	ctx *network.LocalManager
}

const (
	// TCP represents the TCP mode of networking for this local test
	TCP = "tcp"
	// Local represents the Local mode of networking for this local test
	Local = "local"
)

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
		mode:     Local,
		ctx:      network.NewLocalManager(),
	}
}

// NewTCPTest returns a LocalTest but using a TCPRouter as the underlying
// communication layer.
func NewTCPTest() *LocalTest {
	t := NewLocalTest()
	t.mode = TCP
	return t
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
func (l *LocalTest) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	rootServerIdentityID := t.Root.ServerIdentity.ID
	for _, h := range l.Hosts {
		if h.ServerIdentity.ID.Equal(rootServerIdentityID) {
			// XXX do we really need multiples overlays ? Can't we just use the
			// Node, since it is already dispatched as like a TreeNode ?
			return l.Overlays[h.ServerIdentity.ID].CreateProtocolSDA(name, t)
		}
	}
	return nil, errors.New("Didn't find host for tree-root")
}

// GenHosts returns n Hosts with a localRouter
func (l *LocalTest) GenHosts(n int) []*Host {
	hosts := l.genLocalHosts(n)
	for _, host := range hosts {
		l.Hosts[host.ServerIdentity.ID] = host
		l.Overlays[host.ServerIdentity.ID] = host.overlay
		l.Services[host.ServerIdentity.ID] = host.serviceManager.services
	}
	return hosts

}

// GenTree will create a tree of n hosts with a localRouter, and returns the
// list of hosts and the associated roster / tree.
func (l *LocalTest) GenTree(n int, register bool) ([]*Host, *Roster, *Tree) {
	hosts := l.GenHosts(n)

	list := l.GenRosterFromHost(hosts...)
	tree := list.GenerateBinaryTree()
	l.Trees[tree.ID] = tree
	if register {
		hosts[0].overlay.RegisterRoster(list)
		hosts[0].overlay.RegisterTree(tree)
	}
	return hosts, list, tree

}

// GenBigTree will create a tree of n hosts.
// If register is true, the Roster and Tree will be registered with the overlay.
// 'nbrHosts' is how many hosts are created
// 'nbrTreeNodes' is how many TreeNodes are created
// nbrHosts can be smaller than nbrTreeNodes, in which case a given host will
// be used more than once in the tree.
func (l *LocalTest) GenBigTree(nbrTreeNodes, nbrHosts, bf int, register bool) ([]*Host, *Roster, *Tree) {
	hosts := l.GenHosts(nbrHosts)

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
		log.Lvl3("Closing host", host.ServerIdentity.Address)
		err := host.Close()
		if err != nil {
			log.Error("Closing host", host.ServerIdentity.Address,
				"gives error", err)
		}

		for host.Listening() {
			log.Print("Sleeping while waiting to close...")
			time.Sleep(10 * time.Millisecond)
		}
		delete(l.Hosts, host.ServerIdentity.ID)
	}
	for _, node := range l.Nodes {
		log.Lvl3("Closing node", node)
		node.Close()
	}
	l.Nodes = make([]*TreeNodeInstance, 0)
	// Give the nodes some time to correctly close down
	//time.Sleep(time.Millisecond * 500)
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
		MsgType:  network.TypeToPacketTypeID(msg),
		From:     from.token,
		To:       to.token,
	}
	return to.overlay.TransmitMsg(sdaMsg)
}

// AddPendingTreeMarshal takes a treeMarshal and adds it to the list of the
// known trees, also triggering dispatching of SDA-messages waiting for that
// tree
func (l *LocalTest) AddPendingTreeMarshal(h *Host, tm *TreeMarshal) {
	h.overlay.addPendingTreeMarshal(tm)
}

// CheckPendingTreeMarshal looks whether there are any treeMarshals to be
// called
func (l *LocalTest) CheckPendingTreeMarshal(h *Host, el *Roster) {
	h.overlay.checkPendingTreeMarshal(el)
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

// MakeHELS creates nbr hosts, and will return the associated roster. It also
// returns the Service object of the first hosts in the list having sid as a
// ServiceID.
func (l *LocalTest) MakeHELS(nbr int, sid ServiceID) ([]*Host, *Roster, Service) {
	hosts := l.GenHosts(nbr)
	el := l.GenRosterFromHost(hosts...)
	return hosts, el, l.Services[hosts[0].ServerIdentity.ID][sid]
}

// NewPrivIdentity returns a secret + ServerIdentity. The SI will have
// "localhost:+port as first address.
func NewPrivIdentity(port int) (abstract.Scalar, *network.ServerIdentity) {
	address := network.NewLocalAddress("127.0.0.1:" + strconv.Itoa(port))
	priv, pub := PrivPub()
	id := network.NewServerIdentity(pub, address)
	return priv, id
}

// NewTCPHost creates a new host with a tcpRouter with "localhost:"+port as an
// address.
func NewTCPHost(port int) *Host {
	priv, id := NewPrivIdentity(port)
	addr := network.NewTCPAddress(id.Address.NetworkAddress())
	tcpHost, err := network.NewTCPHost(addr)
	if err != nil {
		panic(err)
	}
	id.Address = tcpHost.Address()
	router := network.NewRouter(id, tcpHost)
	h := NewHostWithRouter(id, priv, router)
	go h.Start()
	for !h.Listening() {
		time.Sleep(10 * time.Millisecond)
	}
	return h
}

// NewLocalHost returns a new host using a LocalRouter (channels) to communicate.
// At the return of this function, the router is already Run()ing in a go
// routine.
func NewLocalHost(port int) *Host {
	priv, id := NewPrivIdentity(port)
	localRouter, err := network.NewLocalRouter(id)
	if err != nil {
		panic(err)
	}
	h := NewHostWithRouter(id, priv, localRouter)
	go h.Start()
	for !h.Listening() {
		time.Sleep(10 * time.Millisecond)
	}
	return h
}

// NewLocalHost returns a fresh Host using local connections within the context
// of this LocalTest
func (l *LocalTest) NewLocalHost(port int) *Host {
	priv, id := NewPrivIdentity(port)
	localRouter, err := network.NewLocalRouterWithManager(l.ctx, id)
	if err != nil {
		panic(err)
	}
	h := NewHostWithRouter(id, priv, localRouter)
	go h.Start()
	for !h.Listening() {
		time.Sleep(10 * time.Millisecond)
	}
	return h

}

// NewClient returns *Client for which the types depend on the mode of the
// LocalContext.
func (l *LocalTest) NewClient(serviceName string) *Client {
	switch l.mode {
	case TCP:
		return NewClient(serviceName)
	default:
		return l.NewLocalClient(serviceName)
	}
}

// NewLocalClient returns a new *Client using Local connections within the
// context of this LocalTest.
func (l *LocalTest) NewLocalClient(serviceName string) *Client {
	return &Client{
		ServiceID: ServiceFactory.ServiceID(serviceName),
		net:       network.NewLocalClientWithManager(l.ctx),
	}
}

// genLocalHosts returns n hosts created with a localRouter
func (l *LocalTest) genLocalHosts(n int) []*Host {
	hosts := make([]*Host, n)
	for i := 0; i < n; i++ {
		var host *Host
		port := 2000 + i*10
		switch l.mode {
		case TCP:
			host = NewTCPHost(0)
		default:
			host = l.NewLocalHost(port)
		}
		hosts[i] = host
	}

	for _, h := range hosts {
		for !h.Listening() {
			time.Sleep(40 * time.Millisecond)
		}
		l.Hosts[h.ServerIdentity.ID] = h
	}
	return hosts
}

// PrivPub creates a private/public key pair.
func PrivPub() (abstract.Scalar, abstract.Point) {
	keypair := config.NewKeyPair(network.Suite)
	return keypair.Secret, keypair.Public
}
