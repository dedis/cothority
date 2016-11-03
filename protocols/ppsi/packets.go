package ppsi


import (
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)


type SetsRequest struct {
	SetsIds []int
	
}

type ElgEncryptedMessage struct {
	Content []map[int]abstract.Point
	users map[int]int
	mode int
}


type FullyPhEncryptedMessage struct {
	Content []abstract.Point
	users map[int]int
	mode int
}


type PartiallyPhDecryptedMessage struct {
	Content []abstract.Point
	users map[int]int
	mode int
}

type PlainMessage struct {
	Content []string 
	users map[int]int
	mode int
}

type DoneMessage struct {
	mode int
}

type chanSetsRequest struct {
	*sda.TreeNode
	SetsRequest
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

