package prifi

import (
	"errors"

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

	dbg.Print("Starting PriFi")

	firstMessage := &CLI_REL_UPSTREAM_DATA{100, make([]byte, 0)}
	firstMessageWrapper := Struct_CLI_REL_UPSTREAM_DATA{p.TreeNode(), *firstMessage}

	return p.Received_CLI_REL_UPSTREAM_DATA_dummypingpong(firstMessageWrapper)
}

// NewExampleHandlers initialises the structure for use in one round
func NewPriFiProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	PriFiHandlers := &PriFiProtocolHandlers{
		TreeNodeInstance: n,
		ChildCount:       make(chan int),
	}

	//register client handlers
	err := PriFiHandlers.RegisterHandler(PriFiHandlers.Received_REL_CLI_DOWNSTREAM_DATA_dummypingpong) //TODO : switch with actual protocol
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_REL_CLI_TELL_TRUSTEES_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}

	//register relay handlers
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_CLI_REL_TELL_PK_AND_EPH_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_CLI_REL_UPSTREAM_DATA_dummypingpong) //TODO : switch with actual protocol
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_TRU_REL_DC_CIPHER)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_TRU_REL_SHUFFLE_SIG)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_TRU_REL_TELL_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}

	//register trustees handlers
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = PriFiHandlers.RegisterHandler(PriFiHandlers.Received_REL_TRU_TELL_TRANSCRIPT)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	return PriFiHandlers, nil
}
