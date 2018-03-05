package messaging

import (
	"testing"

	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
