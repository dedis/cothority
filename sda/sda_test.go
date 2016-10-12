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
	log.MainTest(m, 3)
}

// Returns a fresh ServerIdentity + private key with the
// address "127.0.0.1:" + port
func NewTestIdentity(port int) (*network.ServerIdentity, abstract.Scalar) {
	addr := network.NewAddress(network.Local, "127.0.0.1:"+strconv.Itoa(port))
	kp := config.NewKeyPair(network.Suite)
	return network.NewServerIdentity(kp.Public, addr), kp.Secret
}
