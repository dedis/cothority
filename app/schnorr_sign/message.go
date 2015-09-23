package schnorr_sign

import (
	"github.com/dedis/crypto/abstract"
)

// message includes all messages type being sent over the network for the schnorr sign topolgy

// message to send at the end of a "round" or a "setup" to tell wether it's ok or not
type Ack struct {
	Id    int
	Valid bool // flag to tell wether the remote peer is OK or NOT
}

// message to send at the beginning of a connection to tell the remote peer
// our ID and our public key (Basic knowledge)
type Syn struct {
	Id     int
	Public abstract.Point
}
