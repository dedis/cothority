package crypto_test

import (
	"flag"
	"os"
	"testing"

	"github.com/dedis/cothority/log"
)

func TestMain(m *testing.M) {
	flag.Parse()
	log.TestOutput(testing.Verbose(), 4)
	code := m.Run()
	log.AfterTest(nil)
	os.Exit(code)
}
