package template

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	for _, msg := range []interface{}{
		CountRequest{}, CountResponse{},
		ClockRequest{}, ClockResponse{},
	} {
		network.RegisterPacketType(msg)
	}
}

// ClockRequest will run the tepmlate-protocol on the roster and return
// the time spent doing so.
type ClockRequest struct {
	Roster *sda.Roster
}

// ClockResponse returns the time spent for the protocol-run.
type ClockResponse struct {
	Time float64
}

// CountRequest will return how many times the protocol has been run.
type CountRequest struct {
}

// CountResponse returns the number of protocol-runs
type CountResponse struct {
	Count int
}
