package network

import "github.com/dedis/crypto/abstract"

// This file contains usual packets that are needed for different
// protocols.

const (
	ErrorType Type = iota
)

type Error struct {
	Msg string
}

func init() {
	RegisterProtocolType(ErrorType, Error{})
}
