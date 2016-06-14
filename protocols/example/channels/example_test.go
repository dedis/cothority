package example_channels_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/example/channels"
)

// Tests a 2-node system
func TestNode(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)
	sda.ProtocolRegisterName("ExampleChannels", NewProtocol) // change the constructor to local constructor
	defer local.CloseAll()

	p, err := local.CreateProtocol(tree, "ExampleChannels")
	if err != nil {
		t.Fatal("Couldn't create protocol:", err)
	}


	protocol := p.(*example_channels.ProtocolExampleChannels)
	protocol.Start()
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

// Tests a 2-node system
func TestNode2(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)

	// Register a constructor at host level
	for _,h := range local.Hosts {
		h.RegisterNewProtocol(NewProtocol)
	}
	defer local.CloseAll()

	p, err := local.StartProtocol("ExampleChannels",tree)
	if err != nil {
		t.Fatal("Couldn't create protocol:", err)
	}

	protocol := p.(*example_channels.ProtocolExampleChannels)
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

func NewProtocol(tn *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	pi, err := example_channels.NewExampleChannels(tn)
	pi.(*example_channels.ProtocolExampleChannels).Message = "This works."
	return pi,err
}