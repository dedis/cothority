package network

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
