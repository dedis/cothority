package randhound

// Peer refers to a node which contributes to the generation of the public
// randomness.
type Peer struct {
	Rs     []byte              // A peer's trustee-selection random value
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

func (rh *RandHound) newPeer() (*Peer, error) {
	return &Peer{shares: make(map[uint32]*R4Share)}, nil
}
