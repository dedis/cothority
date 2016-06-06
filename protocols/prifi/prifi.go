package prifi

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	prifi_lib "github.com/dedis/cothority/lib/prifi"
	"github.com/dedis/cothority/lib/sda"
)

//defined in cothority/lib/prifi/prifi.go
var prifiProtocol *prifi_lib.PriFiProtocol

func init() {

	messageSender := MessageSender{}

	//first of all, instantiate our prifi library
	currentNode := "relay"
	switch currentNode {
	case "relay":
		prifiProtocol = prifi_lib.NewPriFiRelay(messageSender)
	case "client":
		prifiProtocol = prifi_lib.NewPriFiClient(0, messageSender)
	case "trustee":
		prifiProtocol = prifi_lib.NewPriFiTrustee(0, messageSender)
	}

	//then, register the prifi_lib's message with the network lib here
	network.RegisterMessageType(prifi_lib.CLI_REL_TELL_PK_AND_EPH_PK{})
	network.RegisterMessageType(prifi_lib.CLI_REL_UPSTREAM_DATA{})
	network.RegisterMessageType(prifi_lib.REL_CLI_DOWNSTREAM_DATA{})
	network.RegisterMessageType(prifi_lib.REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG{})
	network.RegisterMessageType(prifi_lib.REL_CLI_TELL_TRUSTEES_PK{})
	network.RegisterMessageType(prifi_lib.REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{})
	network.RegisterMessageType(prifi_lib.REL_TRU_TELL_TRANSCRIPT{})
	network.RegisterMessageType(prifi_lib.TRU_REL_DC_CIPHER{})
	network.RegisterMessageType(prifi_lib.TRU_REL_SHUFFLE_SIG{})
	network.RegisterMessageType(prifi_lib.TRU_REL_TELL_NEW_BASE_AND_EPH_PKS{})
	network.RegisterMessageType(prifi_lib.TRU_REL_TELL_PK{})

	sda.ProtocolRegisterName("PriFi", NewPriFiProtocol)
}

// ProtocolExampleHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PriFiSDAWrapper struct {
	*sda.TreeNodeInstance
	Message    string
	ChildCount chan int
}

type MessageSender struct {
}

func (ms MessageSender) SendToClient(i int, msg interface{}) error {
	dbg.Lvl1("Sending a message to client ", i, " - ", msg)

	return nil
}

func (ms MessageSender) SendToTrustee(i int, msg interface{}) error {

	dbg.Lvl1("Sending a message to trustee ", i, " - ", msg)

	return nil
}

func (ms MessageSender) SendToRelay(msg interface{}) error {

	dbg.Lvl1("Sending a message to relay ", " - ", msg)

	return nil
}

func (p *PriFiSDAWrapper) Start() error {

	dbg.Print("Starting PriFi")

	firstMessage := &prifi_lib.CLI_REL_UPSTREAM_DATA{100, make([]byte, 0)}
	firstMessageWrapper := Struct_CLI_REL_UPSTREAM_DATA{p.TreeNode(), *firstMessage}

	return p.Received_CLI_REL_UPSTREAM_DATA(firstMessageWrapper)
}

// NewExampleHandlers initialises the structure for use in one round
func NewPriFiProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	prifiSDAWrapperHandlers := &PriFiSDAWrapper{
		TreeNodeInstance: n,
		ChildCount:       make(chan int),
	}

	//register client handlers
	err := prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_CLI_DOWNSTREAM_DATA)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_CLI_TELL_TRUSTEES_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}

	//register relay handlers
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_CLI_REL_TELL_PK_AND_EPH_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_CLI_REL_UPSTREAM_DATA)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_TRU_REL_DC_CIPHER)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_TRU_REL_SHUFFLE_SIG)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_TRU_REL_TELL_PK)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}

	//register trustees handlers
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_TRU_TELL_TRANSCRIPT)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}
	return prifiSDAWrapperHandlers, nil
}
