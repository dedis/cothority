package messaging

import (
	"testing"

	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
