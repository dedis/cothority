package main

import (
	"testing"

	"io/ioutil"
	"os"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/identity"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestLoadConfig(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "config.bin")
	dbg.ErrFatal(err)
	tmpfile.Close()
	configFile = tmpfile.Name()
	os.Remove(configFile)
	err = LoadConfig()
	assert.NotNil(t, err)

	local := sda.NewLocalTest()
	_, el, _ := local.GenTree(5, false, false, false)
	defer local.CloseAll()
	clientApp = identity.NewIdentity(el, 50, "one1", "sshpub1")

	err = SaveConfig()
	dbg.ErrFatal(err)

	clientApp = nil
	err = LoadConfig()
	dbg.ErrFatal(err)

	if clientApp.Config.Threshold != 50 {
		t.Fatal("Threshold not correctly loaded")
	}
	if len(clientApp.Config.Owners) != 1 {
		t.Fatal("Owners not correctly loaded")
	}
}

func TestSetup(t *testing.T) {

}
