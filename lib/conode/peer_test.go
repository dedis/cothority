package conode_test

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
	"github.com/dedis/cothority/lib/sign"
"github.com/dedis/cothority/lib/graphs"
	"time"
)

func TestStampListener(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	round1 := conode.NewRoundCosiStamper(peer1.Node)
	round2, err := sign.NewRoundFromType("cosistamper", peer1.Node)

	if err != nil {
		dbg.Fatal("Error when creating round:", err)
	}

	dbg.Lvlf2("Round1: %+v", round1)
	dbg.Lvlf2("Round2: %+v", round2)
	name1, name2 := round1.Name, round2.(*conode.RoundCosiStamper).Name
	if name1 != name2 {
		t.Fatal("Hostname of first round is", name1, "and should be equal to", name2)
	}
	peer1.Close()
	peer2.Close()
}

// Can we build the Peer without a valid key?
func TestEmptyKeys(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	conf1 := readConfig()
	emptyKeys(conf1.Tree)
	peer1 := createPeer(conf1, 1)
	dbg.Lvlf3("Peer 1 is %+v", peer1)

	conf2 := readConfig()
	emptyKeys(conf2.Tree)
	peer2 := createPeer(conf2, 1)
	dbg.Lvlf3("Peer 1 is %+v", peer2)

	go peer1.LoopRounds("cosi", 2)
	go peer2.LoopRounds("cosi", 2)

	time.Sleep(time.Second*2)

	peer1.Close()
	peer2.Close()
}

func emptyKeys(t *graphs.Tree){
	t.PriKey = ""
	t.PubKey = ""
	for _, c := range t.Children{
		emptyKeys(c)
	}
}

func createPeers() (p1, p2 *conode.Peer) {
	conf1 := readConfig()
	peer1 := createPeer(conf1, 1)
	dbg.Lvlf3("Peer 1 is %+v", peer1)

	// conf will hold part of the configuration for each server,
	// so we have to create a second one for the second server
	conf2 := readConfig()
	peer2 := createPeer(conf2, 2)
	dbg.Lvlf3("Peer 2 is %+v", peer2)

	return peer1, peer2
}

func createPeer(conf *app.ConfigConode, id int) *conode.Peer {
	// Read the private / public keys + binded address
	keybase := "testdata/key" + strconv.Itoa(id)
	address := ""
	if sec, err := cliutils.ReadPrivKey(suite, keybase + ".priv"); err != nil {
		dbg.Fatal("Error reading private key file  :", err)
	} else {
		conf.Secret = sec
	}
	if pub, addr, err := cliutils.ReadPubKey(suite, keybase + ".pub"); err != nil {
		dbg.Fatal("Error reading public key file :", err)
	} else {
		conf.Public = pub
		address = addr
	}
	return conode.NewPeer(address, conf)
}

func readConfig() *app.ConfigConode {
	conf := &app.ConfigConode{}
	if err := app.ReadTomlConfig(conf, "testdata/config.toml"); err != nil {
		dbg.Fatal("Could not read toml config... : ", err)
	}
	dbg.Lvl2("Configuration file read")
	suite = app.GetSuite(conf.Suite)
	return conf
}
