package conode_test

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	go peer1.LoopRounds()
	go peer2.LoopRounds()
	time.Sleep(time.Second * 2)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range ([]int{2000, 2000}) {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		tsm, err := s.GetStamp([]byte("test"), stamper)
		dbg.Lvl3("Evaluating results of", stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

		if !tsm.Srep.AggPublic.Equal(s.X0) {
			t.Fatal("Not correct aggregate public key")
		}
	}

	peer1.Close()
	peer2.Close()
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

func runConode(conf *app.ConfigConode, id int) {
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
	peer := conode.NewPeer(address, conf)
	if id == 1 {
		time.Sleep(time.Second)
	}
	peer.LoopRounds()
}
