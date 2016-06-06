package prifi

import (
	"errors"
	"strconv"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	prifi_lib "github.com/dedis/cothority/lib/prifi"
	"github.com/dedis/cothority/lib/sda"
)

//defined in cothority/lib/prifi/prifi.go
var prifiProtocol *prifi_lib.PriFiProtocol

//the "PriFi-Wrapper-Protocol start". It calls the PriFi library with the correct parameters
func (p *PriFiSDAWrapper) Start() error {

	dbg.Print("Starting PriFi Library")

	//initialize the first message (here the dummy ping-pong game)
	firstMessage := &prifi_lib.CLI_REL_UPSTREAM_DATA{100, make([]byte, 0)}
	firstMessageWrapper := Struct_CLI_REL_UPSTREAM_DATA{p.TreeNode(), *firstMessage}

	return p.Received_CLI_REL_UPSTREAM_DATA(firstMessageWrapper)
}

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

	sda.ProtocolRegisterName("PriFi", NewPriFiProtocol)
}

// ProtocolExampleHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PriFiSDAWrapper struct {
	*sda.TreeNodeInstance
	ChildCount chan int
}

type MessageSender struct {
	tree     *sda.TreeNodeInstance
	relay    *sda.TreeNode
	clients  map[int]*sda.TreeNode
	trustees map[int]*sda.TreeNode
}

func (ms MessageSender) SendToClient(i int, msg interface{}) error {
	dbg.Lvl1("Sending a message to client ", i, " - ", msg)

	if client, ok := ms.clients[i]; ok {
		return ms.tree.SendTo(client, msg)
	} else {
		panic("Client " + strconv.Itoa(i) + " is unknown !")
	}

	return nil
}

func (ms MessageSender) SendToTrustee(i int, msg interface{}) error {

	dbg.Lvl1("Sending a message to trustee ", i, " - ", msg)

	if trustee, ok := ms.trustees[i]; ok {
		return ms.tree.SendTo(trustee, msg)
	} else {
		panic("Trustee " + strconv.Itoa(i) + " is unknown !")
	}

	return nil
}

func (ms MessageSender) SendToRelay(msg interface{}) error {
	dbg.Lvl1("Sending a message to relay ", " - ", msg)
	return ms.tree.SendTo(ms.relay, msg)
}

// NewExampleHandlers initialises the structure for use in one round
func NewPriFiProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

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
