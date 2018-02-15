package status

import (
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// PROTOSTART
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "StatusProto";

// ***
// These are the messages used in the API-calls
// ***

// Request is what the Status service is expected to receive from clients.
type Request struct {
}

// Response is what the Status service will reply to clients.
type Response struct {
	Status         map[string]onet.Status
	ServerIdentity *network.ServerIdentity
}
