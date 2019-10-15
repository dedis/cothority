package protocol

import (
	"go.dedis.ch/onet/v4"
)

// PromptDecrypt is sent from node to node prompting the receiver to perform
// their respective partial decryption of the last mix.
type PromptDecrypt struct{}

// MessagePromptDecrypt is a wrapper around PromptDecrypt.
type MessagePromptDecrypt struct {
	*onet.TreeNode
	PromptDecrypt
}

// TerminateDecrypt is sent by the leaf node to the root node upon completion of
// the last partial decryption, which terminates the protocol.
type TerminateDecrypt struct {
	Error string
}

// MessageTerminateDecrypt is a wrapper around TerminateDecrypt.
type MessageTerminateDecrypt struct {
	*onet.TreeNode
	TerminateDecrypt
}
