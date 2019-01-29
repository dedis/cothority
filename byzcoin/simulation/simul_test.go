package main_test

import (
	"testing"

	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestSimulation(t *testing.T) {
	simul.Start("coins.toml")
}
