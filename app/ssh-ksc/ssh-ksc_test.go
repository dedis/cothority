package main

import (
	"testing"

	"io/ioutil"
	"os"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/identity"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
	tmpCleanup()
}

func TestLoadConfig(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "config.bin")
	log.ErrFatal(err)
	tmpfile.Close()
	configFile = tmpfile.Name()
	os.Remove(configFile)
	err = loadConfig()
	assert.Nil(t, err)

	local := sda.NewLocalTest()
	_, el, _ := local.GenTree(5, false, false, false)
	defer local.CloseAll()
	clientApp = identity.NewIdentity(el, 50, "one1", "sshpub1")

	err = saveConfig()
	log.ErrFatal(err)

	clientApp = nil
	err = loadConfig()
	log.ErrFatal(err)

	if clientApp.Config.Threshold != 50 {
		t.Fatal("Threshold not correctly loaded")
	}
	if len(clientApp.Config.Owners) != 1 {
		t.Fatal("Owners not correctly loaded")
	}
}

func TestSetup(t *testing.T) {
	tmpfile := tmpName()
	_, local := saveGroupToml(5, tmpfile)
	defer local.CloseAll()

	sshPub := tmpName()
	ioutil.WriteFile(sshPub, []byte("sshpub"), 0660)
	setup(tmpfile, "test", sshPub, "")

	assert.NotNil(t, clientApp)
	assert.NotNil(t, clientApp.Config)
	assert.Equal(t, "sshpub", clientApp.Config.Data["test"])
	assert.NotEqual(t, 0, len(clientApp.ID))
}

func saveGroupToml(n int, file string) (*config.GroupToml, *sda.LocalTest) {
	local := sda.NewLocalTest()
	hosts := local.GenLocalHosts(n, true, true)
	servers := make([]*config.ServerToml, n)
	for i, h := range hosts {
		pub, err := crypto.Pub64(network.Suite, h.Entity.Public)
		log.ErrFatal(err)
		servers[i] = &config.ServerToml{
			Addresses: h.Entity.Addresses,
			Public:    pub,
		}
	}
	gt := config.NewGroupToml(servers...)
	log.ErrFatal(gt.Save(file))
	return gt, local
}

var tmpfiles []string

func tmpName() string {
	file, err := ioutil.TempFile("", "tmpfile")
	log.ErrFatal(err)
	file.Close()
	tmpfiles = append(tmpfiles, file.Name())
	return file.Name()
}

func tmpCleanup() {
	for _, s := range tmpfiles {
		os.Remove(s)
	}
}
