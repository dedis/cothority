package protocol

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

func TestCosi(t *testing.T) {
	//defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	for _, nbrHosts := range []int{1, 3, 13} {
		log.Lvl2("Running cosi with", nbrHosts, "hosts")
		local := sda.NewLocalTest()
		hosts, el, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true)
		aggPublic := network.Suite.Point().Null()
		for _, e := range el.List {
			aggPublic = aggPublic.Add(aggPublic, e.Public)
		}

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello World Cosi")

		// Register the function generating the protocol instance
		var root *CoSi
		// function that will be called when protocol is finished by the root
		doneFunc := func(sig []byte) {
			suite := hosts[0].Suite()
			publics := el.Publics()
			if err := root.VerifyResponses(aggPublic); err != nil {
				t.Fatal("Error verifying responses", err)
			}
			if err := VerifySignature(suite, publics, msg, sig); err != nil {
				t.Fatal("Error verifying signature:", err)
			}
			done <- true
		}

		// Start the protocol
		p, err := local.CreateProtocol("CoSi", tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}
		root = p.(*CoSi)
		root.Message = msg
		responseFunc := func(in []abstract.Scalar) {
			log.Lvl1("Got response")
			if len(root.Children()) != len(in) {
				t.Fatal("Didn't get same number of responses")
			}
		}
		root.RegisterResponseHook(responseFunc)
		root.RegisterSignatureHook(doneFunc)
		go root.StartProtocol()
		select {
		case <-done:
		case <-time.After(time.Second * 2):
			t.Fatal("Could not get signature verification done in time")
		}
		local.CloseAll()
	}
}
