package crypto_test

import (
	"flag"
	"os"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
)

func TestMain(m *testing.M) {
	flag.Parse()
	dbg.TestOutput(testing.Verbose(), 4)
	code := m.Run()
	dbg.AfterTest(nil)
	os.Exit(code)
}
