package jvss_old

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
)

func init() {

}

// Longterm is what every nodes send to setup the longterm distributed key
// XXX moment it is needed because Deal needs first to call its UnmarshalInit(...)
// method before calling UnmarshalBinary(...). This is not possible for the
// moment with the network library, and poly.Deal should not need for that
// unmarshalInit first (network library can handle abstract.Point / Secret now).
type Longterm struct {
	// The index in the matrix where this deal is
	Index int
	// Bytes
	Bytes []byte
}

// Random is what is sent for each signing request : we need to generate a
// random distributed secret. same as longterm but clearly defined the purpose.
type Random struct {
	// request number
	RequestNo int
	Longterm
}

var LongtermType = network.RegisterMessageType(Longterm{})
var RandomType = network.RegisterMessageType(Random{})

//  SignatureRequest is used when we want to sign something
// This is to be sent by the root to everyone else
type SignatureRequest struct {
	Msg       []byte
	RequestNo int
}

var SignatureRequestType = network.RegisterMessageType(SignatureRequest{})

// SignatureResponse is used when peers respond to a signature request.
// It contains the partial signature that must be aggregated by the requestor to
// sign a message.
type SignatureResponse struct {
	RequestNo int
	Partial   *poly.SchnorrPartialSig
}

var SignatureResponseType = network.RegisterMessageType(SignatureResponse{})

// Unmarshal the poly.Deal inside the bytes
func (dl *Longterm) Deal(suite abstract.Suite, info poly.Threshold) *poly.Deal {
	d := new(poly.Deal).UnmarshalInit(info.T, info.R, info.N, suite)
	if err := d.UnmarshalBinary(dl.Bytes); err != nil {
		dbg.Error("Could not unmarshal Deal")
		return nil
	}
	return d
}

func NewLongtermFromDeal(index int, d *poly.Deal) Longterm {
	buf, _ := d.MarshalBinary()
	return Longterm{
		Bytes: buf,
		Index: index,
	}
}

type LongtermChan struct {
	*sda.TreeNode
	Longterm
}
type RandomChan struct {
	*sda.TreeNode
	Random
}
type RequestChan struct {
	*sda.TreeNode
	SignatureRequest
}
type ResponseChan struct {
	*sda.TreeNode
	SignatureResponse
}
