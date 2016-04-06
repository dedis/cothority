package sda

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
)

type Processor interface {
	Process(msg *network.Message)
}

type ProtocolProcessor interface {
	Process(msg *Data)
}

type ProtocolProcessorHandler struct {
}

func NewProtocolProcessorHandler() *ProtocolProcessorHandler {
	return &ProtocolProcessorHandler{}
}

func (pph *ProtocolProcessorHandler) Process(msg *Data) {

	dbg.Lvl1("Not implemented")
}

func (pph *ProtocolProcessorHandler) RegisterHandler(c interface{}) error {

	dbg.Lvl1("Not implemented")
	return nil
}

// ProtocolProcessorChannel will process messages using channels
type ProtocolProcessorChannel struct {
}

func (pph *ProtocolProcessorChannel) Process(msg *Data) {
	dbg.Lvl1("Not implemented")
}

func (pph *ProtocolProcessorChannel) RegisterChannel(c interface{}) error {

	dbg.Lvl1("Not implemented")
	return nil
}

func NewProtocolProcessorChannel() *ProtocolProcessorChannel {
	return &ProtocolProcessorChannel{}
}
