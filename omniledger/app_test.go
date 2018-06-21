package main

import (
	"testing"

	"github.com/dedis/onet/log"
)

// You may put normal Go tests in this file. For more information
// on testing in Go see: https://golang.org/doc/code.html#Testing
func TestAddition(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("Addition does not work.")
	}
}

// It is useful for Onet applications to run code before and after the
// Go test framework, for example in order to configure logging, to
// set a global time limit, and to check for leftover goroutines.
//
// See:
//   - https://godoc.org/testing#hdr-Main
//   - https://godoc.org/github.com/dedis/onet/log#MainTest
func TestMain(m *testing.M) {
	log.MainTest(m)
}
