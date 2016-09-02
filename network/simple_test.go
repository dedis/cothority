package network

import (
	"golang.org/x/net/context"
	"fmt"
	"sync"
	"testing"
)

var SimplePacketType PacketTypeID

func init() {
	SimplePacketType = RegisterPacketType(SimplePacket{})
}

type SimplePacket struct {
	Name string
}

func TestTCPConnListenerExample(t *testing.T) {
	addr := NewTCPAddress("127.0.0.1:2000")
	server, err := NewTCPListener(addr)
	if err != nil {
		t.Fatal("Could not setup listener")
	}
	serverName := "server"
	clientName := "client"

	done := make(chan bool)
	listenCB := make(chan bool)

	srvConMu := sync.Mutex{}
	cConMu := sync.Mutex{}
	go func() {
		err := server.Listen(func(c Conn) {
			listenCB <- true
			srvConMu.Lock()
			defer srvConMu.Unlock()
			nm, _ := c.Receive(context.TODO())
			if nm.MsgType != SimplePacketType {
				c.Close()
				panic("Packet received not conform")
			}
			simplePacket := nm.Msg.(SimplePacket)
			if simplePacket.Name != clientName {
				panic("Not the right name")
			}
			c.Send(context.TODO(), &SimplePacket{serverName})
			//c.Close()
		})
		if err != nil {
			panic("Couldn't listen:" + err.Error())
		}
		done <- true
	}()
	cConMu.Lock()
	conn, err := NewTCPConn(addr)
	if err != nil {
		panic(err)
	}
	// wait for the listen callback to be called at least once:
	<-listenCB

	conn.Send(context.TODO(), &SimplePacket{clientName})
	nm, err := conn.Receive(context.TODO())
	cConMu.Unlock()
	if err != nil {
		panic(err)
	}
	if nm.MsgType != SimplePacketType {
		panic(fmt.Sprintf("Packet received non conform %+v", nm))
	}
	sp := nm.Msg.(SimplePacket)
	if sp.Name != serverName {
		panic("Name no right")
	}
	cConMu.Lock()
	if err := conn.Close(); err != nil {
		panic("Couldn't close client connection")
	}
	cConMu.Unlock()

	srvConMu.Lock()
	defer srvConMu.Unlock()
	if err := server.Stop(); err != nil {
		panic("Couldn't close server connection")
	}
	<-done
	// Output
	// Client received server
	// Server received client
}
