package sda

import (
	"testing"

	"gopkg.in/dedis/cothority.v0/lib/dbg"
)

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	dbg.MainTest(m)
}
