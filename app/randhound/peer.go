package randhound

import "github.com/dedis/crypto/random"

// TODO: figure out which variables from the old RandHound server (see
// app/rand/srv.go) are necessary and which ones are covered by SDA
type Peer struct {
	keysize  int    // Key size in bytes
	hashsize int    // Hash size in bytes
	Rs       []byte //Peer's  trustee-selection random value
	i1       I1     // I1 message we received from the leader
	i2       I2     // I2 message we received from the leader
	i3       I3     // I3 message we received from the leader
	i4       I4     // I4 message we received from the leader
	r1       R1     // R1 message we sent to the leader
	r2       R2     // R2 message we sent to the leader
	r3       R3     // R3 message we sent to the leader
	r4       R4     // R4 message we sent to the leader
}

func (p *ProtocolRandHound) newPeer() *Peer {

	keysize := 16
	hashsize := keysize * 2

	// Choose peer's trustee-selsection randomness
	rs := make([]byte, hashsize)
	random.Stream.XORKeyStream(rs, rs)

	return &Peer{
		keysize:  keysize,
		hashsize: hashsize,
		Rs:       rs,
	}
}
