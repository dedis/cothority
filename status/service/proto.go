package status

import (
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

// PROTOSTART
// type :map\[string\]onet.Status:map<string, onet.Status>
// package status;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "StatusProto";
//
// import "onet.proto";
// import "network.proto";

// Request is what the Status service is expected to receive from clients.
type Request struct {
}

// Response is what the Status service will reply to clients.
type Response struct {
	Status         map[string]*onet.Status
	ServerIdentity *network.ServerIdentity
}

// CheckConnectivity is sent by a client to check the connectivity of a given
// roster. The Time must be within 2 minutes of the server's time. The signature
// must be a schnorr-signature using the private conode-key on the following
// message:
//   sha256( bytes.LittleEndian.PutUInt64(Time) |
//           binary.LittleEndian.PutUInt64(Timeout) |
//           FindFaulty ? byte(1) : byte(0) |
//           protobuf.Encode(List[0]) | protobuf.Encode(List[1])... )
type CheckConnectivity struct {
	Time       int64
	Timeout    int64
	FindFaulty bool
	List       []*network.ServerIdentity
	Signature  []byte
}

// CheckConnectivityReply is the minimum list of all nodes that can contact each
// other.
type CheckConnectivityReply struct {
	Nodes []*network.ServerIdentity
}
