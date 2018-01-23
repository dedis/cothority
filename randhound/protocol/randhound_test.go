package protocol

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func TestRandHound(test *testing.T) {

	name := "RandHound"
	nodes := 11
	groups := 2
	purpose := "RandHound test run"

	local := onet.NewLocalTest(cothority.Suite)
	_, _, tree := local.GenTree(int(nodes), true)
	defer local.CloseAll()

	log.Lvlf1("RandHound - starting")
	protocol, err := local.CreateProtocol(name, tree)
	if err != nil {
		test.Fatal("RandHound - couldn't initialise protocol:", err)
	}
	rh := protocol.(*RandHound)
	if err := rh.Setup(nodes, groups, purpose); err != nil {
		test.Fatal("RandHound - couldn't initialise protocol:", err)
	}
	if err := rh.Start(); err != nil {
		test.Fatal(err)
	}

	select {
	case <-rh.Done:
		log.Lvlf1("RandHound - done")

		random, transcript, err := rh.Random()
		if err != nil {
			test.Fatal(err)
		}
		log.Lvlf1("RandHound - collective randomness: ok")

		//log.Lvlf1("RandHound - collective randomness: %v", random)
		//_ = transcript

		err = Verify(rh.Suite(), random, transcript)
		if err != nil {
			test.Fatal(err)
		}
		log.Lvlf1("RandHound - verification: ok")

	case <-time.After(time.Second * time.Duration(nodes) * 2):
		test.Fatal("RandHound – time out")
	}
}
