package sda

import (
	"testing"

	"github.com/dedis/cothority/network"
)

// Test complete parsing of new incoming packet
// - Test if it is SDAMessage
// - reject if unknown ProtocolID
// - setting up of graph and Hostlist
// - instantiating ProtocolInstance

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

var SimpleMessageType = network.RegisterMessageType(SimpleMessage{})

type simpleMessageProc struct {
	t     *testing.T
	relay chan SimpleMessage
}

func newSimpleMessageProc(t *testing.T) *simpleMessageProc {
	return &simpleMessageProc{
		t:     t,
		relay: make(chan SimpleMessage),
	}
}

func (smp *simpleMessageProc) Process(p *network.Packet) {
	if p.MsgType != SimpleMessageType {
		smp.t.Fatal("Wrong message")
	}
	sm := p.Msg.(SimpleMessage)
	smp.relay <- sm
}
