package protocol

import (
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// DefaultKDProtocolName is the name of the default distibution protocol for
// pairing keys.
const DefaultKDProtocolName = "blsftKeyDistDefault"

func init() {
	network.RegisterMessages(&Request{}, &Reply{}, &Distribute{})
}

// Request asks all the nodes to send their public keys. It is sent to all
// nodes from the root-node.
type Request struct{}

type structRequest struct {
	*onet.TreeNode
	Request
}

type Reply struct {
	Public []byte
}

type structReply struct {
	*onet.TreeNode
	Reply
}

type Distribute struct {
	Publics [][]byte
}

type structDistribute struct {
	*onet.TreeNode
	Distribute
}
