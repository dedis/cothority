package prifi

import (
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/prifi/config"
	"github.com/dedis/cothority/protocols/prifi/crypto"
	"github.com/dedis/cothority/protocols/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
)

//Constants
const MaxUint uint32 = uint32(4294967295)
const socksHeaderLength = 6 // Number of bytes of cell payload to reserve for connection header, length
const WAIT_FOR_PUBLICKEY_SLEEP_TIME = 100 * time.Millisecond
const CLIENT_FAILED_CONNECTION_WAIT_BEFORE_RETRY = 1000 * time.Millisecond
const UDP_DATAGRAM_WAIT_TIMEOUT = 5 * time.Second

const (
	CLIENT_STATE_INITIALIZING int16 = iota
	CLIENT_STATE_EPH_KEYS_SENT
	CLIENT_STATE_READY
)

//State information to hold :

var clientState ClientState

type ClientState struct {
	CellCoder           dcnet.CellCoder
	ephemeralPrivateKey abstract.Secret
	EphemeralPublicKey  abstract.Point
	Id                  int
	LatencyTest         bool
	MessageHistory      abstract.Cipher
	Name                string
	nClients            int
	nTrustees           int
	PayloadLength       int
	privateKey          abstract.Secret //those are kept by the SDA stack
	PublicKey           abstract.Point  //those are kept by the SDA stack
	sharedSecrets       []abstract.Point
	TrusteePublicKey    []abstract.Point
	UsablePayloadLength int
	UseSocksProxy       bool
	UseUDP              bool
	MySlot              int
	currentState        int16
}

//dummy state, to be removed
var clientStateInt int32 = 0

/**
 * Used to initialize the state of this trustee. Must be called before anything else.
 */
func initClient(clientId int, nTrustees int, nClients int, payloadLength int, useSocksProxy bool, latencyTest bool, useUDP bool) *ClientState {

	params := new(ClientState)

	params.Name = "Client-" + strconv.Itoa(clientId)
	params.Id = clientId
	params.nClients = nClients
	params.nTrustees = nTrustees
	params.PayloadLength = payloadLength
	params.UseSocksProxy = useSocksProxy
	params.LatencyTest = latencyTest
	params.UseUDP = useUDP

	//prepare the crypto parameters
	rand := config.CryptoSuite.Cipher([]byte(params.Name))
	base := config.CryptoSuite.Point().Base()

	//generate own parameters
	params.privateKey = config.CryptoSuite.Secret().Pick(rand)                 //NO, this should be kept by SDA
	params.PublicKey = config.CryptoSuite.Point().Mul(base, params.privateKey) //NO, this should be kept by SDA

	//placeholders for pubkeys and secrets
	params.TrusteePublicKey = make([]abstract.Point, nTrustees)
	params.sharedSecrets = make([]abstract.Point, nTrustees)

	//sets the cell coder, and the history
	params.CellCoder = config.Factory()
	params.UsablePayloadLength = params.CellCoder.ClientCellSize(payloadLength)

	params.MySlot = -1
	params.currentState = CLIENT_STATE_INITIALIZING

	return params
}

//Messages to handle :
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK

