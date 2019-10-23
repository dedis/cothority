package cosi

import (
	"testing"
	"time"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

var tSuite = cothority.Suite

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestCosi(t *testing.T) {
	for _, nbrHosts := range []int{1, 3, 13} {
		log.Lvl2("Running cosi with", nbrHosts, "hosts")
		local := onet.NewLocalTest(tSuite)
		hosts, el, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true)
		aggPublic := tSuite.Point().Null()
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
		responseFunc := func(in []kyber.Scalar) {
			log.Lvl1("Got response")
			if len(root.Children()) != len(in) {
				t.Fatal("Didn't get same number of responses")
			}
		}
		root.RegisterResponseHook(responseFunc)
		root.RegisterSignatureHook(doneFunc)
		go root.Start()
		select {
		case <-done:
		case <-time.After(time.Second * 2):
			t.Fatal("Could not get signature verification done in time")
		}
		local.CloseAll()
	}
}
