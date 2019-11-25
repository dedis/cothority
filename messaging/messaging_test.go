package messaging

import (
	"testing"

	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
