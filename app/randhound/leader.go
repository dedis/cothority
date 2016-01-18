package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/dedis/crypto/random"
)

// TODO: figure out which variables from the old RandHound client (see
// app/rand/cli.go) are necessary and which ones are covered by SDA
type Leader struct {
	keysize  int      // Key size in bytes
	hashsize int      // Hash size in bytes
	Session  *Session // Session parameter block
	SID      []byte   // Session fingerprint
	Group    *Group   // Group parameter block
	GID      []byte   // Group fingerprint
	Rc       []byte   // Client's trustee-selection random value
	Rs       [][]byte // Server's trustee-selection random values
	i1       I1       // I1 message
	i2       I2       // I2 message
	i3       I3       // I3 message
	i4       I4       // I4 message
	r1       []R1     // Decoded R1 messages
	r2       []R2     // Decoded R2 messages
	r3       []R3     // Decoded R3 messages
	r4       []R4     // Decoded R4 messages

	//deals  []poly.Promise   // Unmarshaled deals from servers
	//shares []poly.PriShares // Revealed shares

	//t Transcript // Third-party verifiable message transcript
}

func (p *ProtocolRandHound) newLeader() *Leader {

	keysize := 16
	hashsize := keysize * 2

	// Choose client's trustee-selection randomness
	rc := make([]byte, hashsize)
	random.Stream.XORKeyStream(rc, rc)

	// Setup session
	x, _ := p.ProtocolStruct.Host.Entity.Public.MarshalBinary()
	session := &Session{LPubKey: x, Purpose: "RandHound test run", Time: time.Now()} // TODO: make channel for purpose
	y := []byte(session.Purpose)
	z, _ := session.Time.MarshalBinary()
	sid := p.Hash(x, y, z)

	// Setup group (TODO: marshal and include public keys of the peers, error checking)
	group := &Group{make([][]byte, 0), 1, 2, 3, 4}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, group.F)
	binary.Write(buf, binary.LittleEndian, group.L)
	binary.Write(buf, binary.LittleEndian, group.K)
	binary.Write(buf, binary.LittleEndian, group.T)
	gid := p.Hash(buf.Bytes())

	return &Leader{
		keysize:  keysize,
		hashsize: hashsize,
		Session:  session,
		SID:      sid,
		Group:    group,
		GID:      gid,
		Rc:       rc,
	}
}
