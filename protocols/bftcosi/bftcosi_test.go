package bftcosi

import (
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
var failCount int
var countMut sync.Mutex

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestBftCoSi(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSi"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	for i := 1; i <= 3; i++ {
		dbg.Lvl1("Standard at", failCount)
		runProtocol(t, TestProtocolName)
	}
}

func TestCheckFail(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiFail"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFail)
	})

	for failCount = 1; failCount <= 3; failCount++ {
		dbg.Lvl1("Fail at", failCount)
		runProtocol(t, TestProtocolName)
	}
}

func TestCheckFailMore(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiFailMore"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailMore)
	})

	for _, n := range []int{3, 13} {
		for failCount = 1; failCount <= 3; failCount++ {
			dbg.Lvl1("FailMore at", failCount)
			runProtocolOnce(t, n, TestProtocolName, failCount == 1 || n > 3)
		}
	}
}

func runProtocol(t *testing.T, name string) {
	for _, nbrHosts := range []int{3, 13} {
		runProtocolOnce(t, nbrHosts, name, true)
	}
}
func runProtocolOnce(t *testing.T, nbrHosts int, name string, succeed bool) {
	countMut.Lock()
	veriCount = 0
	countMut.Unlock()
	dbg.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)

	done := make(chan bool)
	// create the message we want to sign for this round
	msg := []byte("Hello BFTCoSi")

	// Start the protocol
	node, err := local.CreateProtocol(tree, name)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}

	// Register the function generating the protocol instance
	var root *ProtocolBFTCoSi
	root = node.(*ProtocolBFTCoSi)
	root.Msg = msg
	root.Data = []byte("1")
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
		err := cosi.VerifyCosiSignatureWithException(root.Suite(),
			root.AggregatedPublic, msg, sig.Sig, nil)
		if succeed && err != nil {
			t.Fatalf("%s Verification of the signature failed: %s", root.Name(), err.Error())
		}
		if !succeed && err == nil {
			t.Fatal(root.Name(), "Shouldn't have succeeded for", nbrHosts, "hosts, and failing:", failCount)
		}
	case <-time.After(wait):
		t.Fatal("Waited", wait, "sec for BFTCoSi to finish ...")
	}
}

func verify(m []byte, d []byte) bool {
	countMut.Lock()
	veriCount++
	dbg.Lvl1("Verification called", veriCount, "times")
	countMut.Unlock()
	dbg.Lvl1("Ignoring message:", string(m))
	if len(d) != 1 {
		dbg.Error("Didn't receive correct data")
		return false
	}
	// everything is OK, always:
	return true
}

func verifyFail(m []byte, d []byte) bool {
	countMut.Lock()
	defer countMut.Unlock()
	veriCount++
	if veriCount == failCount {
		dbg.Lvl1("Failing for count==", failCount)
		return false
	}
	dbg.Lvl1("Verification called", veriCount, "times")
	dbg.Lvl1("Ignoring message:", string(m))
	if len(d) != 1 {
		dbg.Error("Didn't receive correct data")
		return false
	}
	// everything is OK, always:
	return true
}

func verifyFailMore(m []byte, d []byte) bool {
	countMut.Lock()
	defer countMut.Unlock()
	veriCount++
	if veriCount <= failCount {
		dbg.Lvl1("Failing for count==", failCount)
		return false
	}
	dbg.Lvl1("Verification called", veriCount, "times")
	dbg.Lvl1("Ignoring message:", string(m))
	if len(d) != 1 {
		dbg.Error("Didn't receive correct data")
		return false
	}
	// everything is OK, always:
	return true
}
