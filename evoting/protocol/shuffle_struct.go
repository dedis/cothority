package protocol

import (
	"github.com/dedis/onet"
)

// PromptShuffle is sent from node to node prompting the receiver to perform
// their respective shuffle (re-encryption) of the ballots.
type PromptShuffle struct{}

// MessagePrompt is a wrapper around PromptShuffle
type MessagePrompt struct {
	*onet.TreeNode
	PromptShuffle
}

// TerminateShuffle is sent by the leaf node to the root node upon completion of
// the last shuffle, which terminates the protocol.
type TerminateShuffle struct{}

// MessageTerminate is a wrapper around TerminateShuffle.
type MessageTerminate struct {
	*onet.TreeNode
	TerminateShuffle
}
