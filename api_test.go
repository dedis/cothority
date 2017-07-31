package onchain_secrets_test

import (
	"testing"

	// We need to include the service so it is started.
	_ "github.com/dedis/onchain-secrets/service"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}
