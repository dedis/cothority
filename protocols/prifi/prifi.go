package prifi

import (
	"errors"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(CLI_REL_TELL_PK_AND_EPH_PK{})
	network.RegisterMessageType(CLI_REL_UPSTREAM_DATA{})
	network.RegisterMessageType(REL_CLI_DOWNSTREAM_DATA{})
	network.RegisterMessageType(REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG{})
	network.RegisterMessageType(REL_CLI_TELL_TRUSTEES_PK{})
	network.RegisterMessageType(REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{})
	network.RegisterMessageType(REL_TRU_TELL_TRANSCRIPT{})
	network.RegisterMessageType(TRU_REL_DC_CIPHER{})
	network.RegisterMessageType(TRU_REL_SHUFFLE_SIG{})
	network.RegisterMessageType(TRU_REL_TELL_NEW_BASE_AND_EPH_PKS{})
	network.RegisterMessageType(TRU_REL_TELL_PK{})
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

	firstMessage := &CLI_REL_UPSTREAM_DATA{100, make([]byte, 0)}

	return p.HandleDataUp(Struct_CLI_REL_UPSTREAM_DATA{p.TreeNode(), *firstMessage})
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

func (p *PriFiProtocolHandlers) HandleDataUp(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	receivedNo := msg.RoundId

	if receivedNo == 110 {
		//SWITCH PROTOCOL
		dbg.Print("COMMUNICATE: I'd like to switch to protocol SETUP")
	}

	dbg.Lvl1("COMMUNICATE: I'm", p.Name())
	dbg.Lvl1("COMMUNICATE: I received the DataUp with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	toSend := &REL_CLI_DOWNSTREAM_DATA{receivedNo + 1, make([]byte, 0)}

	for _, c := range p.Children() {
		dbg.Lvl1("COMMUNICATE: I'm", p.Name(), ", sending DataDown to ", c.Entity.Public)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PriFiProtocolHandlers) HandleDataDown(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl2("COMMUNICATE: I'm", p.Name())
	dbg.Lvl2("COMMUNICATE: I received the DataDown with content", receivedNo)

	toSend := &CLI_REL_UPSTREAM_DATA{receivedNo + 1, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("COMMUNICATE: I'm", p.Entity().Public, ", sending DataUp to ", p.Parent().Entity.String)

	return p.SendTo(p.Parent(), toSend)
}
