package prifi

import (
	"errors"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(DataUp{})
	network.RegisterMessageType(DataDown{})
	sda.ProtocolRegisterName("PriFi", NewPriFiProtocol)
}

// ProtocolExampleHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PriFiProtocolHandlers struct {
	*sda.TreeNodeInstance
	Message    string
	ChildCount chan int
}

func (p *PriFiProtocolHandlers) Start() error {

	dbg.Print("Starting PriFiProtocolHandlers")

	return p.HandleDataUp(StructDataUp{p.TreeNode(), DataUp{100}})
}

// NewExampleHandlers initialises the structure for use in one round
func NewPriFiProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	ExampleHandlers := &PriFiProtocolHandlers{
		TreeNodeInstance: n,
		ChildCount:       make(chan int),
	}
	err := ExampleHandlers.RegisterHandler(ExampleHandlers.HandleDataUp)
	if err != nil {
		return nil, errors.New("couldn't register dataup-handler: " + err.Error())
	}
	err = ExampleHandlers.RegisterHandler(ExampleHandlers.HandleDataDown)
	if err != nil {
		return nil, errors.New("couldn't register datadown-handler: " + err.Error())
	}
	return ExampleHandlers, nil
}

func (p *PriFiProtocolHandlers) HandleDataUp(msg StructDataUp) error {

	receivedNo := msg.Data

	if receivedNo == 110 {
		//SWITCH PROTOCOL
		dbg.Print("COMMUNICATE: I'd like to switch to protocol SETUP")
	}

	dbg.Lvl1("COMMUNICATE: I'm", p.Name())
	dbg.Lvl1("COMMUNICATE: I received the DataUp with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	toSend := &DataDown{receivedNo + 1}

	for _, c := range p.Children() {
		dbg.Lvl1("COMMUNICATE: I'm", p.Name(), ", sending DataDown to ", c.Entity.Public)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PriFiProtocolHandlers) HandleDataDown(msg StructDataDown) error {

	receivedNo := msg.Data

	dbg.Lvl2("COMMUNICATE: I'm", p.Name())
	dbg.Lvl2("COMMUNICATE: I received the DataDown with content", receivedNo)

	toSend := &DataUp{receivedNo + 1}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("COMMUNICATE: I'm", p.Entity().Public, ", sending DataUp to ", p.Parent().Entity.String)

	return p.SendTo(p.Parent(), toSend)
}
