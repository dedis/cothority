package network

import (
	"fmt"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

// Secure_test is analog to simple_test it uses the same structure to send
// The difference lies in which host and connections it uses. here we are going
// to use SecureTcpHost and SecureConn with Entities
// Now you connect to someone else using Entity instead of directly addresses

func TestSecureSimple(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	priv1, id1 := genEntity("localhost:2000")
	priv2, id2 := genEntity("localhost:2001")
	sHost1 := NewSecureTCPHost(priv1, id1)
	sHost2 := NewSecureTCPHost(priv2, id2)

	packetToSend := SimplePacket{"HelloWorld"}
	done := make(chan error)
	doneListen := make(chan bool)
	go func() {
		err := sHost1.Listen(func(c SecureConn) {
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
			if !nm.Entity.Equal(id2) {
				c.Close()
				done <- fmt.Errorf("Not same entity")
			}
			dbg.Lvl3("Connection accepted")
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

func genEntity(name string) (abstract.Secret, *Entity) {
	kp := config.NewKeyPair(Suite)
	return kp.Secret, &Entity{
		Public:    kp.Public,
		Addresses: []string{name},
	}
}
