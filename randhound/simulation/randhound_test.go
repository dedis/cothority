package main_test

import (
	"testing"

	"github.com/dedis/onet/simul"
)

func TestSimulation(t *testing.T) {
	simul.Start("randhound.toml")
}
