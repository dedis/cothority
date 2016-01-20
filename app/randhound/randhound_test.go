package randhound_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/app/randhound"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
)

func TestRandHound(t *testing.T) {

	// setup general parameters
	var npeers int = 10
	var trustees int = 5
	var purpose string = "RandHound test run"
	var ip string = "localhost"
	var port int = 2000
	configs := make([]string, npeers)
	for i := 0; i < npeers; i++ {
		configs[i] = ip + ":" + strconv.Itoa(i+port)
	}

	// setup hosts
	h := setupHosts(t, configs)
	l := make([]*network.Entity, len(h))
	go h[0].ProcessMessages() // h[0] is the leader / protocol initiator
	for i := range h {
		defer h[i].Close()
		l[i] = h[i].Entity
	}

	list := sda.NewEntityList(l)
	tree, _ := list.GenerateBinaryTree()
	h[0].AddEntityList(list)
	h[0].AddTree(tree)

	// Setup RandHound channels and send parameters (TODO: this should not be done here, find a better solution)
	if randhound.Purpose == nil {
		randhound.Purpose = make(chan string, 1)
	}
	if randhound.Trustees == nil {
		randhound.Trustees = make(chan int, 1)
	}
	randhound.Purpose <- purpose
	randhound.Trustees <- trustees

	// run RandHound protocol
	dbg.Lvl1("RandHound: starting")
	_, err := h[0].StartNewProtocolName("RandHound", tree.Id)
	if err != nil {
		t.Fatal("Could not start protocol:", err)
	}

	select {
	case _ = <-randhound.Done:
		dbg.Lvl1("RandHound: done")
	case <-time.After(time.Second * 10):
		t.Fatal("RandHound did not finish in time")
	}

}

func newHost(t *testing.T, address string) *sda.Host {
	priv, pub := config.NewKeyPair(edwards.NewAES128SHA256Ed25519(false))
	entity := network.NewEntity(pub, address)
	return sda.NewHost(entity, priv)
}

func setupHosts(t *testing.T, configs []string) []*sda.Host {
	hosts := make([]*sda.Host, len(configs))
	for i := range configs {
		hosts[i] = newHost(t, configs[i])
		if i > 0 {
			hosts[i].Listen()
			_, err := hosts[0].Connect(hosts[i].Entity) // connect leader to all peers
			if err != nil {
				t.Fatal(err)
			}
			go hosts[i].ProcessMessages()
		}
	}
	return hosts
}
