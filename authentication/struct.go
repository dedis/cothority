package authentication

import (
	"github.com/dedis/cothority/authentication/darc"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(
		GetPolicy{}, GetPolicyReply{},
		UpdatePolicy{}, UpdatePolicyReply{},
		UpdatePolicyPIN{}, UpdatePolicyPINReply{},
	)
}

// PROTOSTART
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "AuthProto";

// ***
// These are the messages used in the API-calls
// ***

// GetPolicy can be called from a client to get the latest version of a given
// policy. The system will return the most exact match. Given policies of
// "", "Skipchain", "Skipchain.StoreSkipBlock", GetPolicy{"Skipchain"} or
// GetPolicy{"Skipchain.GetUpdateChain"} would return the second policy,
// while GetPolicy{"Status"} would return the first policy.
type GetPolicy struct {
	Policy string
}

// GetPolicyReply contains the latest known version of the darc in the
// authentication service. If the Rule has not been found, then the closest
// matching policy will be returned.
// The Policy field is the actual policy under which this darc is stored.
type GetPolicyReply struct {
	Policy string
	Latest *darc.Darc
}

// UpdatePolicy proposes a new darc for a given policy. If the proposed darc
// is new (version == 0), then the Signature must contain a valid signature on
// the id of the new darc by an owner of the latest version of the root-darc.
type UpdatePolicy struct {
	Policy    string
	NewDarc   *darc.Darc
	Signature *darc.Signature
}

// UpdatePolicyReply doesn't return anything and is empty.
type UpdatePolicyReply struct{}

// UpdatePolicyPIN takes a new policy and a PIN. If the PIN is empty, then the
// authentication service will print a PIN to the server logs, then the client
// must read this out, and resend the data using that PIN.
// A darc submitted with this method must be a new darc (version = 0), but it
// can overwrite an existing darc-policy!
type UpdatePolicyPIN struct {
	Policy  string
	NewDarc *darc.Darc
	PIN     string
}

// UpdatePolicyPINReply is an empty reply. In case of an error, it will
// be transmitted through the onet.Client service.
type UpdatePolicyPINReply struct{}
