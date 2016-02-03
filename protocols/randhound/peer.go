package randhound

import "github.com/dedis/crypto/random"

type Peer struct {
	self   int       // Peer's index in the entity list
	Rs     []byte    // Peer's trustee-selection random value
	shares []R4Share // Peer's shares
	i1     I1        // I1 message we received from the leader
	i2     I2        // I2 - " -
	i3     I3        // I3 - " -
	i4     I4        // I4 - " -
	r1     R1        // R1 message we sent to the leader
	r2     R2        // R2 - " -
	r3     R3        // R3 - " -
	r4     R4        // R4 - " -
}

func (rh *RandHound) newPeer() (*Peer, error) {

	// Choose peer's trustee-selsection randomness
	hs := rh.Node.Suite().Hash().Size()
	rs := make([]byte, hs)
	random.Stream.XORKeyStream(rs, rs)

	return &Peer{
		self: rh.PID[rh.Node.TreeNode().Id],
		Rs:   rs,
	}, nil
}
