package randhound

import "github.com/dedis/crypto/poly"

type Leader struct {
	Rc      []byte                 // Leader's trustee-selection random value
	Rs      [][]byte               // Peers' trustee-selection random values
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
	Result  chan []byte            // For returning the generated randomness
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
		Result:  make(chan []byte),
	}, nil
}
