package main_test

import (
	"testing"

	"gopkg.in/dedis/onet.v1/simul"
)

func TestSimulation(t *testing.T) {
	simul.Start("jvss.toml")
}
