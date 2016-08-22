package network

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

var SimplePacketType MessageTypeID

func init() {
	SimplePacketType = RegisterMessageType(SimplePacket{})
}

type SimplePacket struct {
	Name string
}

func TestTCPConnListenerExample(t *testing.T) {
	server := NewTCPListener()
	addr := Address("tcp://127.0.0.1:2000")
	serverName := "server"
	clientName := "client"

	done := make(chan bool)
	listenCB := make(chan bool)

	srvConMu := sync.Mutex{}
	cConMu := sync.Mutex{}
	go func() {
		err := server.Listen(addr, func(c Conn) {
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
		close(done)
	}()
	cConMu.Lock()
	conn, err := NewTCPConn(addr.NetworkAddress())
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
