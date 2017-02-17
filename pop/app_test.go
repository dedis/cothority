package main

import (
	"io/ioutil"
	"testing"

	"os"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1/log"
)

func TestConfigNew(t *testing.T) {
	tmp, err := ioutil.TempFile("", "config")
	log.ErrFatal(err)
	tmp.Close()
	defer os.Remove(tmp.Name())
	cfg, err := newConfig(tmp.Name())
	require.NotNil(t, err)
	os.Remove(tmp.Name())
	cfg, err = newConfig(tmp.Name())
	log.ErrFatal(err)
	require.Equal(t, -1, cfg.Index)
	cfg.Index = 777
	cfg.write()

	cfg, err = newConfig(tmp.Name())
	log.ErrFatal(err)
	require.Equal(t, 777, cfg.Index)
}

func TestMainFunc(t *testing.T) {
	os.Args = []string{os.Args[0], "--help"}
	main()
}
