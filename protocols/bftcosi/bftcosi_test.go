package bftcosi

import (
	"fmt"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// Dummy verification function: always returns OK/true/no-error on data
var veriCount int

func veri(m []byte, ok chan bool) {
	veriCount++
	dbg.Print("Verification called", veriCount, "times")
	dbg.Print("Ignoring message:", string(m))
	// everything is OK, always:
	ok <- true
}

const TestProtocolName = "DummyBFTCoSi"

func TestBftCoSi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.Node) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, veri)
	})

	for _, nbrHosts := range []int{3, 13} {
		veriCount = 0
		dbg.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
		local := sda.NewLocalTest()
		defer local.CloseAll()

		_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello BFTCoSi")

		// Start the protocol
		node, err := local.CreateNewNodeName(TestProtocolName, tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}

		// Register the function generating the protocol instance
		var root *ProtocolBFTCoSi
		root = node.ProtocolInstance().(*ProtocolBFTCoSi)
		root.Msg = msg
		// function that will be called when protocol is finished by the root
		root.RegisterOnDone(func() {
			done <- true
		})
		go node.StartProtocol()
		// are we done yet?
		wait := time.Second * 3
		select {
		case <-done:
			assert.Equal(t, veriCount, nbrHosts,
				"Each host should have called verification.")
			sig := root.Signature()
			if err := cosi.VerifyCosiSignatureWithException(root.Suite(),
				root.aggregatedPublic, msg, sig.Sig,
				sig.Exceptions); err != nil {

				t.Fatal(fmt.Sprintf("%s Verification of the signature failed: %s", root.Name(), err.Error()))
			}
		case <-time.After(wait):
			t.Fatal("Waited", wait, "sec for BFTCoSi to finish ...")
		}
		local.CloseAll()
	}
}
