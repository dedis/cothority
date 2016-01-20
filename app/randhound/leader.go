package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

// TODO: figure out which variables from the old RandHound client (see
// app/rand/cli.go) are necessary and which ones are covered by SDA
type Leader struct {
	Session *Session    // Session parameter block
	SID     []byte      // Session fingerprint
	Group   *Group      // Group parameter block
	GID     []byte      // Group fingerprint
	Rc      []byte      // Leader's trustee-selection random value
	Rs      [][]byte    // Peers' trustee-selection random values
	i1      I1          // I1 message
	i2      I2          // I2 message
	i3      I3          // I3 message
	i4      I4          // I4 message
	r1      []R1        // Decoded R1 messages
	r2      []R2        // Decoded R2 messages
	r3      []R3        // Decoded R3 messages
	r4      []R4        // Decoded R4 messages
	deals   []poly.Deal // Unmarshaled deals from peers
	//shares []poly.PriShares // Revealed shares

	//t Transcript // Third-party verifiable message transcript
}

func (p *ProtocolRandHound) newLeader() (*Leader, error) {

	// Choose client's trustee-selection randomness
	hs := p.ProtocolStruct.Host.Suite().Hash().Size()
	rc := make([]byte, hs)
	random.Stream.XORKeyStream(rc, rc)

	// Setup session
	purpose := <-Purpose
	session, sid, err := p.newSession(purpose)
	if err != nil {
		return nil, err
	}

	// Setup group
	group, gid, err := p.newGroup()
	if err != nil {
		return nil, err
	}

	return &Leader{
		Session: session,
		SID:     sid,
		Group:   group,
		GID:     gid,
		Rc:      rc,
	}, nil
}

func (p *ProtocolRandHound) newSession(purpose string) (*Session, []byte, error) {

	pub, err := p.ProtocolStruct.Host.Entity.Public.MarshalBinary()
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
		Time:    t}, p.Hash(pub, []byte(purpose), tm), nil
}

func (p *ProtocolRandHound) newGroup() (*Group, []byte, error) {

	npeers := len(p.Children)
	ntrustees := <-Trustees
	buf := new(bytes.Buffer)
	ppub := make([][]byte, npeers)
	gp := [4]uint32{
		uint32(npeers / 3),
		uint32(npeers - (npeers / 3)),
		uint32(ntrustees),
		uint32((ntrustees + 1) / 2),
	} // Group parameters: F, L, K, T

	// Include public keys of all peers
	for i, c := range p.Children {
		pub, err := c.Entity.Public.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		err = binary.Write(buf, binary.LittleEndian, pub)
		if err != nil {
			return nil, nil, err
		}
		ppub[i] = pub
	}

	// Process group parameters
	for _, g := range gp {
		err := binary.Write(buf, binary.LittleEndian, g)
		if err != nil {
			return nil, nil, err
		}
	}

	return &Group{
		PPubKey: ppub,
		F:       gp[0],
		L:       gp[1],
		K:       gp[2],
		T:       gp[3]}, p.Hash(buf.Bytes()), nil
}
