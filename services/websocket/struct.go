package websocket

import "github.com/dedis/cothority/network"

func init() {
	network.RegisterPacketType(SignRequest{})
	network.RegisterPacketType(SignReply{})
}

type Ping struct {
	Msg string
}

// SignRequest is interpreted as a request to sign the Hash using the
// nodes in NodeList.
type SignRequest struct {
	Hash []byte
	// list of "host1:port1;host2:port2" for all nodes that need to sign.
	// different hosts are separated by a ";"
	NodeList string
}

// SignReply returns the collective signature, together with the aggregate
// key.
type SignReply struct {
	Signature []byte
	Aggregate []byte
}
