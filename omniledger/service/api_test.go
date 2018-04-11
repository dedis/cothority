package lleap_test

import (
	"testing"

	// We need to include the service so it is started.
	"github.com/dedis/kyber/suites"
	_ "github.com/dedis/lleap/service"
	"github.com/dedis/onet/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
