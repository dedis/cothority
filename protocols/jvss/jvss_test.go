package jvss_test

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/crypto/poly"
	"github.com/satori/go.uuid"
	"testing"
	"time"
)

var CustomJVSSProtocolID = uuid.NewV5(uuid.NamespaceURL, "jvss_test")

// Test if the setup of the longterm secret for one protocol instance is correct
// or not.
func TestJVSSLongterm(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	// setup two hosts
	hosts := sda.SetupHostsMock(network.Suite, "127.0.0.1:2000", "127.0.0.1:4000")
	h1, h2 := hosts[0], hosts[1]
	// connect them
	h1.Connect(h2.Entity)
	defer h1.Close()
	defer h2.Close()
	// register the protocol with our custom channels so we know at which steps
	// are both of the hosts
	ch1 := make(chan *poly.SharedSecret)
	ch2 := make(chan *poly.SharedSecret)
	var done1 bool
	var done2 bool
	fn := func(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
		pi := jvss.NewJVSSProtocol(h, t, tok)
		pi.RegisterOnLongtermDone(func(sh *poly.SharedSecret) {
			go func() {
				if !done1 {
					done1 = true
					ch1 <- sh
				} else {
					done2 = true
					ch2 <- sh
				}
			}()
		})
		return pi
	}
	sda.ProtocolRegister(CustomJVSSProtocolID, fn)
	// Create the entityList  + tree
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)
	fmt.Println("Configuration done")
	go h1.StartNewProtocol(CustomJVSSProtocolID, tree.Id)
	// wait for the longterm secret to be generated
	var found1 bool
	var found2 bool
	var found bool
	for !found {

		select {
		case <-ch1:
			fmt.Println("Channel 1 received")
			found1 = true
			if found2 {
				found = true
				break
			}
		case <-ch2:
			fmt.Println("Channel 2 ")
			found2 = true
			if found1 {
				found = true
				break
			}
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout on the longterm distributed secret generation")
		}
	}

}
