package crypto_test

import (
	"flag"
	"os"
	"testing"

	"gopkg.in/dedis/cothority.v0/lib/dbg"
)

func TestMain(m *testing.M) {
	flag.Parse()
	dbg.TestOutput(testing.Verbose(), 4)
	code := m.Run()
	dbg.AfterTest(nil)
	os.Exit(code)
}
