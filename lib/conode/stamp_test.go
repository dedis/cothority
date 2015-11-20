package conode_test

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"strconv"
	"testing"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	conf := readConfig()
	go runConode(conf, 1)
	// conf will hold part of the configuration for each server,
	// so we have to create a second one for the second server
	conf = readConfig()
	go runConode(conf, 2)
	time.Sleep(time.Second * 2)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range ([]int{2000, 2010}) {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		tsm, err := s.GetStamp([]byte("test"), stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

		if !tsm.Srep.AggPublic.Equal(s.X0) {
			t.Fatal("Not correct aggregate public key")
		}
	}
	//stopConode()
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
	conode.RunServer(address, conf)
}
