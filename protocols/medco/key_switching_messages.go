package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

type KeySwitchedCipherMessage struct {
	VisitorMessage
	Vect CipherVector
	NewKey abstract.Point
	OriginalEphemeralKeys []abstract.Point
}

type KeySwitchedCipherStruct struct {
	*sda.TreeNode
	KeySwitchedCipherMessage
}