package randhound_test

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/app/randhound"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
)

func TestRandHound(t *testing.T) {

	// Setup parameters
	var name string = "RandHound"       // Protocol name
	var np int = 10                     // Number of peers
	var lidx int = 0                    // Index of the leader node
	var T, R, N int = 3, 3, 5           // VSS parameters (T <= R <= N)
	var p string = "RandHound test run" // Purpose
	var ip string = "localhost"         // IP address
	var bp int = 2000                   // Base port

	configs := make([]string, np)
	for i := 0; i < np; i++ {
		configs[i] = ip + ":" + strconv.Itoa(i+bp)
	}

	// Setup hosts (leader gets shuffled to index 0)
	h := setupHosts(t, configs, lidx)
	l := make([]*network.Entity, len(h))
	go h[0].ProcessMessages() // start the leader node
	for i := range h {
		defer h[i].Close()
		l[i] = h[i].Entity
	}
	list := sda.NewEntityList(l)
	tree, _ := list.GenerateBinaryTree()
	h[0].AddEntityList(list)
	h[0].AddTree(tree)

	// Register RandHound protocol
	fn := func(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
		return randhound.NewRandHound(h, t, tok, T, R, N, p)
	}
	sda.ProtocolRegisterName(name, fn)

	// Run RandHound protocol
	log.Printf("RandHound - starting")
	_, err := h[0].StartNewProtocolName(name, tree.Id)
	if err != nil {
		t.Fatal("Could not start protocol:", err)
	}

	select {
	case _ = <-randhound.Done:
		log.Printf("RandHound - done")
	case <-time.After(time.Second * 10):
		t.Fatal("RandHound did not finish in time")
	}
}

func newHost(t *testing.T, address string) *sda.Host {
	keypair := config.NewKeyPair(edwards.NewAES128SHA256Ed25519(false))
	entity := network.NewEntity(keypair.Public, address)
	return sda.NewHost(entity, keypair.Secret)
}

func setupHosts(t *testing.T, configs []string, lidx int) []*sda.Host {
	hosts := make([]*sda.Host, len(configs))

	// Setup the leader at index 0
	hosts[0] = newHost(t, configs[lidx])

	// Setup the peers
	j := 1
	for i := 0; i < len(configs); i += 1 {
		if i != lidx {
			hosts[j] = newHost(t, configs[i])
			hosts[j].Listen()
			_, err := hosts[0].Connect(hosts[j].Entity) // connect leader to peer
			if err != nil {
				t.Fatal(err)
			}
			go hosts[j].ProcessMessages()
			j += 1
		}
	}
	return hosts
}
