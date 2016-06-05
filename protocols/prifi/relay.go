package prifi

import (
	"errors"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
	"github.com/lbarman/prifi/config"
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
	Id                 int
	Connected          bool
	PublicKey          abstract.Point
	EphemeralPublicKey abstract.Point
}

type NeffShuffleState struct {
	ClientPublicKeys  []abstract.Point
	G_s               []abstract.Point
	ephPubKeys_s      [][]abstract.Point
	proof_s           [][]byte
	nextFreeId_Proofs int
	signatures_s      [][]byte
	signature_count   int
}

var relayState RelayState

//State information to hold :
type RelayState struct {
	//RelayPort				string
	//PublicKey				abstract.Point
	//privateKey			abstract.Secret
	//trusteesHosts			[]string

	Name                     string
	nClients                 int
	nTrustees                int
	UseUDP                   bool
	UseDummyDataDown         bool
	UDPBroadcastConn         net.Conn
	privateKey               abstract.Secret //those are kept by the SDA stack
	PublicKey                abstract.Point  //those are kept by the SDA stack
	clients                  []NodeRepresentation
	trustees                 []NodeRepresentation
	CellCoder                dcnet.CellCoder
	MessageHistory           abstract.Cipher
	UpstreamCellSize         int
	DownstreamCellSize       int
	WindowSize               int
	ReportingLimit           int
	currentShuffleTranscript NeffShuffleState
}

func initRelay(nTrustees int, nClients int, upstreamCellSize int, downstreamCellSize int, windowSize int, useDummyDataDown bool, reportingLimit int, useUDP bool) *RelayState {
	params := new(RelayState)

	params.Name = "Relay"
	params.UpstreamCellSize = upstreamCellSize
	params.DownstreamCellSize = downstreamCellSize
	params.WindowSize = windowSize
	params.ReportingLimit = reportingLimit
	params.UseUDP = useUDP
	params.UseDummyDataDown = useDummyDataDown

	//prepare the crypto parameters
	rand := config.CryptoSuite.Cipher([]byte(params.Name))
	base := config.CryptoSuite.Point().Base()

	//generate own parameters
	params.privateKey = config.CryptoSuite.Secret().Pick(rand)
	params.PublicKey = config.CryptoSuite.Point().Mul(base, params.privateKey)

	params.nClients = nClients
	params.nTrustees = nTrustees

	//sets the cell coder, and the history
	params.CellCoder = config.Factory()

	return params
}

//dummy state, to be removed
var relayStateInt int32 = 0

//Messages to handle :
//CLI_REL_TELL_PK_AND_EPH_PK
//CLI_REL_UPSTREAM_DATA
//TRU_REL_DC_CIPHER
//TRU_REL_SHUFFLE_SIG
//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
//TRU_REL_TELL_PK

