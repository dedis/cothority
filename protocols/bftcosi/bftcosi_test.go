package bftcosi

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

// Dummy verification function: always returns OK/true/no-error on data
var counter int

func veri(m []byte, ok chan bool) error {
	counter++
	dbg.Print("Verification called", counter, "times")
	// everything is OK, always:
	ok <- true
	return nil
}

func TestBftCoSi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)

	sda.ProtocolRegisterName("DummyBFTCoSi", func(n *sda.Node) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, veri)
	})

	for _, nbrHosts := range []int{3, 13} {
		counter = 0
		dbg.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
		local := sda.NewLocalTest()
		defer local.CloseAll()

		_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello BFTCoSi")

		// Start the protocol
		node, err := local.CreateNewNodeName("DummyBFTCoSi", tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}

		// Register the function generating the protocol instance
		var root *ProtocolBFTCoSi
		root = node.ProtocolInstance().(*ProtocolBFTCoSi)
		root.Msg = msg
		root.ProtoName = "DummyBFTCosi"
		// function that will be called when protocol is finished by the root
		root.RegisterOnDone(func() {
			done <- true
		})
		go node.StartProtocol()
		// are we done yet?
		wait := time.Second * 3
		select {
		case <-done:
		case <-time.After(wait):
			t.Fatal("Waited", wait, "sec for BFTCoSi to finish ...")
		}
	}
}
