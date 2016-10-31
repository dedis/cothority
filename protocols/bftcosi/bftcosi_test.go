package bftcosi

import (
	"sync"
	"testing"
	"time"

	"strconv"

	"errors"

	"fmt"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

type Counter struct {
	veriCount   int
	refuseCount int
	sync.Mutex
}

type Counters struct {
	counters []*Counter
	sync.Mutex
}

func (co *Counters) add(c *Counter) {
	co.Lock()
	co.counters = append(co.counters, c)
	co.Unlock()
}

func (co *Counters) size() int {
	co.Lock()
	defer co.Unlock()
	return len(co.counters)
}

func (co *Counters) get(i int) *Counter {
	co.Lock()
	defer co.Unlock()
	return co.counters[i]
}

var counters = &Counters{}
var cMux sync.Mutex

func TestMain(m *testing.M) {
	log.MainTest(m, 5)
}

func TestBftCoSi(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSi"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify)
	})

	log.Lvl2("Simple count")
	runProtocol(t, TestProtocolName, 0)
}

func TestThreshold(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiThr"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
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
		_, _, tree := local.GenBigTree(hosts, hosts, 2, true)
		log.Lvl3("Tree is:", tree.Dump())

		// Start the protocol
		node, err := local.CreateProtocol(TestProtocolName, tree)
		log.ErrFatal(err)
		bc := node.(*ProtocolBFTCoSi)
		assert.Equal(t, thr, bc.threshold, "hosts was %d", hosts)
		local.CloseAll()
	}
}

func TestCheckRefuse(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiRefuse"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyRefuse)
	})

	for refuseCount := 1; refuseCount <= 3; refuseCount++ {
		log.Lvl2("Refuse at", refuseCount)
		runProtocol(t, TestProtocolName, refuseCount)
	}
}

func TestCheckRefuseMore(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiRefuseMore"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyRefuseMore)
	})

	for _, n := range []int{3, 4, 13} {
		for refuseCount := 1; refuseCount <= 3; refuseCount++ {
			log.Lvl2("RefuseMore at", refuseCount)
			runProtocolOnce(t, n, TestProtocolName,
				refuseCount, refuseCount < (n+1)*2/3)
		}
	}
}

func TestCheckRefuseBit(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiRefuseBit"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyRefuseBit)
	})

	wg := sync.WaitGroup{}
	for _, n := range []int{3} {
		for refuseCount := 0; refuseCount < 1<<uint(n); refuseCount++ {
			wg.Add(1)
			go func(n, fc int) {
				log.Lvl1("RefuseBit at", n, fc)
				log.ErrFatal(runProtocolOnceGo(n, TestProtocolName,
					fc, bitCount(fc) < (n+1)*2/3))
				log.Lvl3("Done with", n, fc)
				wg.Done()
			}(n, refuseCount)
		}
	}
	wg.Wait()
}

func TestCheckRefuseParallel(t *testing.T) {
	//t.Skip("Skipping and hoping it will be resolved with #467")
	const TestProtocolName = "DummyBFTCoSiRefuseParallel"

	// Register test protocol using BFTCoSi
	sda.GlobalProtocolRegister(TestProtocolName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verifyRefuseBit)
	})

	wg := sync.WaitGroup{}
	n := 3
	for fc := 0; fc < 8; fc++ {
		wg.Add(1)
		go func(fc int) {
			log.ErrFatal(runProtocolOnceGo(n, TestProtocolName,
				fc, bitCount(fc) < (n+1)*2/3))
			log.Lvl3("Done with", n, fc)
			wg.Done()
		}(fc)
		//wg.Wait()
	}
	wg.Wait()
}

func runProtocol(t *testing.T, name string, refuseCount int) {
	for _, nbrHosts := range []int{3, 4, 13} {
		runProtocolOnce(t, nbrHosts, name, refuseCount, true)
	}
}
func runProtocolOnce(t *testing.T, nbrHosts int, name string, refuseCount int,
	succeed bool) {
	if err := runProtocolOnceGo(nbrHosts, name, refuseCount, succeed); err != nil {
		t.Fatal(err)
	}
}
func runProtocolOnceGo(nbrHosts int, name string, refuseCount int,
	succeed bool) error {
	log.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 2, true)
	log.Lvl3("Tree is:", tree.Dump())

	done := make(chan bool)
	// create the message we want to sign for this round
	msg := []byte("Hello BFTCoSi")

	// Start the protocol
	node, err := local.CreateProtocol(name, tree)
	if err != nil {
		return errors.New("Couldn't create new node: " + err.Error())
	}

	// Register the function generating the protocol instance
	var root *ProtocolBFTCoSi
	root = node.(*ProtocolBFTCoSi)
	root.Msg = msg
	cMux.Lock()
	counter := &Counter{refuseCount: refuseCount}
	counters.add(counter)
	root.Data = []byte(strconv.Itoa(counters.size() - 1))
	log.Lvl3("Added counter", counters.size()-1, refuseCount)
	cMux.Unlock()
	log.ErrFatal(err)
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	log.Lvl1("Launched protocol")
	// are we done yet?
	wait := time.Second * 60
	select {
	case <-done:
		counter.Lock()
		if counter.veriCount != nbrHosts {
			return errors.New("Each host should have called verification.")
		}
		// if assert refuses we don't care for unlocking (t.Refuse)
		counter.Unlock()
		sig := root.Signature()
		err := sig.Verify(root.Suite(), root.Roster().Publics())
		if succeed && err != nil {
			return fmt.Errorf("%s Verification of the signature refused: %s - %+v", root.Name(), err.Error(), sig.Sig)
		}
		if !succeed && err == nil {
			return fmt.Errorf("%s: Shouldn't have succeeded for %d hosts, but signed for count: %d",
				root.Name(), nbrHosts, refuseCount)
		}
	case <-time.After(wait):
		log.Lvl1("Going to break because of timeout")
		return errors.New("Waited " + wait.String() + " for BFTCoSi to finish ...")
	}
	return nil
}

// Verify function that returns true if the length of the data is 1.
func verify(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters.get(c)
	counter.Lock()
	counter.veriCount++
	log.Lvl4("Verification called", counter.veriCount, "times")
	counter.Unlock()
	if len(d) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// Verify-function that will refuse if we're the `refuseCount`ed call.
func verifyRefuse(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters.get(c)
	counter.Lock()
	defer counter.Unlock()
	counter.veriCount++
	if counter.veriCount == counter.refuseCount {
		log.Lvl2("Refusing for count==", counter.refuseCount)
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

// Verify-function that will refuse for all calls >= `refuseCount`.
func verifyRefuseMore(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters.get(c)
	counter.Lock()
	defer counter.Unlock()
	counter.veriCount++
	if counter.veriCount <= counter.refuseCount {
		log.Lvlf2("Refusing for %d<=%d", counter.veriCount,
			counter.refuseCount)
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

// Verify-function that will refuse if the `called` bit is 0.
func verifyRefuseBit(m []byte, d []byte) bool {
	c, err := strconv.Atoi(string(d))
	log.ErrFatal(err)
	counter := counters.get(c)
	counter.Lock()
	defer counter.Unlock()
	log.Lvl4("Counter", c, counter.refuseCount, counter.veriCount)
	myBit := uint(counter.veriCount)
	counter.veriCount++
	if counter.refuseCount&(1<<myBit) != 0 {
		log.Lvl2("Refusing for myBit ==", myBit)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	return true
}
