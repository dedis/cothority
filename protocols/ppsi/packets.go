package ppsi

import (
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

type Init struct {
}

type ElgEncryptedMessage struct {
	Content   []map[int]abstract.Point
	Users     map[int]int
	NumPhones int
	Sets      int
	ID        int
}

type FullyPhEncryptedMessage struct {
	Content []abstract.Point
	Users   map[int]int
	Mode    int
	Sets    int
	ID      int
}

type PartiallyPhDecryptedMessage struct {
	Content []abstract.Point
	Users   map[int]int
	Mode    int
	Sets    int
	ID      int
}

type PlainMessage struct {
	Content []string
	Users   map[int]int
	Mode    int
	ID      int
}

type DoneMessage struct {
	Src  int
	Sets int
}

type chanElgEncryptedMessage struct {
	*sda.TreeNode
	ElgEncryptedMessage
}

type chanFullyPhEncryptedMessage struct {
	*sda.TreeNode
	FullyPhEncryptedMessage
}
type chanPartiallyPhDecryptedMessage struct {
	*sda.TreeNode
	PartiallyPhDecryptedMessage
}

type chanPlainMessage struct {
	*sda.TreeNode
	PlainMessage
}
type chanDoneMessage struct {
	*sda.TreeNode
	DoneMessage
}

type chanInitiateRequest struct {
	*sda.TreeNode
	Init
}
