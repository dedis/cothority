package crypto_test

import (
	"flag"
	"github.com/dedis/cothority/lib/dbg"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	dbg.TestOutput(testing.Verbose(), 4)
	code := m.Run()
	dbg.AfterTest(nil)
	os.Exit(code)
}
