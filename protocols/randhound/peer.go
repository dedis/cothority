package randhound

type Peer struct {
	Rs     []byte           // Peer's trustee-selection random value
	shares map[int]*R4Share // Peer's shares
	i1     *I1              // I1 message we received from the leader
	i2     *I2              // I2 - " -
	i3     *I3              // I3 - " -
	i4     *I4              // I4 - " -
	r1     *R1              // R1 message we sent to the leader
	r2     *R2              // R2 - " -
	r3     *R3              // R3 - " -
	r4     *R4              // R4 - " -
}

func (rh *RandHound) newPeer() (*Peer, error) {
	return &Peer{shares: make(map[int]*R4Share)}, nil
}
