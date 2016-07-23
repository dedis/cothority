package bftcosi

import (
	"sync"
	"testing"
	"time"

	"strconv"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

type Counter struct {
	veriCount int
	failCount int
	sync.Mutex
}

var counters []*Counter
var cMux sync.Mutex

func TestMain(m *testing.M) {
	//log.Info("skipping test because of https://github.com/dedis/cothority/issues/467")
	log.MainTest(m, 2)
}

func TestBftCoSi(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSi"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	log.Lvl2("Simple count")
	runProtocol(t, TestProtocolName, 0)
}

func TestThreshold(t *testing.T) {
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
	const TestProtocolName = "DummyBFTCoSiFail"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFail)
	})

	for failCount := 1; failCount <= 3; failCount++ {
		log.Lvl2("Fail at", failCount)
		runProtocol(t, TestProtocolName, failCount)
	}
}

func TestCheckFailMore(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiFailMore"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailMore)
	})

	for _, n := range []int{3, 4, 13} {
		for failCount := 1; failCount <= 3; failCount++ {
			log.Lvl2("FailMore at", failCount)
			runProtocolOnce(t, n, TestProtocolName,
				failCount, failCount < (n+1)*2/3)
		}
	}
}

func TestCheckFailBit(t *testing.T) {
	//t.Skip("Skipping and hoping it will be resolved with #467")
	const TestProtocolName = "DummyBFTCoSiFailBit"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailBit)
	})

	wg := sync.WaitGroup{}
	for _, n := range []int{3} {
		for failCount := 0; failCount < 1<<uint(n); failCount++ {
			wg.Add(1)
			go func(n, fc int) {
				log.Lvl1("FailBit at", n, fc)
				runProtocolOnce(t, n, TestProtocolName,
					fc, bitCount(fc) < (n+1)*2/3)
				log.LLvl3("Done with", n, fc)
				wg.Done()
			}(n, failCount)
		}
	}
	wg.Wait()
}

func TestCheckFailParallel(t *testing.T) {
	//t.Skip("Skipping and hoping it will be resolved with #467")
	const TestProtocolName = "DummyBFTCoSiFailParallel"

	// Register test protocol using BFTCoSi
	sda.ProtocolRegisterName(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyFailBit)
	})

	wg := sync.WaitGroup{}
	n := 3
	for fc := 0; fc < 8; fc++ {
		wg.Add(1)
		go func(fc int) {
			runProtocolOnce(t, n, TestProtocolName,
				8, bitCount(fc) < (n+1)*2/3)
			log.LLvl3("Done with", n, fc)
			wg.Done()
		}(fc)
		//wg.Wait()
	}
	wg.Wait()
}

func runProtocol(t *testing.T, name string, failCount int) {
	for _, nbrHosts := range []int{3, 4, 13} {
		runProtocolOnce(t, nbrHosts, name, failCount, true)
	}
}
func runProtocolOnce(t *testing.T, nbrHosts int, name string, failCount int,
	succeed bool) {
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
	cMux.Lock()
	counter := &Counter{failCount: failCount}
	counters = append(counters, counter)
	root.Data = []byte(strconv.Itoa(len(counters) - 1))
	log.LLvl3("Added counter", len(counters)-1, failCount)
	cMux.Unlock()
	log.ErrFatal(err)
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	// are we done yet?
	wait := time.Second * 60
	select {
	case <-done:
		counter.Lock()
		assert.Equal(t, counter.veriCount, nbrHosts,
			"Each host should have called verification.")
		// if assert fails we don't care for unlocking (t.Fail)
		counter.Unlock()
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
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters[c]
	counter.Lock()
	defer counter.Unlock()
	counter.veriCount++
	log.Lvl4("Verification called", counter.veriCount, "times")
	counter.Unlock()
	if len(d) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// Verify-function that will fail if we're the `failCount`ed call.
func verifyFail(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters[c]
	counter.Lock()
	defer counter.Unlock()
	counter.veriCount++
	if counter.veriCount == counter.failCount {
		log.Lvl2("Failing for count==", counter.failCount)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	log.Lvl3("Ignoring message:", string(m))
	if len(d) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// Verify-function that will fail for all calls >= `failCount`.
func verifyFailMore(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters[c]
	counter.Lock()
	defer counter.Unlock()
	counter.veriCount++
	if counter.veriCount <= counter.failCount {
		log.Lvlf2("Failing for %d<=%d", counter.veriCount,
			counter.failCount)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	log.Lvl3("Ignoring message:", string(m))
	if len(d) == 0 {
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
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters[c]
	counter.Lock()
	defer counter.Unlock()
	log.LLvl4("Counter", c, counter)
	myBit := uint(counter.veriCount)
	counter.veriCount++
	if counter.failCount&(1<<myBit) != 0 {
		log.Lvl2("Failing for myBit==", myBit)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	return true
}
