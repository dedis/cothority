package main

import (
	"os"
	"testing"
)

func TestSimulation(t *testing.T) {
	os.Args = []string{os.Args[0], "cosi.toml"}
	main()
	os.Args = []string{os.Args[0], "cosi_verification.toml"}
	main()
}
