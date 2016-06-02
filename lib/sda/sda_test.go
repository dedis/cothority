package sda

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
)

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	dbg.MainTest(m)
}
