package main

import (
	"testing"

	"os"
)

func TestSimulation(t *testing.T) {
	os.Args = []string{os.Args[0], "channels.toml"}
	main()
	os.Args = []string{os.Args[0], "handlers.toml"}
	main()
}
