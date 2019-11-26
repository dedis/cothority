package main

import (
	"testing"

	"go.dedis.ch/onet/v3/simul"
)

func TestSimulation(t *testing.T) {
	raiseLimit()
	simul.Start("local.toml")
}
