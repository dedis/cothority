package main

import (
	"flag"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/ssh-ks"
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
