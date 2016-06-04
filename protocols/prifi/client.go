package prifi

import (
	"math/rand"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
)

//Constants
const MaxUint uint32 = uint32(4294967295)
const socksHeaderLength = 6 // Number of bytes of cell payload to reserve for connection header, length
const WAIT_FOR_PUBLICKEY_SLEEP_TIME = 100 * time.Millisecond
const CLIENT_FAILED_CONNECTION_WAIT_BEFORE_RETRY = 1000 * time.Millisecond
const UDP_DATAGRAM_WAIT_TIMEOUT = 5 * time.Second

//State information to hold :
type ClientState struct {
	Id   int
	Name string

	//PublicKey			abstract.Point  //those are kept by the SDA stack
	//privateKey		abstract.Secret  //those are kept by the SDA stack

	EphemeralPublicKey  abstract.Point
	ephemeralPrivateKey abstract.Secret

	nClients  int
	nTrustees int

	PayloadLength       int
	UsablePayloadLength int
	UseSocksProxy       bool
	LatencyTest         bool
	UseUDP              bool

	TrusteePublicKey []abstract.Point
	sharedSecrets    []abstract.Point

	CellCoder dcnet.CellCoder

	MessageHistory abstract.Cipher
}

//dummy state, to be removed
var clientState int32 = 0

//Messages to handle :
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK

func (p *PriFiProtocolHandlers) Received_REL_CLI_DOWNSTREAM_DATA(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl2("I'm", p.Name())
	dbg.Lvl2("I received the REL_CLI_DOWNSTREAM_DATA with content", receivedNo)

	if clientState == 0 {
		clientState = int32(rand.Intn(10000))
		dbg.Lvl2("I'm", p.Name(), ", setting clientstate to ", clientState)
	} else {
		dbg.Lvl2("I'm", p.Name(), ", keeping clientstate at ", clientState)
	}

	toSend := &CLI_REL_UPSTREAM_DATA{clientState, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("I'm", p.Name(), ", sending CLI_REL_UPSTREAM_DATA with clientState ", clientState)

	return p.SendTo(p.Parent(), toSend)
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_TRUSTEES_PK(msg Struct_REL_CLI_TELL_TRUSTEES_PK) error {

	return nil
}
