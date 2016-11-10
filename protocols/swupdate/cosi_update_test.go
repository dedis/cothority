package swupdate

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/cosi"
)

var name = "TestCoSiUpdate"

func TestCosi(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	var failingPercent = 0.5
	for i, nbrHosts := range []int{1, 3, 13} {
		failingHosts := math.Floor(float64(nbrHosts) * float64(failingPercent))
		log.Lvl2("Running cosi with", failingHosts, "/", nbrHosts, "failing hosts")

		protocolName := protoName(i)
		registerProtocol(protocolName, nbrHosts, int(failingHosts))

		local := sda.NewLocalTest()
		hosts, el, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)
		aggPublic := network.Suite.Point().Null()
		for _, e := range el.List {
			aggPublic = aggPublic.Add(aggPublic, e.Public)
		}

		done := make(chan bool)
		// create the message we want to sign for this round
		msg := []byte("Hello World Cosi")

		// Register the function generating the protocol instance
		var root *CoSiUpdate
		// function that will be called when protocol is finished by the root
		doneFunc := func(sig []byte) {
			suite := hosts[0].Suite()
			publics := el.Publics()
			if err := root.cosi.VerifyResponses(aggPublic); err != nil {
				t.Fatal("Error verifying responses", err)
			}
			if err := cosi.VerifySignature(suite, publics, msg, sig); err != nil {
				t.Fatal("Error verifying signature:", err)
			}
			done <- true
		}

		// Start the protocol
		p, err := local.CreateProtocol(tree, protocolName)
		if err != nil {
			t.Fatal("Couldn't create new node:", err)
		}
		root = p.(*CoSiUpdate)
		root.Message = msg
		root.RegisterSignatureHook(doneFunc)
		go root.StartProtocol()
		select {
		case <-done:
		case <-time.After(time.Second * 2):
			t.Fatal("Could not get signature verification done in time")
		}
		local.CloseAll()
	}
}

// registerProtocol will register the protocol name
// with *failing* number of nodes that refuse to sign
func registerProtocol(protoName string, nbrHosts, failing int) {
	var failedIdx []int
	for len(failedIdx) < failing {
		idx := rand.Int() % nbrHosts
		if idx != 0 {
			failedIdx = append(failedIdx, idx)
		}
	}
	sort.Sort(sort.IntSlice(failedIdx))
	var successFn = func(data []byte) bool {
		return true
	}
	var failFn = func(data []byte) bool {
		return false
	}
	sda.ProtocolRegisterName(protoName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		var fn VerificationHook
		if n.Index() != 0 && sort.SearchInts(failedIdx, n.Index()) == n.Index() {
			fn = failFn
		} else {
			fn = successFn
		}
		return NewCoSiUpdate(n, fn)
	})
}

func protoName(round int) string {
	r := strconv.Itoa(round)
	return name + "_" + r
}
