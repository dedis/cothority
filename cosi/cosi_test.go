package main

import (
	"os"
	"testing"
)

func TestMainFunc(t *testing.T) {
	os.Args = []string{os.Args[0], "--help"}
	main()
}
