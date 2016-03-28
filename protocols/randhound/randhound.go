package randhound

import (
	"errors"
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type RandHound struct {
	*sda.Node
	GID     []byte   // Group ID
	Group   *Group   // Group parameters
	SID     []byte   // Session ID
	Session *Session // Session parameters
	Leader  *Leader  // Protocol leader
	Peer    *Peer    // Current peer
}

func NewRandHound(node *sda.Node) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		Node: node,
	}

	// Setup leader or peer depending on the node's location in the tree
	if node.IsRoot() {
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
		if err := rh.RegisterHandler(h); err != nil {
			return nil, errors.New("Couldn't register handler: " + err.Error())
		}
	}

	return rh, nil
}

// Setup creates the group and session parameters of the RandHound protocol.
// Needs to be called before Start.
func (rh *RandHound) Setup(nodes int, trustees int, purpose string) error {

	// Setup group
	group, gid, err := rh.newGroup(nodes, trustees)
	if err != nil {
		return err
	}
	rh.GID = gid
	rh.Group = group

	// Setup session
	session, sid, err := rh.newSession(rh.Node.Entity().Public, purpose, time.Now())
	if err != nil {
		return err
	}
	rh.SID = sid
	rh.Session = session

	return nil
}

// Start initiates the RandHound protocol. The leader chooses its
// trustee-selection randomness, forms the message I1 and sends it to its
// children.
func (rh *RandHound) Start() error {

	hs := rh.Node.Suite().Hash().Size()
	rc := make([]byte, hs)
	random.Stream.XORKeyStream(rc, rc)
	rh.Leader.Rc = rc

	rh.Leader.i1 = I1{
		SID:     rh.SID,
		Session: rh.Session,
		GID:     rh.GID,
		Group:   rh.Group,
		HRc:     rh.hash(rh.Leader.Rc),
	}

	return rh.sendToChildren(&rh.Leader.i1)
}
