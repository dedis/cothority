package conode_test

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
	"github.com/dedis/cothority/lib/sign"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestPeer(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	round, err := sign.NewRoundFromType("cosistamper", peer1.Node)
	if err != nil {
		dbg.Fatal("Couldn't create cosistamp", err)
	}
	peer1.StartAnnouncement(round)
	time.Sleep(time.Second)
	peer1.Close()
	peer2.Close()
}

func TestRoundCosiStamper(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	round1 := conode.NewRoundCosiStamper(peer1.Node)
	round2, err := sign.NewRoundFromType("cosistamper", peer1.Node)

	if err != nil {
		dbg.Fatal("Error when creating round:", err)
	}

	dbg.Lvlf2("Round1: %+v", round1)
	dbg.Lvlf2("Round2: %+v", round2)
	name1, name2 := round1.(*conode.RoundCosiStamper).Name,
	round2.(*conode.RoundCosiStamper).Name
	if name1 != name2 {
		t.Fatal("Hostname of first round is", name1, "and should be equal to", name2)
	}
	peer1.Close()
	peer2.Close()
}

func createPeers() (p1, p2 *conode.Peer) {
	conf := readConfig()
	peer1 := createPeer(conf, 1)
	dbg.Lvlf3("Peer 1 is %+v", peer1)

	// conf will hold part of the configuration for each server,
	// so we have to create a second one for the second server
	conf = readConfig()
	peer2 := createPeer(conf, 2)
	dbg.Lvlf3("Peer 2 is %+v", peer1)

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
