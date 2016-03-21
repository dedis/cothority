package randhound

import (
	"errors"

	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type RandHound struct {
	*sda.Node
	Leader   *Leader     // Pointer to the protocol leader
	Peer     *Peer       // Pointer to the 'current' peer
	NumPeers int         // Number of peers without the leader
	T        int         // Minimum number of shares needed to reconstruct the secret
	R        int         // Minimum number of signatures needed to certify a deal
	N        int         // Total number of trustees / shares (T <= R <= N)
	Purpose  string      // Purpose of the protocol instance
	Done     chan bool   // For signaling that a protocol run is finished (leader only)
	Result   chan []byte // For returning the generated randomness (leader only)
}

func NewRandHound(node *sda.Node) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		Node:     node,
		NumPeers: len(node.Tree().ListNodes()) - 1,
	}

	// Setup leader or peer depending on the node's location in the tree
	if node.IsRoot() {
		rh.Done = make(chan bool, 1)
		rh.Result = make(chan []byte)
		leader, err := rh.newLeader()
		if err != nil {
			return nil, err
		}
		rh.Leader = leader
	} else {
		peer, err := rh.newPeer()
		if err != nil {
			return nil, err
		}
		rh.Peer = peer
	}

	// Setup message handlers
	handlers := []interface{}{
		rh.handleI1, rh.handleR1,
		rh.handleI2, rh.handleR2,
		rh.handleI3, rh.handleR3,
		rh.handleI4, rh.handleR4,
	}
	for _, h := range handlers {
		err := rh.RegisterHandler(h)
		if err != nil {
			return nil, errors.New("Couldn't register handler: " + err.Error())
		}
	}

	return rh, nil
}

// Start initiates the RandHound protocol. The leader forms the message I1 and
// sends it to  its children.
func (rh *RandHound) Start() error {
	rh.Leader.i1 = I1{
		SID:     rh.Leader.SID,
		GID:     rh.Leader.GID,
		HRc:     rh.hash(rh.Leader.Rc),
		T:       rh.T,
		R:       rh.R,
		N:       rh.N,
		Purpose: rh.Purpose,
	}
	return rh.sendToChildren(&rh.Leader.i1)
}
