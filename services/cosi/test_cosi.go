package cosi

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"testing"
)

// Test hacky way of launching CoSi protocol externally
func TestHackyCosi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	// make up the hosts and the entity list
	h1 := sda.NewLocalHost(2000)
	h1.Listen()
	h1.StartProcessMessages()
	defer h1.Close()
	h2 := sda.NewLocalHost(2010)
	h2.Listen()
	h2.StartProcessMessages()
	defer h2.Close()
	h3 := sda.NewLocalHost(2020)
	h3.Listen()
	h3.StartProcessMessages()
	defer h3.Close()

	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity, h3.Entity})

	// create the fake client
	kp := config.NewKeyPair(network.Suite)
	client := network.NewSecureTCPHost(kp.Secret, network.NewEntity(kp.Public, "localhost:3000"))
	msg := []byte("Hello World")
	req := &SignRequest{
		EntityList: el,
		Message:    msg,
	}

	// Connect to the root
	con, err := client.Open(h1.Entity)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// send the request
	if err := con.Send(context.TODO(), req); err != nil {
		t.Fatal(err)
	}
	// wait for the response
	packet, err := con.Receive(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	// verify signature
	response, ok := packet.Msg.(SignResponse)
	if !ok {
		t.Fatal("Could not cast the response to a CoSiResponse")
	}
	if err := cosi.VerifySignature(network.Suite, msg, el.Aggregate, response.Challenge, response.Response); err != nil {
		t.Fatal("Response has not been verified correctly")
	}

}
