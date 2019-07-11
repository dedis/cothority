package protocol

import (
	"time"
)

// Parameters holds a set of parameters, mainly for simulation purposes
type Parameters struct {
	GossipTick    time.Duration // periodic interval
	RumorPeers    int           // number of peers that a rumor message is sent to
	ShutdownPeers int           // number of peers that the shutdown message is sent to
	TreeMode      bool          // aggregate messages wherever possible
	Threshold     int
}

// DefaultParams returns a set of default parameters
func DefaultParams(n int) Parameters {
	t := DefaultThreshold(n)

	if n == 1 {
		n++
	}

	return Parameters{
		GossipTick:    100 * time.Millisecond,
		RumorPeers:    n - 1,
		ShutdownPeers: n - 1,
		TreeMode:      true,
		Threshold:     t,
	}
}
