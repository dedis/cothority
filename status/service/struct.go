package status

import (
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// Request is what the Status service is expected to receive from clients.
type Request struct {
}

// Response is what the Status service will reply to clients.
type Response struct {
	Status         map[string]*onet.Status
	ServerIdentity *network.ServerIdentity
}
