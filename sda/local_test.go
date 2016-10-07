package sda

import (
	"testing"

	"github.com/dedis/cothority/log"
)

func TestGenLocalHost(t *testing.T) {
	l := NewLocalTest()
	hosts := l.genLocalHosts(2)
	defer l.CloseAll()

	log.Lvl4("Hosts are:", hosts[0].Address(), hosts[1].Address())
	if hosts[0].Address() == hosts[1].Address() {
		t.Fatal("Both addresses are equal")
	}
}
