package sda

import (
	"testing"

	"github.com/dedis/cothority/log"
	"gopkg.in/dedis/cothority.v0/lib/sda"
)

func TestGenLocalHost(t *testing.T) {
	l := sda.NewLocalTest()
	hosts := l.GenLocalHosts(2, false, false)
	defer l.CloseAll()

	log.Lvl4("Hosts are:", hosts[0].Address(), hosts[1].Address())
	if hosts[0].Address() == hosts[1].Address() {
		t.Fatal("Both addresses are equal")
	}
}
