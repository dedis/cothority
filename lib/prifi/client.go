package prifi

import (
	"encoding/binary"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/prifi/config"
	"github.com/dedis/cothority/lib/prifi/crypto"
	"github.com/dedis/cothority/lib/prifi/dcnet"
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
	DataForDCNet        chan []byte //VPN / SOCKS should put data there !
	DataFromDCNet       chan []byte //VPN / SOCKS should read data from there !
	DataOutputEnabled   bool        //if FALSE, nothing will be written to DataFromDCNet
	roundCount          int32       //modulo number of clients, used only to test if "isMySlot"
}

//dummy state, to be removed
var clientStateInt int32 = 0

/**
 * Used to initialize the state of this trustee. Must be called before anything else.
 */
func NewClientState(clientId int, nTrustees int, nClients int, payloadLength int, latencyTest bool, useUDP bool) *ClientState {

	params := new(ClientState)

	params.Name = "Client-" + strconv.Itoa(clientId)
	params.Id = clientId
	params.nClients = nClients
	params.nTrustees = nTrustees
	params.PayloadLength = payloadLength
	params.UseSocksProxy = false //deprecated
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
	params.roundCount = 0

	return params
}

//Messages to handle :
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK

func (p *PriFiProtocol) Received_REL_CLI_DOWNSTREAM_DATA_dummypingpong(msg REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	//dbg.Lvl2("I'm", p.Name())
	dbg.Lvl2("I received the REL_CLI_DOWNSTREAM_DATA with content", receivedNo)

	if clientStateInt == 0 {
		clientStateInt = int32(rand.Intn(10000))
		dbg.Lvl2("setting clientstate to ", clientStateInt)
	} else {
		dbg.Lvl2("keeping clientstate at ", clientStateInt)
	}

	toSend := &CLI_REL_UPSTREAM_DATA{clientStateInt, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("sending CLI_REL_UPSTREAM_DATA with clientState ", clientStateInt)

	dbg.Print(p)
	dbg.Print(p.messageSender)
	return p.messageSender.SendToRelay(toSend)
}

func (p *PriFiProtocol) Received_REL_CLI_DOWNSTREAM_DATA(msg REL_CLI_DOWNSTREAM_DATA) error {

	//this can only happens in the state TRUSTEE_STATE_SHUFFLE_DONE
	if clientState.currentState != CLIENT_STATE_READY {
		e := "Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_DOWNSTREAM_DATA, but not in state CLIENT_STATE_READY, in state " + strconv.Itoa(int(clientState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl5("Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_DOWNSTREAM_DATA")
	}

	/*
	 * HANDLE THE DOWNSTREAM DATA
	 */

	//pass the data to the VPN/SOCKS5 proxy, if enabled
	if clientState.DataOutputEnabled {
		clientState.DataFromDCNet <- msg.Data //TODO : this should be encrypted, and we need to check if it's our data
	}

	//write the next upstream slice. First, determine if we can embed payload this round
	currentRound := clientState.roundCount % int32(clientState.nClients)
	isMySlot := false
	if currentRound == int32(clientState.MySlot) {
		isMySlot = true
	}
	//test if it is the answer from our ping (for latency test)
	if clientState.LatencyTest && len(msg.Data) > 2 {
		pattern := int(binary.BigEndian.Uint16(msg.Data[0:2]))
		if pattern == 43690 { //1010101010101010
			clientId := int(binary.BigEndian.Uint16(msg.Data[2:4]))
			if clientId == clientState.Id {
				timestamp := int64(binary.BigEndian.Uint64(msg.Data[4:12]))
				diff := MsTimeStamp() - timestamp

				dbg.Lvl1("Client " + strconv.Itoa(clientState.Id) + " : New latency measured " + strconv.FormatInt(diff, 10))
			}
		}
	}
	//if the flag "Resync" is on, we cannot write data up, but need to resend the keys instead
	if msg.FlagResync == true {

		dbg.Lvl1("Client " + strconv.Itoa(clientState.Id) + " : Relay wants to resync, going to state CLIENT_STATE_INITIALIZING ")
		clientState.currentState = CLIENT_STATE_INITIALIZING

		//TODO : regenerate ephemeral keys ?

		return nil
	}

	/*
	 * PRODUCE THE UPSTREAM DATA
	 */

	var upstreamCellContent []byte

	//if we can...
	if isMySlot {
		select {

		//either select data from the data we have to send, if any
		case upstreamCellContent = <-clientState.DataForDCNet:

		//or, if we have nothing to send, and we are doing Latency tests, embed a pre-crafted message that we will recognize later on
		default:
			if clientState.LatencyTest {

				if clientState.PayloadLength < 12 {
					panic("Trying to do a Latency test, but payload is smaller than 10 bytes.")
				}

				buffer := make([]byte, clientState.PayloadLength)
				pattern := uint16(43690)  //1010101010101010
				currTime := MsTimeStamp() //timestamp in Ms

				binary.BigEndian.PutUint16(buffer[0:2], pattern)
				binary.BigEndian.PutUint16(buffer[2:4], uint16(clientState.Id))
				binary.BigEndian.PutUint64(buffer[4:12], uint64(currTime))

				upstreamCellContent = buffer
			}
		}
	}

	//produce the next upstream cell
	upstreamCell := clientState.CellCoder.ClientEncode(upstreamCellContent, clientState.PayloadLength, clientState.MessageHistory)

	//send the data to the relay
	toSend := &CLI_REL_UPSTREAM_DATA{clientState.roundCount, upstreamCell}
	err := p.messageSender.SendToRelay(toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send CLI_REL_UPSTREAM_DATA, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Client " + strconv.Itoa(trusteeState.Id) + " : sent CLI_REL_UPSTREAM_DATA for round " + strconv.Itoa(int(clientState.roundCount)))
	}

	//one round just passed
	clientState.roundCount++

	return nil
}

func (p *PriFiProtocol) Received_REL_CLI_TELL_TRUSTEES_PK(msg REL_CLI_TELL_TRUSTEES_PK) error {

	//this can only happens in the state TRUSTEE_STATE_SHUFFLE_DONE
	if clientState.currentState != CLIENT_STATE_INITIALIZING {
		e := "Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_TELL_TRUSTEES_PK, but not in state CLIENT_STATE_INITIALIZING, in state " + strconv.Itoa(int(clientState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Client " + strconv.Itoa(clientState.Id) + " : Received a REL_CLI_TELL_TRUSTEES_PK")
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
	err := p.messageSender.SendToRelay(toSend) //TODO : this should be the root ! make sure of it
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

func (p *PriFiProtocol) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {

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

	//prepare for commmunication
	clientState.MySlot = mySlot
	clientState.roundCount = 0

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

/**
 * Auxiliary function that returns the current timestamp, in miliseconds
 */
func MsTimeStamp() int64 {
	//http://stackoverflow.com/questions/24122821/go-golang-time-now-unixnano-convert-to-milliseconds
	return time.Now().UnixNano() / int64(time.Millisecond)
}
