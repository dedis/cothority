package messaging

import (
	"testing"

	"github.com/dedis/onet/log"
	"go.dedis.ch/kyber/suites"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
