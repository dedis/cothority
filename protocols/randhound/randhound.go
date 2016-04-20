// Package randhound is a client/server protocol that allows a list of nodes to produce
// a public random string in an unbiasable and verifiable way given that a
// threshold of nodes is honest. The protocol is driven by a leader (= client)
// which scavenges the public randomness from its peers (= servers) over the
// course of four round-trips (= phases).
package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

// RandHound is the main protocol struct and implements the
// sda.ProtocolInstance interface.
type RandHound struct {
	*sda.TreeNodeInstance
	GID     []byte   // Group ID
	Group   *Group   // Group parameters
	SID     []byte   // Session ID
	Session *Session // Session parameters
	Leader  *Leader  // Protocol leader
	Peer    *Peer    // Current peer
}

// Session encapsulates some metadata of a RandHound protocol run.
type Session struct {
	Fingerprint []byte    // Fingerprint of a public key (usually of the leader)
	Purpose     string    // Purpose of randomness
	Time        time.Time // Scheduled initiation time
}

// Group encapsulates all the configuration parameters of a list of RandHound nodes.
type Group struct {
	N uint32 // Total number of nodes (peers + leader)
	F uint32 // Maximum number of Byzantine nodes tolerated (1/3)
	L uint32 // Minimum number of non-Byzantine nodes required (2/3)
	K uint32 // Total number of trustees (= shares generated per peer)
	R uint32 // Minimum number of signatures needed to certify a deal
	T uint32 // Minimum number of shares needed to reconstruct a secret
}

// Leader (=client) refers to the node which initiates the RandHound protocol,
// moves it forward, and ultimately outputs the generated public randomness.
type Leader struct {
	rc      []byte                 // Leader's trustee-selection random value
	rs      [][]byte               // Peers' trustee-selection random values
	i1      *I1                    // I1 message sent to the peers
	i2      *I2                    // I2 - " -
	i3      *I3                    // I3 - " -
	i4      *I4                    // I4 - " -
	r1      map[uint32]*R1         // R1 messages received from the peers
	r2      map[uint32]*R2         // R2 - " -
	r3      map[uint32]*R3         // R3 - " -
	r4      map[uint32]*R4         // R4 - " -
	states  map[uint32]*poly.State // States for deals and responses from peers
	invalid map[uint32]*[]uint32   // Map to mark invalid shares
	Done    chan bool              // For signaling that a protocol run is finished
}

// Peer (=server) refers to a node which contributes to the generation of the
// public randomness.
type Peer struct {
	rs     []byte              // A peer's trustee-selection random value
	shares map[uint32]*R4Share // A peer's shares
	i1     *I1                 // I1 message we received from the leader
	i2     *I2                 // I2 - " -
	i3     *I3                 // I3 - " -
	i4     *I4                 // I4 - " -
	r1     *R1                 // R1 message we sent to the leader
	r2     *R2                 // R2 - " -
	r3     *R3                 // R3 - " -
	r4     *R4                 // R4 - " -
}

// NewRandHound generates a new RandHound instance.
func NewRandHound(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		TreeNodeInstance: node,
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
	h := []interface{}{
		rh.handleI1, rh.handleR1,
		rh.handleI2, rh.handleR2,
		rh.handleI3, rh.handleR3,
		rh.handleI4, rh.handleR4,
	}
	err := rh.RegisterHandlers(h...)

	return rh, err
}

// Setup configures a RandHound instance by creating group and session
// parameters of the protocol. Needs to be called before Start.
func (rh *RandHound) Setup(nodes uint32, trustees uint32, purpose string) error {

	// Setup group
	group, gid, err := rh.newGroup(nodes, trustees)
	if err != nil {
		return err
	}
	rh.GID = gid
	rh.Group = group

	// Setup session
	session, sid, err := rh.newSession(rh.Entity().Public, purpose, time.Now())
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

	hs := rh.Suite().Hash().Size()
	rc := make([]byte, hs)
	random.Stream.XORKeyStream(rc, rc)
	rh.Leader.rc = rc

	rh.Leader.i1 = &I1{
		SID:     rh.SID,
		Session: rh.Session,
		GID:     rh.GID,
		Group:   rh.Group,
		HRc:     rh.hash(rh.Leader.rc),
	}

	return rh.SendToChildren(rh.Leader.i1)
}

func (rh *RandHound) newSession(public abstract.Point, purpose string, time time.Time) (*Session, []byte, error) {

	buf := new(bytes.Buffer)

	pub, err := public.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	if err = binary.Write(buf, binary.LittleEndian, pub); err != nil {
		return nil, nil, err
	}
	tm, err := time.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	if err = binary.Write(buf, binary.LittleEndian, tm); err != nil {
		return nil, nil, err
	}
	if err = binary.Write(buf, binary.LittleEndian, []byte(purpose)); err != nil {
		return nil, nil, err
	}

	return &Session{
		Fingerprint: pub,
		Purpose:     purpose,
		Time:        time}, rh.hash(buf.Bytes()), nil
}

func (rh *RandHound) newGroup(nodes uint32, trustees uint32) (*Group, []byte, error) {

	buf := new(bytes.Buffer)

	n := nodes    // Number of nodes (peers + leader)
	k := trustees // Number of trustees (= shares generaetd per peer)

	// Setup group parameters: note that T <= R <= K must hold;
	// T = R for simplicity, might change later
	gp := [6]uint32{
		n,           // N: total number of nodes (peers + leader)
		n / 3,       // F: maximum number of Byzantine nodes tolerated
		n - (n / 3), // L: minimum number of non-Byzantine nodes required
		k,           // K: total number of trustees (= shares generated per peer)
		(k + 1) / 2, // R: minimum number of signatures needed to certify a deal
		(k + 1) / 2, // T: minimum number of shares needed to reconstruct a secret
	}

	// Include public keys of all nodes into group ID
	for _, x := range rh.List() {
		pub, err := x.Entity.Public.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		if err = binary.Write(buf, binary.LittleEndian, pub); err != nil {
			return nil, nil, err
		}
	}

	// Include group parameters into group ID
	for _, g := range gp {
		if err := binary.Write(buf, binary.LittleEndian, g); err != nil {
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

func (rh *RandHound) newLeader() (*Leader, error) {
	return &Leader{
		r1:      make(map[uint32]*R1),
		r2:      make(map[uint32]*R2),
		r3:      make(map[uint32]*R3),
		r4:      make(map[uint32]*R4),
		states:  make(map[uint32]*poly.State),
		invalid: make(map[uint32]*[]uint32),
		Done:    make(chan bool, 1),
	}, nil
}

func (rh *RandHound) newPeer() (*Peer, error) {
	return &Peer{shares: make(map[uint32]*R4Share)}, nil
}