func (p *PriFiProtocolHandlers) Received_CLI_REL_UPSTREAM_DATA_dummypingpong(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl1("I'm", p.Name())
	dbg.Lvl1("I received the CLI_REL_UPSTREAM_DATA with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	if relayStateInt == 0 {
		dbg.Print(rand.Intn(10000))
		dbg.Print(rand.Intn(10000))
		relayStateInt = int32(rand.Intn(10000))
		dbg.Lvl1("I'm", p.Name(), ", setting relaystate to ", relayStateInt)
	} else {
		dbg.Lvl1("I'm", p.Name(), ", keeping relaystate at ", relayStateInt)
	}

	toSend := &REL_CLI_DOWNSTREAM_DATA{relayStateInt, make([]byte, 0), false}

	for _, c := range p.Children() {
		dbg.Lvl1("I'm", p.Name(), ", sending REL_CLI_DOWNSTREAM_DATA with relayState ", relayStateInt)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

/*
 * DC-Net communication operation
 */

func (p *PriFiProtocolHandlers) Received_CLI_REL_UPSTREAM_DATA(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_DC_CIPHER(msg Struct_TRU_REL_DC_CIPHER) error {

	return nil
}

/*
 * PriFi Setup
 */

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_PK(msg Struct_TRU_REL_TELL_PK) error {

	//Note : is this still needed ? I don't think so; maybe if the trustees also have an ephemeral key ?
	return nil
}

func (p *PriFiProtocolHandlers) Received_CLI_REL_TELL_PK_AND_EPH_PK(msg Struct_CLI_REL_TELL_PK_AND_EPH_PK) error {

	//collect this client information
	nextId := len(relayState.clients)
	newClient := NodeRepresentation{nextId, true, msg.Pk, msg.EphPk}

	relayState.clients = append(relayState.clients, newClient)

	//TODO : sanity check that we don't have twice the same client

	dbg.Lvl3("Relay : Received a CLI_REL_TELL_PK_AND_EPH_PK, registered client ID" + strconv.Itoa(nextId))

	//if we have collected all clients, continue
	if len(relayState.clients) == relayState.nClients {

		//prepare the arrays; pack the public keys and ephemeral public keys
		pks := make([]abstract.Point, relayState.nClients)
		ephPks := make([]abstract.Point, relayState.nClients)
		for i := 0; i < relayState.nClients; i++ {
			pks[i] = relayState.clients[i].PublicKey
			ephPks[i] = relayState.clients[i].EphemeralPublicKey
		}
		G := config.CryptoSuite.Point().Base()

		//prepare the empty shuffle
		emptyG_s := make([]abstract.Point, relayState.nTrustees)
		emptyEphPks_s := make([][]abstract.Point, relayState.nTrustees)
		emptyProof_s := make([][]byte, relayState.nTrustees)
		emptySignature_s := make([][]byte, relayState.nTrustees)
		relayState.currentShuffleTranscript = NeffShuffleState{pks, emptyG_s, emptyEphPks_s, emptyProof_s, 0, emptySignature_s, 0}

		//send to the 1st trustee
		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.SendTo(p.Parent(), toSend) //TODO : this should be the trustee X !
		if err != nil {
			e := "Could not send REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (0-th iteration), error is " + err.Error()
			dbg.Error(e)
			return errors.New(e)
		} else {
			dbg.Lvl3("Relay : sent REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (0-th iteration)")
		}

		//should change state here
	}

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(msg Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS) error {

	//store this shuffle's result in our transcript
	j := relayState.currentShuffleTranscript.nextFreeId_Proofs
	relayState.currentShuffleTranscript.G_s[j] = msg.NewBase
	relayState.currentShuffleTranscript.ephPubKeys_s[j] = msg.NewEphPks
	relayState.currentShuffleTranscript.proof_s[j] = msg.Proof

	relayState.currentShuffleTranscript.nextFreeId_Proofs = j + 1

	//if we're still waiting on some trustees, send them the new shuffle
	if relayState.currentShuffleTranscript.nextFreeId_Proofs != relayState.nTrustees {

		pks := relayState.currentShuffleTranscript.ClientPublicKeys
		ephPks := msg.NewEphPks
		G := msg.NewBase

		//send to the i-th trustee
		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.SendTo(p.Parent(), toSend) //TODO : this should be the trustee X !
		if err != nil {
			e := "Could not send REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (" + strconv.Itoa(j+1) + "-th iteration), error is " + err.Error()
			dbg.Error(e)
			return errors.New(e)
		} else {
			dbg.Lvl3("Relay : sent REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (" + strconv.Itoa(j+1) + "-th iteration)")
		}

	} else {
		//if we have all the shuffles

		//pack transcript
		G_s := relayState.currentShuffleTranscript.G_s
		ephPublicKeys_s := relayState.currentShuffleTranscript.ephPubKeys_s
		proof_s := relayState.currentShuffleTranscript.proof_s

		//broadcast to all trustees
		for j := 0; j < relayState.nTrustees; j++ {
			//send to the j-th trustee
			toSend := &REL_TRU_TELL_TRANSCRIPT{G_s, ephPublicKeys_s, proof_s}
			err := p.SendTo(p.Parent(), toSend) //TODO : this should be the trustee X !
			if err != nil {
				e := "Could not send REL_TRU_TELL_TRANSCRIPT to " + strconv.Itoa(j+1) + "-th trustee, error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_TRU_TELL_TRANSCRIPT to " + strconv.Itoa(j+1) + "-th trustee")
			}
		}

		//change state
	}

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_SHUFFLE_SIG(msg Struct_TRU_REL_SHUFFLE_SIG) error {

	//sanity check
	if msg.TrusteeId < 0 || msg.TrusteeId > len(relayState.currentShuffleTranscript.signatures_s) {
		e := "Relay : One of the following check failed : msg.TrusteeId >= 0 && msg.TrusteeId < len(relayState.currentShuffleTranscript.signatures_s) ; msg.TrusteeId = " + strconv.Itoa(trusteeState.Id) + ";"
		dbg.Error(e)
		return errors.New(e)
	}

	//store this shuffle's signature in our transcript
	relayState.currentShuffleTranscript.signatures_s[msg.TrusteeId] = msg.Sig
	relayState.currentShuffleTranscript.signature_count++

	//if we have all the signatures
	if relayState.currentShuffleTranscript.signature_count == relayState.nTrustees {

		//We could verify here before broadcasting to the clients, for performance (but this does not add security)

		//prepare the message for the clients
		lastPermutationIndex := relayState.nTrustees - 1
		G := relayState.currentShuffleTranscript.G_s[lastPermutationIndex]
		ephPks := relayState.currentShuffleTranscript.ephPubKeys_s[lastPermutationIndex]
		signatures := relayState.currentShuffleTranscript.signatures_s

		//broadcast to all clients
		for i := 0; i < relayState.nClients; i++ {
			//send to the i-th client
			toSend := &REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG{G, ephPks, signatures}
			err := p.SendTo(p.Parent(), toSend) //TODO : this should be the client X !
			if err != nil {
				e := "Could not send REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG to " + strconv.Itoa(i+1) + "-th client, error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG to " + strconv.Itoa(i+1) + "-th client")
			}
		}
	}

	return nil
}
