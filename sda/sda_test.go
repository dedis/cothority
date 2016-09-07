package sda

import (
	"strconv"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Returns a fresh ServerIdentity + private key with the
// address "127.0.0.1:" + port
func NewTestIdentity(port int) (*network.ServerIdentity, abstract.Scalar) {
	addr := network.NewAddress(network.Local, "127.0.0.1:"+strconv.Itoa(port))
	kp := config.NewKeyPair(network.Suite)
	return network.NewServerIdentity(kp.Public, addr), kp.Secret
}

func TwoTestHosts() (*Host, *Host) {
	id1, s1 := NewTestIdentity(2000)
	id2, s2 := NewTestIdentity(2001)

	r1, err := network.NewLocalRouter(id1)
	r2, err2 := network.NewLocalRouter(id2)
	if err != nil {
		panic(err)
	} else if err2 != nil {
		panic(err2)
	}

	h1 := NewHostWithRouter(id1, s1, r1)
	h2 := NewHostWithRouter(id2, s2, r2)

	go h1.Start()
	go h2.Start()

	return h1, h2
}
