package network

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/assert"
)

type basicProcessor struct {
	msgChan chan Packet
}

func (bp *basicProcessor) Process(msg *Packet) {
	bp.msgChan <- *msg
}

type basicMessage struct {
	Value int
}

var basicMessageType = RegisterPacketType(&basicMessage{})

func TestBlockingDispatcher(t *testing.T) {
	defer log.AfterTest(t)

	dispatcher := NewBlockingDispatcher()
	processor := &basicProcessor{make(chan Packet, 1)}

	dispatcher.RegisterProcessor(processor, basicMessageType)
	dispatcher.Dispatch(&Packet{
		Msg:     basicMessage{10},
		MsgType: basicMessageType})

	select {
	case m := <-processor.msgChan:
		msg, ok := m.Msg.(basicMessage)
		assert.True(t, ok)
		assert.Equal(t, msg.Value, 10)
	default:
		t.Error("No message received")
	}
}
