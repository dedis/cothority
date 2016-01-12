package network

import (
	"fmt"
	"golang.org/x/net/context"
	"testing"
	"time"
)

func init() {
	RegisterProtocolType(SimplePacketType, SimplePacket{})
}

type SimplePacket struct {
	Name string
}

const (
	SimplePacketType = iota + 70
)

func TestSimple(t *testing.T) {
	client := NewTcpHost()
	clientName := "client"
	server := NewTcpHost()
	serverName := "server"
	go server.Listen("127.0.0.1:2000", func(c Conn) {
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
	time.Sleep(1 * time.Second)
	conn, err := client.Open("127.0.0.1:2000")
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
}
