package status

import (
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// Request is what the Status service is expected to receive from clients.
type Request struct {
}

// Response is what the Status service will reply to clients.
type Response struct {
	Status         map[string]*onet.Status
	ServerIdentity *network.ServerIdentity
}
