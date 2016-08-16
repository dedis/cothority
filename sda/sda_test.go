package sda

import (
	"testing"

	"fmt"

	"github.com/dedis/cothority/log"
)

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	fmt.Println("hi")
	log.MainTest(m)
}
