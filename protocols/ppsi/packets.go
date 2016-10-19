package protocol

import (
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)


type SetsRequest struct {
	SetsIds []int
	
}

type ElgEncryptedMessage struct {
	Content []abstract.Scalar  //not sure about the by value/by reference with the []abstract.Scalar
	users map[int]int
	mode int
}


type FullyPhEncryptedMessage struct {
	Content []abstract.Scalar
	users map[int]int
	mode int
}


type PartiallyPhDecryptedMessage struct {
	Content []abstract.Scalar
	users map[int]int
	mode int
}

type PlainMessage struct {
	Content []String  //do we want the plain phone numbers as Strings?
	users map[int]int
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

