package main_test

import (
	"testing"

	"github.com/dedis/onet/simul"
)

func TestSimulation(t *testing.T) {
	simul.Start("cosi.toml", "cosi_verification.toml")
}
