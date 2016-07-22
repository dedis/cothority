package bftcosi

import (
	"sync"
	"testing"
	"time"

	"flag"
	"os"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

var veriCount int
var failCount int
var countMut sync.Mutex

func TestMain(m *testing.M) {
	//log.MainTest(m)
	flag.Parse()
	log.SetDebugVisible(1)
	code := m.Run()
	log.AfterTest(nil)
	os.Exit(code)
}

func TestBftCoSi(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSi"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	log.Lvl1("Standard at", failCount)
	runProtocol(t, TestProtocolName)
}

func TestThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test because of https://github.com/dedis/cothority/issues/467")
	}

	const TestProtocolName = "DummyBFTCoSiThr"
	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	local := sda.NewLocalTest()
	defer local.CloseAll()
	tests := []struct{ h, t int }{
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{5, 4},
		{6, 4},
	}
	for _, s := range tests {
		hosts, thr := s.h, s.t
		log.Lvl3("Hosts is", hosts)
		_, _, tree := local.GenBigTree(hosts, hosts, 2, true, true)
		log.Lvl3("Tree is:", tree.Dump())

		// Start the protocol
		node, err := local.CreateProtocol(tree, TestProtocolName)
		log.ErrFatal(err)
		bc := node.(*ProtocolBFTCoSi)
		assert.Equal(t, thr, bc.threshold, "hosts was %d", hosts)
		local.CloseAll()
	}
}

func TestCheckFail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test because of https://github.com/dedis/cothority/issues/467")
	}
	const TestProtocolName = "DummyBFTCoSiFail"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFail)
	})

	for failCount = 1; failCount <= 3; failCount++ {
		log.Lvl1("Fail at", failCount)
		runProtocol(t, TestProtocolName)
	}
}

func TestCheckFailMore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test because of https://github.com/dedis/cothority/issues/467")
	}
	const TestProtocolName = "DummyBFTCoSiFailMore"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailMore)
	})

	for _, n := range []int{3, 4, 13} {
		for failCount = 1; failCount <= 3; failCount++ {
			log.Lvl1("FailMore at", failCount)
			runProtocolOnce(t, n, TestProtocolName,
				failCount < (n+1)*2/3)
		}
	}
}

func TestCheckFailBit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test because of https://github.com/dedis/cothority/issues/467")
	}
	const TestProtocolName = "DummyBFTCoSiFailBit"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailBit)
	})

	for _, n := range []int{2, 3, 4} {
		for failCount = 0; failCount < 1<<uint(n); failCount++ {
			log.Lvl1("FailBit at", failCount)
			runProtocolOnce(t, n, TestProtocolName,
				bitCount(failCount) < (n+1)*2/3)

		}
	}
}

func runProtocol(t *testing.T, name string) {
	for _, nbrHosts := range []int{3, 4, 13} {
		runProtocolOnce(t, nbrHosts, name, true)
	}
}
func runProtocolOnce(t *testing.T, nbrHosts int, name string, succeed bool) {
	countMut.Lock()
	veriCount = 0
	countMut.Unlock()
	log.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 2, true, true)
	log.Lvl3("Tree is:", tree.Dump())

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
	wait := time.Second * 60
	select {
	case <-done:
		countMut.Lock()
		assert.Equal(t, veriCount, nbrHosts,
			"Each host should have called verification.")
		// if assert fails we don't care for unlocking (t.Fail)
		countMut.Unlock()
		sig := root.Signature()
		err := sig.Verify(root.Suite(), root.Roster().Publics())
		if succeed && err != nil {
			t.Fatalf("%s Verification of the signature failed: %s - %+v", root.Name(), err.Error(), sig.Sig)
		}
		if !succeed && err == nil {
			t.Fatal(root.Name(), "Shouldn't have succeeded for", nbrHosts, "hosts, but signed for count:", failCount)
		}
	case <-time.After(wait):
		t.Fatal("Waited", wait, "for BFTCoSi to finish ...")
	}
}

// Verify function that returns true if the length of the data is 1.
func verify(m []byte, d []byte) bool {
	countMut.Lock()
	veriCount++
	log.Lvl4("Verification called", veriCount, "times")
	countMut.Unlock()
	if len(d) != 1 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// Verify-function that will fail if we're the `failCount`ed call.
func verifyFail(m []byte, d []byte) bool {
	countMut.Lock()
	defer countMut.Unlock()
	veriCount++
	if veriCount == failCount {
		log.Lvl1("Failing for count==", failCount)
		return false
	}
	log.Lvl1("Verification called", veriCount, "times")
	log.Lvl1("Ignoring message:", string(m))
	if len(d) != 1 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// Verify-function that will fail for all calls >= `failCount`.
func verifyFailMore(m []byte, d []byte) bool {
	countMut.Lock()
	defer countMut.Unlock()
	veriCount++
	if veriCount <= failCount {
		log.Lvlf1("Failing for %d<=%d", veriCount, failCount)
		return false
	}
	log.Lvl1("Verification called", veriCount, "times")
	log.Lvl1("Ignoring message:", string(m))
	if len(d) != 1 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

func bitCount(x int) int {
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// Verify-function that will fail if the `called` bit is 0.
func verifyFailBit(m []byte, d []byte) bool {
	countMut.Lock()
	myBit := uint(veriCount)
	defer countMut.Unlock()
	veriCount++
	if failCount&(1<<myBit) != 0 {
		log.Lvl1("Failing for myBit==", myBit)
		return false
	}
	log.Lvl1("Verification called", veriCount, "times")
	return true
}
