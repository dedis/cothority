package randhound

import "github.com/dedis/crypto/random"

// TODO: figure out which variables from the old RandHound server (see
// app/rand/srv.go) are necessary and which ones are covered by SDA
type Peer struct {
	keysize  int
	hashsize int
	Rs       []byte
	i1       I1
	i2       I2
	i3       I3
	i4       I4
	r1       R1
	r2       R2
	r3       R3
	r4       R4
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
