package jvss_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/testutil"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/crypto/poly"
	"github.com/satori/go.uuid"
)

var CustomJVSSProtocolID = uuid.NewV5(uuid.NamespaceURL, "jvss_test")

// Test if the setup of the long-term secret for one protocol instance is correct
// or not.
func TestJVSSLongterm(t *testing.T) {
	defer testutil.AfterTest(t)
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
	ch := make(chan *poly.SharedSecret, 2)
	var p1 *jvss.JVSSProtocol
	fn := func(node *sda.Node) (sda.ProtocolInstance, error) {
		pi, err := jvss.NewJVSSProtocol(node)
		if err != nil {
			return nil, err
		}
		pi.RegisterOnLongtermDone(func(sh *poly.SharedSecret) {
			go func() {
				ch <- sh
			}()
		})
		p1 = pi
		return pi, nil
	}
	sda.ProtocolRegister(CustomJVSSProtocolID, fn)
	// Create the entityList  + tree
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	go h1.StartNewNode(CustomJVSSProtocolID, tree)

	// wait for the longterm secret to be generated:
	res := make([]*poly.SharedSecret, 2)
	for count := 0; count != 2; {
		select {
		case poly := <-ch:
			res[count] = poly
			count++
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout on the longterm distributed secret generation")
		}
	}

	if !res[0].Pub.Equal(res[1].Pub) {
		t.Fatal("longterm generated are not equal")
	}

}

// Test if the setup of the longterm secret for one protocol instance is correct
// or not.
func TestJVSSSign(t *testing.T) {
	defer testutil.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	// setup two hosts
	hosts := sda.SetupHostsMock(network.Suite, "127.0.0.1:2000", "127.0.0.1:4000")
	h1, h2 := hosts[0], hosts[1]
	// connect them
	h1.Connect(h2.Entity)
	defer h1.Close()
	defer h2.Close()
	var done1 bool
	doneLongterm := make(chan bool)
	var p1 *jvss.JVSSProtocol
	fn := func(node *sda.Node) (sda.ProtocolInstance, error) {
		pi, err := jvss.NewJVSSProtocol(node)
		if err != nil {
			return nil, err
		}
		if !done1 {
			// only care about the first host
			pi.RegisterOnLongtermDone(func(sh *poly.SharedSecret) {
				go func() {
					doneLongterm <- true
				}()
			})
			done1 = true
			p1 = pi
		}
		return pi, nil
	}
	sda.ProtocolRegister(CustomJVSSProtocolID, fn)
	// Create the entityList  + tree
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	// start the protocol
	go h1.StartNewNode(CustomJVSSProtocolID, tree)
	// wait for the long-term secret to be generated
	select {
	case <-doneLongterm:
		break
	case <-time.After(time.Second * 5):
		t.Fatal("Timeout on the longterm distributed secret generation")
	}

	// now make the signing
	msg := []byte("Hello World\n")
	doneSig := make(chan bool)
	var schnorrSig *poly.SchnorrSig
	var err error
	go func() {
		schnorrSig, err = p1.Sign(msg)
		doneSig <- true
	}()

	// wait for the signing or timeout
	select {
	case <-doneSig:
	//it's fine
	case <-time.After(time.Second * 5):
		t.Fatal("Could not get the signature done before timeout")
	}

	// verify it
	if err != nil {
		t.Fatal("Error generating signature:", err)
	}
	err = p1.Verify(msg, schnorrSig)
	if err != nil {
		t.Fatal("Could not verify signature :s", err)
	}
}