func (p *PriFiProtocolHandlers) Received_REL_CLI_DOWNSTREAM_DATA_dummypingpong(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl2("I'm", p.Name())
	dbg.Lvl2("I received the REL_CLI_DOWNSTREAM_DATA with content", receivedNo)

	if clientStateInt == 0 {
		clientStateInt = int32(rand.Intn(10000))
		dbg.Lvl2("I'm", p.Name(), ", setting clientstate to ", clientStateInt)
	} else {
		dbg.Lvl2("I'm", p.Name(), ", keeping clientstate at ", clientStateInt)
	}

	toSend := &CLI_REL_UPSTREAM_DATA{clientStateInt, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("I'm", p.Name(), ", sending CLI_REL_UPSTREAM_DATA with clientState ", clientStateInt)

	return p.SendTo(p.Parent(), toSend)
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_DOWNSTREAM_DATA(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_TRUSTEES_PK(msg Struct_REL_CLI_TELL_TRUSTEES_PK) error {

	//this can only happens in the state TRUSTEE_STATE_SHUFFLE_DONE
	if clientState.currentState != CLIENT_STATE_INITIALIZING {
		e := "Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_TELL_TRUSTEES_PK, but not in state CLIENT_STATE_INITIALIZING, in state " + strconv.Itoa(int(clientState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(clientState.Id) + " : REL_CLI_TELL_TRUSTEES_PK")
	}

	//sanity check
	if len(msg.Pks) < 1 {
		e := "Client " + strconv.Itoa(clientState.Id) + " : len(msg.Pks) must be >= 1"
		dbg.Error(e)
		return errors.New(e)
	}

	//first, collect the public keys from the trustees, and derive the secrets
	clientState.nTrustees = len(msg.Pks)

	clientState.TrusteePublicKey = make([]abstract.Point, clientState.nTrustees)
	clientState.sharedSecrets = make([]abstract.Point, clientState.nTrustees)

	for i := 0; i < len(msg.Pks); i++ {
		clientState.TrusteePublicKey[i] = msg.Pks[i]
		clientState.sharedSecrets[i] = config.CryptoSuite.Point().Mul(msg.Pks[i], clientState.privateKey)
	}

	//then, generate our ephemeral keys (used for shuffling)
	clientState.generateEphemeralKeys()

	//send the keys to the relay
	toSend := &CLI_REL_TELL_PK_AND_EPH_PK{clientState.PublicKey, clientState.EphemeralPublicKey}
	err := p.SendTo(p.Parent(), toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send CLI_REL_TELL_PK_AND_EPH_PK, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Client " + strconv.Itoa(trusteeState.Id) + " : sent CLI_REL_TELL_PK_AND_EPH_PK")
	}

	//change state
	clientState.currentState = CLIENT_STATE_EPH_KEYS_SENT

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {

	//this can only happens in the state TRUSTEE_STATE_SHUFFLE_DONE
	if clientState.currentState != CLIENT_STATE_EPH_KEYS_SENT {
		e := "Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG, but not in state CLIENT_STATE_EPH_KEYS_SENT, in state " + strconv.Itoa(int(clientState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(clientState.Id) + " : REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG")
	}

	//verify the signature
	G := msg.Base
	ephPubKeys := msg.EphPks
	signatures := msg.TrusteesSigs

	G_bytes, _ := G.MarshalBinary()
	M := make([]byte, 0)
	M = append(M, G_bytes...)
	for k := 0; k < len(ephPubKeys); k++ {
		pkBytes, _ := ephPubKeys[k].MarshalBinary()
		M = append(M, pkBytes...)
	}

	for j := 0; j < clientState.nTrustees; j++ {
		err := crypto.SchnorrVerify(config.CryptoSuite, M, clientState.TrusteePublicKey[j], signatures[j])

		if err != nil {
			e := "Client " + strconv.Itoa(clientState.Id) + " : signature from trustee " + strconv.Itoa(j) + " is invalid "
			dbg.Error(e)
			return errors.New(e)
		}
	}

	dbg.Lvl3("Client " + strconv.Itoa(clientState.Id) + "; all signatures Ok")

	//now, using the ephemeral keys received (the output of the neff shuffle), identify our slot
	myPrivKey := clientState.ephemeralPrivateKey
	ephPubInBaseG := config.CryptoSuite.Point().Mul(G, myPrivKey)
	mySlot := -1

	for j := 0; j < len(ephPubKeys); j++ {
		if ephPubKeys[j].Equal(ephPubInBaseG) {
			mySlot = j
		}
	}

	if mySlot == -1 {
		e := "Client " + strconv.Itoa(clientState.Id) + "; Can't recognize our slot !"
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Client " + strconv.Itoa(clientState.Id) + "; Our slot is " + strconv.Itoa(mySlot) + " out of " + strconv.Itoa(len(ephPubKeys)) + " slots")
	}

	clientState.MySlot = mySlot

	//change state
	clientState.currentState = CLIENT_STATE_READY

	return nil
}

/**
 * Auxiliary function used by Received_REL_CLI_TELL_TRUSTEES_PK
 */
func (clientState *ClientState) generateEphemeralKeys() {

	//prepare the crypto parameters
	rand := config.CryptoSuite.Cipher([]byte(clientState.Name))
	base := config.CryptoSuite.Point().Base()

	//generate ephemeral keys
	Epriv := config.CryptoSuite.Secret().Pick(rand)
	Epub := config.CryptoSuite.Point().Mul(base, Epriv)

	clientState.EphemeralPublicKey = Epub
	clientState.ephemeralPrivateKey = Epriv

}
