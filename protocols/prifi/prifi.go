package prifi

/*
 * PRIFI SDA WRAPPER
 *
 * Caution : this is not the "PriFi protocol", which is really a "PriFi Library" which you need to import, and feed with some network methods.
 * This is the "PriFi-SDA-Wrapper" protocol, which imports the PriFi lib, gives it "SendToXXX()" methods and calls the "prifi_library.MessageReceived()"
 * methods (it build a map that converts the SDA tree into identities), and starts the PriFi Library.
 */

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	prifi_lib "github.com/dedis/cothority/lib/prifi"
	"github.com/dedis/cothority/lib/sda"
)

//this is the actual "PriFi" (DC-net) protocol/library
//defined in cothority/lib/prifi/prifi.go
var prifiProtocol *prifi_lib.PriFiProtocol

//the "PriFi-Wrapper-Protocol start". It calls the PriFi library with the correct parameters
func (p *PriFiSDAWrapper) Start() error {

	if !p.configSet {
		dbg.Error("Trying to start PriFi Library, but config not set !")
	}

	dbg.Lvl3("Starting PriFi-SDA-Wrapper Protocol")

	//initialize the first message (here the dummy ping-pong game)
	firstMessage := &prifi_lib.CLI_REL_UPSTREAM_DATA{100, make([]byte, 0)}
	firstMessageWrapper := Struct_CLI_REL_UPSTREAM_DATA{p.TreeNode(), *firstMessage}

	return p.Received_CLI_REL_UPSTREAM_DATA(firstMessageWrapper)
}

/**
 * On initialization of the PriFi-SDA-Wrapper protocol, it need to register the PriFi-Lib messages to be able to marshall them.
 * If we forget some messages there, it will crash when PriFi-Lib will call SendToXXX() with this message !
 */
func init() {

	//register the prifi_lib's message with the network lib here
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

	sda.ProtocolRegisterName("PriFi-SDA-Wrapper", NewPriFiSDAWrapperProtocol)
}

//This is the PriFi-SDA-Wrapper protocol struct. It contains the SDA-tree, and a chanel that stops the simulation when it receives a "true"
type PriFiSDAWrapper struct {
	*sda.TreeNodeInstance
	configSet   bool
	config      interface{}
	DoneChannel chan bool
}

func (p *PriFiSDAWrapper) SetConfig(config interface{}) {
	p.config = config
	p.configSet = false
	dbg.Lvl2("Setting PriFi config to be : ", config)
}

/**
 * This function is called on all nodes of the SDA-tree (when they receive their first prifi message).
 * It build a network map (deterministic from the order of the tree), which allows to build the
 * messageSender struct needed by PriFi-Lib.
 * Then, it instantiate PriFi-Lib with the correct state, given the role of the node.
 * Finally, it registers handlers so it can unmarshal messages and give them back to prifi. It is kind of ridiculous to have a handler for each
 * message, as PriFi-Lib is able to recognize the messages (everything is fed to ReceivedMessage() in PriFi-Lib), but that is how the SDA works
 * for now.
 */
func NewPriFiSDAWrapperProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	//fill in the network host map
	nodes := n.List()
	nodeRelay := nodes[0]
	nodesTrustee := make(map[int]*sda.TreeNode)
	nodesTrustee[0] = nodes[1]
	nodesClient := make(map[int]*sda.TreeNode)
	for i := 2; i < len(nodes); i++ {
		nodesClient[i-2] = nodes[i]
	}
	messageSender := MessageSender{n, nodeRelay, nodesTrustee, nodesClient}

	//parameters goes there
	nClients := 2
	nTrustees := 1
	upCellSize := 1000
	downCellSize := 10000
	relayWindowSize := 10
	relayUseDummyDataDown := false
	relayReportingLimit := -1
	useUDP := false
	doLatencyTests := true

	//first of all, instantiate our prifi library with the correct role, given our position in the tree
	switch n.Index() {
	case 0:
		dbg.Print(n.Name(), " starting as a PriFi relay")
		relayState := prifi_lib.NewRelayState(nTrustees, nClients, upCellSize, downCellSize, relayWindowSize, relayUseDummyDataDown, relayReportingLimit, useUDP)
		prifiProtocol = prifi_lib.NewPriFiRelay(messageSender, relayState)
	case 1:
		dbg.Print(n.Name(), " starting as PriFi trustee 0")
		trusteeId := 0
		trusteeState := prifi_lib.NewTrusteeState(trusteeId, nTrustees, nClients, upCellSize)
		prifiProtocol = prifi_lib.NewPriFiTrustee(messageSender, trusteeState)
	default:
		clientId := (n.Index() - 2)
		dbg.Print(n.Name(), " starting as a PriFi client", clientId)
		clientState := prifi_lib.NewClientState(clientId, nTrustees, nClients, upCellSize, doLatencyTests, useUDP)
		prifiProtocol = prifi_lib.NewPriFiClient(messageSender, clientState)
	}

	//instantiate our PriFi wrapper protocol
	prifiSDAWrapperHandlers := &PriFiSDAWrapper{
		TreeNodeInstance: n,
		DoneChannel:      make(chan bool),
	}

	//register handlers
	err := prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_ALL_ALL_PARAMETERS)
	if err != nil {
		return nil, errors.New("couldn't register handler: " + err.Error())
	}

	//register client handlers
	err = prifiSDAWrapperHandlers.RegisterHandler(prifiSDAWrapperHandlers.Received_REL_CLI_DOWNSTREAM_DATA)
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
