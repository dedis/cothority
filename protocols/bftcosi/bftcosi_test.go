package bftcosi

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// TODO Dummy verification function:
// always returns OK/true/no-error on data

func TestBftCoSi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)

	for _, nbrHosts := range []int{1, 3, 13} {
		dbg.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
		local := sda.NewLocalTest()

		_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello BFTCoSi")

		// Register the function generating the protocol instance
		var root *ProtocolBFTCoSi
		// function that will be called when protocol is finished by the root
		doneFunc := func(chal abstract.Secret, resp abstract.Secret) {
			done <- true
		}

		RegisterVerification("DummyBFTCosi", func() {

		})

		// Start the protocol
		node, err := local.CreateNewNodeName("BFTCoSi", tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}
		root = node.ProtocolInstance().(*ProtocolBFTCoSi)
		root.Msg = msg
		root.RegisterOnDone(doneFunc)
		go node.StartProtocol()
		select {
		case <-done:
		case <-time.After(time.Second * 2):
			t.Fatal("Waited 2 sec for BFTCoSi to finish ...")
		}
		local.CloseAll()
	}
}
