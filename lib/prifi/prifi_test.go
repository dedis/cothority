package prifi

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
)

func TestPrifi(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	dbg.Lvl1("Testing PriFi protocol...")

	nbrHosts := 2
	local := sda.NewLocalTest()
	hosts, el, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true, true)
	p, err := local.CreateProtocol("PriFi", tree)

	//var client1 *ClientState = initClient(0, 1, 1, 1000, false, false, false)

}
