package ntree_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/ntree"
	"github.com/dedis/cothority/sda"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestNtree(t *testing.T) {
	for _, nbrHosts := range []int{1, 3, 13} {
		log.Lvl2("Running ntree with", nbrHosts, "hosts")
		local := sda.NewLocalTest()

		_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true)

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Ntree rocks slowly")

		// Register the function generating the protocol instance
		var root *ntree.Protocol
		// function that will be called when protocol is finished by the root
		doneFunc := func() bool {
			done <- true
			return true
		}

		// Start the protocol
		pi, err := local.CreateProtocol("NaiveTree", tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}
		root = pi.(*ntree.Protocol)
		root.Message = msg
		root.OnDoneCallback(doneFunc)
		err = pi.Start()
		if nbrHosts == 1 {
			if err == nil {
				t.Fatal("Shouldn't be able to start NTree with 1 node")
			}
		} else if err != nil {
			t.Fatal("Couldn't start protocol:", err)
		} else {
			select {
			case <-done:
			case <-time.After(time.Second * 2):
				t.Fatal("Protocol didn't finish in time")
			}
		}
		local.CloseAll()
	}
}
