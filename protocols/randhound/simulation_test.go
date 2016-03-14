package randhound_test

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/randhound"
)

func TestSimulation(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	sim, err := randhound.NewRHSimulation("hosts=10\nbf=2\n")
	if err != nil {
		t.Fatal("New failed:", err)
	}
	config, err := sim.Setup("", []string{"localhost"})
	if err != nil {
		t.Fatal("Config failed:", err)
	}
	err = sim.Run(config)
	if err != nil {
		t.Fatal("Run failed:", err)
	}
}
