package bftcosi

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
)

// Dummy verification function: always returns OK/true/no-error on data
var veriCount int
var countMut sync.Mutex

const TestProtocolName = "DummyBFTCoSi"

func TestBftCoSi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	for _, nbrHosts := range []int{3, 13} {
		countMut.Lock()
		veriCount = 0
		countMut.Unlock()
		dbg.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
		local := sda.NewLocalTest()
		_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello BFTCoSi")

		// Start the protocol
		node, err := local.CreateProtocol(TestProtocolName, tree)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}

		// Register the function generating the protocol instance
		var root *ProtocolBFTCoSi
		root = node.(*ProtocolBFTCoSi)
		root.Msg = msg
		// function that will be called when protocol is finished by the root
		root.RegisterOnDone(func() {
			done <- true
		})
		go node.Start()
		// are we done yet?
		wait := time.Second * 3
		select {
		case <-done:
			countMut.Lock()
			assert.Equal(t, veriCount, nbrHosts,
				"Each host should have called verification.")
			// if assert fails we don't care for unlocking (t.Fail)
			countMut.Unlock()
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

func verify(m []byte) bool {
	countMut.Lock()
	veriCount++
	dbg.Lvl1("Verification called", veriCount, "times")
	countMut.Unlock()
	dbg.Lvl1("Ignoring message:", string(m))
	// everything is OK, always:
	return true
}
