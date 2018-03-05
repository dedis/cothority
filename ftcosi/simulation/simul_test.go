package main

import (
	"testing"

	"gopkg.in/dedis/onet.v2/simul"
)

func TestSimulation(t *testing.T) {
	raiseLimit()
	simul.Start("local.toml")
}
