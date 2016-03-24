package randhound

import (
	"bytes"
	"encoding/binary"
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
		err := rh.RegisterHandler(h)
		if err != nil {
			return nil, errors.New("Couldn't register handler: " + err.Error())
		}
	}

	return rh, nil
}

// Setup stores basic parameters of the RandHound protocol. Needs to be called
// before Start
func (rh *RandHound) Setup(nodes int, trustees int, purpose string) error {

	// Setup group
	group, gid, err := rh.newGroup(nodes, trustees)
	if err != nil {
		return err
	}
	rh.GID = gid
	rh.Group = group

	// Setup session
	session, sid, err := rh.newSession(purpose)
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

	// Choose trustee-selection randomness
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

func (rh *RandHound) newGroup(nodes int, trustees int) (*Group, []byte, error) {

	n := nodes    // Number of nodes (peers + leader)
	k := trustees // Number of trustees (= shares)
	buf := new(bytes.Buffer)

	// Setup group parameters: note that T <= R <= K must hold;
	// T = R for simplicity, might change later
	gp := [6]int{
		n,           // N: total number of nodes (peers + leader)
		n / 3,       // F: maximum number of Byzantine nodes tolerated
		n - (n / 3), // L: minimum number of non-Byzantine nodes required
		k,           // K: total number of trustees (= shares)
		(k + 1) / 2, // R: minimum number of signatures needed to certify a deal
		(k + 1) / 2, // T: minimum number of shares needed to reconstruct a secret
	}

	// Include public keys of all nodes into group ID
	for _, x := range rh.Tree().ListNodes() {
		pub, err := x.Entity.Public.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		err = binary.Write(buf, binary.LittleEndian, pub)
		if err != nil {
			return nil, nil, err
		}
	}

	// Process group parameters
	for _, g := range gp {
		err := binary.Write(buf, binary.LittleEndian, uint32(g))
		if err != nil {
			return nil, nil, err
		}
	}

	return &Group{
		N: gp[0],
		F: gp[1],
		L: gp[2],
		K: gp[3],
		R: gp[4],
		T: gp[5]}, rh.hash(buf.Bytes()), nil
}

func (rh *RandHound) newSession(purpose string) (*Session, []byte, error) {

	pub, err := rh.Node.Entity().Public.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	t := time.Now()
	tm, err := t.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return &Session{
		LPubKey: pub,
		Purpose: purpose,
		Time:    t}, rh.hash(pub, []byte(purpose), tm), nil
}
