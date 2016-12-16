package main

import (
	"testing"

	"io/ioutil"
	"os"

	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestWriteConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "config")
	log.ErrFatal(err)
	fileConfig = f.Name()
	f.Close()
	os.Remove(fileConfig)
	readConfig()
	require.NotNil(t, mainConfig)
	mainConfig.Address = network.NewAddress(network.PlainTCP, "127.0.0.1:2000")
	writeConfig()
	mainConfig = &Config{}
	readConfig()
	require.Equal(t, "2000", mainConfig.Address.Port())
	pub := network.Suite.Point().Mul(nil, mainConfig.Private)
	require.True(t, mainConfig.Public.Equal(pub))
	os.Remove(fileConfig)
}
