package bftcosi

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
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

const defaultTimeout = 100 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestBftCoSi(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSi"

	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(TestProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify, defaultTimeout)
	})

	log.Lvl2("Simple count")
	runProtocol(t, TestProtocolName, 0, []int{3, 4, 13})
}

var tSuite = cothority.Suite

func TestThreshold(t *testing.T) {
	const TestProtocolName = "DummyBFTCoSiThr"

	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(TestProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify, defaultTimeout)
	})

	tests := []struct{ h, t int }{
		{1, 0},
		{2, 0},
		{3, 0},
		{4, 1},
		{5, 1},
		{7, 2},
	}
	for _, s := range tests {
		local := onet.NewLocalTest(tSuite)
		hosts, thr := s.h, s.t
		log.Lvl3("Hosts is", hosts)
		_, _, tree := local.GenBigTree(hosts, hosts, min(2, hosts-1), true)
		log.Lvl3("Tree is:", tree.Dump())

		// Start the protocol
		node, err := local.CreateProtocol(TestProtocolName, tree)
		log.ErrFatal(err)
		bc := node.(*ProtocolBFTCoSi)
		assert.Equal(t, thr, bc.AllowedExceptions, "hosts was %d", hosts)

		// we need to wait a bit for the protocols to finish because
		// some might not receive challenges and only exit after the
		// timeout
		time.Sleep(defaultTimeout * 2)
		local.CloseAll()
	}
}

func TestNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("node failure tests do not run on travis, see #1000")
	}

	const TestProtocolName = "DummyBFTCoSiNodeFailure"

	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(TestProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, verify, defaultTimeout)
	})

	nbrHostsArr := []int{5, 7, 10}
	for _, nbrHosts := range nbrHostsArr {
		if err := runProtocolOnceGo(nbrHosts, TestProtocolName, 0, true, 1, nbrHosts-1); err != nil {
			t.Fatalf("%d/%s/%d/%t: %s", nbrHosts, TestProtocolName, 0, true, err)
		}
	}
}

func runProtocol(t *testing.T, name string, refuseCount int, hosts []int) {
	for _, n := range hosts {
		runProtocolOnce(t, n, name, refuseCount, true)
	}
}

func runProtocolOnce(t *testing.T, nbrHosts int, name string, refuseCount int, succeed bool) {
	if err := runProtocolOnceGo(nbrHosts, name, refuseCount, succeed, 0, nbrHosts-1); err != nil {
		t.Fatalf("%d/%s/%d/%t: %s", nbrHosts, name, refuseCount, succeed, err)
	}
}

func runProtocolOnceGo(nbrHosts int, name string, refuseCount int, succeed bool, killCount int, bf int) error {
	log.Lvl2("Running BFTCoSi with", nbrHosts, "hosts")
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	// we set the branching factor to nbrHosts - 1 to have the root broadcast messages
	servers, _, tree := local.GenBigTree(nbrHosts, nbrHosts, bf, true)
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

	// kill the leafs first
	killCount = min(killCount, len(servers))
	for i := len(servers) - 1; i > len(servers)-killCount-1; i-- {
		log.Lvl3("Pausing server:", servers[i].ServerIdentity.Public, servers[i].Address())
		servers[i].Pause()
	}

	go root.Start()
	log.Lvl1("Launched protocol")
	// are we done yet?
	wait := time.Second * 60
	select {
	case <-done:
		counter.Lock()
		if counter.veriCount < nbrHosts-((nbrHosts-1)/3) {
			return errors.New("each host should have called verification")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
