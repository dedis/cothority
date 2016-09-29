package randhound_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/randhound"
	"github.com/dedis/cothority/sda"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestRandHound(t *testing.T) {

	// Setup parameters
	var name string = "RandHound"             // Protocol name
	var nodes uint32 = 10                     // Number of nodes (peers + leader)
	var trustees uint32 = 5                   // Number of trustees
	var purpose string = "RandHound test run" // Purpose
	var shards uint32 = 2                     // Number of shards created from the randomness

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false)
	defer local.CloseAll()

	// Setup and Start RandHound
	log.Lvlf1("RandHound - starting")
	leader, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise RandHound protocol:", err)
	}
	rh := leader.(*randhound.RandHound)
	err = rh.Setup(nodes, trustees, purpose)
	if err != nil {
		t.Fatal("Couldn't initialise RandHound protocol:", err)
	}
	log.Lvlf1("RandHound - group config: %d %d %d %d %d %d\n", rh.Group.N, rh.Group.F, rh.Group.L, rh.Group.K, rh.Group.R, rh.Group.T)
	log.Lvlf1("RandHound - shards: %d\n", shards)
	leader.Start()

	select {
	case <-rh.Leader.Done:
		log.Lvlf1("RandHound - done")
		rnd, err := rh.Random()
		if err != nil {
			t.Fatal(err)
		}
		sharding, err := rh.Shard(rnd, shards)
		if err != nil {
			t.Fatal(err)
		}
		log.Lvlf1("RandHound - random bytes: %v\n", rnd)
		log.Lvlf1("RandHound - sharding: %v\n", sharding)
	case <-time.After(time.Second * 60):
		t.Fatal("RandHound – time out")
	}
}
