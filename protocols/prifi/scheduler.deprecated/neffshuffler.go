package scheduler

import (
	"github.com/dedis/crypto/abstract"
)

type neffShuffler struct {
	suite abstract.Suite
}

// Simple DC-net encoder providing no disruption or equivocation protection,
// for experimentation and baseline performance evaluations.
func NeffShufflerFactory() CellCoder {
	return new(simpleCoder)
}

///// Client methods /////

func (c *simpleCoder) ClientCellSize(payloadlen int) int {
	return payloadlen // no expansion
}
