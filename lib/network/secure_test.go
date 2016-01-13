package network

import (
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
	"testing"
	"time"
)

// Secure_test is analog to simple_test it uses the same structure to send
// The difference lies in which host and connections it uses. here we are going
// to use SecureTcpHost and SecureConn with Entities
// Now you connect to someone else using Entity instead of directly addresses

func TestSecureSimple(t *testing.T) {
	priv1, id1 := genEntity("localhost:2000")
	priv2, id2 := genEntity("localhost:2001")
	sHost1 := NewSecureTcpHost(priv1, id1)
	sHost2 := NewSecureTcpHost(priv2, id2)

	packetToSend := SimplePacket{"HelloWorld"}
	done := make(chan error)
	go sHost1.Listen(func(c SecureConn) {
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
			done <- fmt.Errorf("Not same identity")
		}
		close(done)
	})
	time.Sleep(1 * time.Second)
	// Open connection to identity1
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
	sHost1.Close()
	sHost2.Close()
}

func genEntity(name string) (abstract.Secret, *Entity) {
	kp := cliutils.KeyPair(tSuite)
	return kp.Secret, &Entity{
		Public:    kp.Public,
		Addresses: []string{name},
	}
}
