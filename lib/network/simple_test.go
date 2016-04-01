package network

import (
	"fmt"
	"sync"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"golang.org/x/net/context"
)

var SimplePacketType MessageTypeID

func init() {
	SimplePacketType = RegisterMessageType(SimplePacket{})
}

type SimplePacket struct {
	Name string
}

func TestSimple(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	client := NewTCPHost()
	clientName := "client"
	server := NewTCPHost()
	serverName := "server"

	done := make(chan bool)
	listenCB := make(chan bool)

	srvConMu := sync.Mutex{}
	cConMu := sync.Mutex{}

	go func() {
		err := server.Listen("localhost:2000", func(c Conn) {
			listenCB <- true
			srvConMu.Lock()
			defer srvConMu.Unlock()
			nm, _ := c.Receive(context.TODO())
			if nm.MsgType != SimplePacketType {
				c.Close()
				t.Fatal("Packet received not conform")
			}
			simplePacket := nm.Msg.(SimplePacket)
			if simplePacket.Name != clientName {
				t.Fatal("Not the right name")
			}
			c.Send(context.TODO(), &SimplePacket{serverName})
			//c.Close()
		})
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		close(done)
	}()
	cConMu.Lock()
	conn, err := client.Open("localhost:2000")
	if err != nil {
		t.Fatal(err)
	}
	// wait for the listen callback to be called at least once:
	<-listenCB

	conn.Send(context.TODO(), &SimplePacket{clientName})
	nm, err := conn.Receive(context.TODO())
	cConMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if nm.MsgType != SimplePacketType {
		t.Fatal(fmt.Sprintf("Packet received non conform %+v", nm))
	}
	sp := nm.Msg.(SimplePacket)
	if sp.Name != serverName {
		t.Fatal("Name no right")
	}
	cConMu.Lock()
	if err := client.Close(); err != nil {
		t.Fatal("Couldn't close client connection")
	}
	cConMu.Unlock()

	srvConMu.Lock()
	defer srvConMu.Unlock()
	if err := server.Close(); err != nil {
		t.Fatal("Couldn't close server connection")
	}
	<-done
}
