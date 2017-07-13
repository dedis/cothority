package main

import (
	"testing"

	"os"
)

func TestSimulation(t *testing.T) {
	os.Args = []string{os.Args[0], "cosi.toml"}
	main()
	os.Args = []string{os.Args[0], "cosi_verification.toml"}
	main()
}
