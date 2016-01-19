package network

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"testing"
	"time"
)

var SimplePacketType uuid.UUID

func init() {
	SimplePacketType = RegisterMessageType(SimplePacket{})
}

type SimplePacket struct {
	Name string
}

func TestSimple(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	client := NewTcpHost()
	clientName := "client"
	server := NewTcpHost()
	serverName := "server"
	done := make(chan bool)
	go func() {
		err := server.Listen("localhost:2000", func(c Conn) {
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
			c.Close()
		})
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		close(done)
	}()
	time.Sleep(1 * time.Second)
	conn, err := client.Open("localhost:2000")
	if err != nil {
		t.Fatal(err)
	}
	conn.Send(context.TODO(), &SimplePacket{clientName})
	nm, err := conn.Receive(context.TODO())
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
	client.Close()
	server.Close()
	<-done
}
