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
	sda.ProtocolRegisterName("Communicate", NewCommunicateProtocol)
}

// ProtocolExampleHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type CommunicateProtocolHandlers struct {
	*sda.TreeNodeInstance
	Message    string
	ChildCount chan int
}

// NewExampleHandlers initialises the structure for use in one round
func NewCommunicateProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	ExampleHandlers := &CommunicateProtocolHandlers{
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

func (p *CommunicateProtocolHandlers) HandleDataUp(msg StructDataUp) error {

	receivedNo := msg.Data
	dbg.Lvl1("I'm", p.Entity().Public, ", I received the DataUp with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	toSend := &DataDown{receivedNo + 1}

	for _, c := range p.Children() {
		var a int = 2
		var b int = 3
		dbg.Lvl1("I'm", p.Entity().Public, ", sending DataDown to ", c.Entity().Public)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}
}

func (p *CommunicateProtocolHandlers) HandleDataDown(msg StructDataDown) error {

	receivedNo := msg.Data
	dbg.Lvl1("I'm", p.Entity().Public, ", I received the DataDown with content", receivedNo)

	toSend := &DataUp{receivedNo + 1}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl1("I'm", p.Entity().Public, ", sending DataUp to ", p.Parent().Entity().String)

	return p.SendTo(p.Parent(), toSend)
}

func (p *CommunicateProtocolHandlers) Start() error {

	dbg.Lvl3("Starting ExampleHandlers")

	return p.HandleDataUp(StructDataUp{p.TreeNode(), DataUp{100}})
}
