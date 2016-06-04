package prifi

import (
	"math/rand"
	"net"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
)

//Constants
const CONTROL_LOOP_SLEEP_TIME = 1 * time.Second
const PROCESSING_LOOP_SLEEP_TIME = 0 * time.Second
const INBETWEEN_CONFIG_SLEEP_TIME = 0 * time.Second
const NEWCLIENT_CHECK_SLEEP_TIME = 10 * time.Millisecond
const CLIENT_READ_TIMEOUT = 5 * time.Second
const RELAY_FAILED_CONNECTION_WAIT_BEFORE_RETRY = 10 * time.Second
const (
	PROTOCOL_STATUS_OK = iota
	PROTOCOL_STATUS_GONNA_RESYNC
	PROTOCOL_STATUS_RESYNCING
)

type NodeRepresentation struct {
	Id        int
	Conn      net.Conn //classical TCP connection
	Connected bool
	PublicKey abstract.Point
}

//State information to hold :
type RelayState struct {
	//RelayPort				string
	//PublicKey				abstract.Point
	//privateKey			abstract.Secret
	//trusteesHosts			[]string

	Name               string
	nClients           int
	nTrustees          int
	UseUDP             bool
	UseDummyDataDown   bool
	UDPBroadcastConn   net.Conn
	clients            []NodeRepresentation
	trustees           []NodeRepresentation
	CellCoder          dcnet.CellCoder
	MessageHistory     abstract.Cipher
	UpstreamCellSize   int
	DownstreamCellSize int
	WindowSize         int
	ReportingLimit     int
}

//dummy state, to be removed
var relayState int32 = 0

//Messages to handle :
//CLI_REL_TELL_PK_AND_EPH_PK
//CLI_REL_UPSTREAM_DATA
//TRU_REL_DC_CIPHER
//TRU_REL_SHUFFLE_SIG
//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
//TRU_REL_TELL_PK

func (p *PriFiProtocolHandlers) Received_CLI_REL_TELL_PK_AND_EPH_PK(msg Struct_CLI_REL_TELL_PK_AND_EPH_PK) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_CLI_REL_UPSTREAM_DATA_dummypingpong(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl1("I'm", p.Name())
	dbg.Lvl1("I received the CLI_REL_UPSTREAM_DATA with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	if relayState == 0 {
		dbg.Print(rand.Intn(10000))
		dbg.Print(rand.Intn(10000))
		relayState = int32(rand.Intn(10000))
		dbg.Lvl1("I'm", p.Name(), ", setting relaystate to ", relayState)
	} else {
		dbg.Lvl1("I'm", p.Name(), ", keeping relaystate at ", relayState)
	}

	toSend := &REL_CLI_DOWNSTREAM_DATA{relayState, make([]byte, 0)}

	for _, c := range p.Children() {
		dbg.Lvl1("I'm", p.Name(), ", sending REL_CLI_DOWNSTREAM_DATA with relayState ", relayState)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PriFiProtocolHandlers) Received_CLI_REL_UPSTREAM_DATA(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_DC_CIPHER(msg Struct_TRU_REL_DC_CIPHER) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_SHUFFLE_SIG(msg Struct_TRU_REL_SHUFFLE_SIG) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(msg Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_PK(msg Struct_TRU_REL_TELL_PK) error {

	return nil
}
