package network

import (
	"fmt"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

// Secure_test is analog to simple_test it uses the same structure to send
// The difference lies in which host and connections it uses. here we are going
// to use SecureTcpHost and SecureConn with ServerIdentities
// Now you connect to someone else using ServerIdentity instead of directly addresses

func TestSecureSimple(t *testing.T) {
	priv1, id1 := genServerIdentity("localhost:2000")
	priv2, id2 := genServerIdentity("localhost:2001")
	sHost1 := NewSecureTCPHost(priv1, id1)
	sHost2 := NewSecureTCPHost(priv2, id2)

	packetToSend := SimplePacket{"HelloWorld"}
	done := make(chan error)
	doneListen := make(chan bool)
	go func() {
		err := sHost1.Listen(func(c *SecureTCPConn) {
			nm, err := c.Receive(context.TODO())
			if err != nil {
				c.Close()
				done <- fmt.Errorf("Error receiving:")
			}
			if nm.MsgType != SimplePacketType {
				done <- fmt.Errorf("Wrong type received")
			}
			sp := nm.Msg.(SimplePacket)
			if sp.Name != packetToSend.Name {
				c.Close()
				done <- fmt.Errorf("Not same packet received!")
			}
			if !nm.ServerIdentity.Equal(id2) {
				c.Close()
				done <- fmt.Errorf("Not same entity")
			}
			log.Lvl3("Connection accepted")
			close(done)
		})
		if err != nil {
			t.Fatal("Listening-error:", err)
		}
		doneListen <- true
	}()
	//time.Sleep(1 * time.Second)
	// Open connection to entity
	c, err := sHost2.Open(id1)
	if err != nil {
		t.Fatal("Error during opening connection to id1")
	}

	ctx := context.TODO()
	if err := c.Send(ctx, &packetToSend); err != nil {
		c.Close()
		t.Fatal(err)
	}
	e, more := <-done
	if more && e != nil {
		t.Fatal(e)
	}
	err = sHost1.Close()
	if err != nil {
		t.Fatal("Closing sHost1:", err)
	}
	err = sHost2.Close()
	if err != nil {
		t.Fatal("Closing sHost2:", err)
	}
	if !<-doneListen {
		t.Fatal("Couldn't close")
	}
}

func genServerIdentity(name string) (abstract.Scalar, *ServerIdentity) {
	kp := config.NewKeyPair(Suite)
	return kp.Secret, &ServerIdentity{
		Public:    kp.Public,
		Addresses: []string{name},
	}
}
