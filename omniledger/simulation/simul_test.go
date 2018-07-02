package main_test

import (
	"testing"

	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestSimulation(t *testing.T) {
	simul.Start("service.toml")
}
