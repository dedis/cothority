package messaging

import (
	"testing"

	"github.com/dedis/kyber/group"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

var tSuite network.Suite

func init() {
	var err error
	tSuite, err = group.Suite("Ed25519")
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	log.MainTest(m)
}
