package main

import (
	log "github.com/ineiti/cothorities/test/debug_lvl/debug_lvl"
)

func main() {
	log.DebugVisible = 3
	log.Println(1, "Hello", "there")
	log.Println(2, "Hi there")
	log.Println(3, "Bonjour")
}
