package prifi

import (
	"encoding/binary"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/prifi/dcnet"
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
	RELAY_STATE_BEFORE_INIT int16 = iota
	RELAY_STATE_COLLECTING_TRUSTEES_PKS
	RELAY_STATE_COLLECTING_CLIENT_PKS
	RELAY_STATE_COLLECTING_SHUFFLES
	RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES
	RELAY_STATE_COMMUNICATING
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

type DCNetRound struct {
	currentRound       int32
	trusteeCipherCount int
	clientCipherCount  int
}

func (p *PriFiProtocol) hasAllCiphers(dcnet *DCNetRound) bool {
	if p.relayState.nClients == dcnet.clientCipherCount && p.relayState.nTrustees == dcnet.trusteeCipherCount {
		return true
	}
	return false
}

type BufferedTrusteeCipher struct {
	RoundId int32
	Data    map[int][]byte
}

//State information to hold :
type RelayState struct {
	//RelayPort				string
	//PublicKey				abstract.Point
	//privateKey			abstract.Secret
	//trusteesHosts			[]string

	bufferedTrusteeCiphers   map[int32]BufferedTrusteeCipher
	CellCoder                dcnet.CellCoder
	clients                  []NodeRepresentation
	currentDCNetRound        DCNetRound
	currentShuffleTranscript NeffShuffleState
	currentState             int16
	DataForClients           chan []byte //VPN / SOCKS should put data there !
	DataFromDCNet            chan []byte //VPN / SOCKS should read data from there !
	DataOutputEnabled        bool        //if FALSE, nothing will be written to DataFromDCNet
	DownstreamCellSize       int
	MessageHistory           abstract.Cipher
	Name                     string
	nClients                 int
	nTrustees                int
	privateKey               abstract.Secret //those are kept by the SDA stack
	PublicKey                abstract.Point  //those are kept by the SDA stack
	ReportingLimit           int
	trustees                 []NodeRepresentation
	nTrusteesPkCollected     int
	UpstreamCellSize         int
	UseDummyDataDown         bool
	UseUDP                   bool
	WindowSize               int
}

func NewRelayState(nTrustees int, nClients int, upstreamCellSize int, downstreamCellSize int, windowSize int, useDummyDataDown bool, reportingLimit int, useUDP bool, dataOutputEnabled bool) *RelayState {
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

	params.clients = make([]NodeRepresentation, 0)
	params.trustees = make([]NodeRepresentation, nTrustees)
	params.nTrusteesPkCollected = 0

	//sets the cell coder, and the history
	params.CellCoder = config.Factory()

	params.DataForClients = make(chan []byte)
	params.DataFromDCNet = make(chan []byte)
	params.DataOutputEnabled = dataOutputEnabled

	params.currentState = RELAY_STATE_COLLECTING_TRUSTEES_PKS

	return params
}

//dummy state, to be removed
var relayStateInt int32 = 0

//Messages to handle :
//ALL_ALL_PARAMETERS
//CLI_REL_TELL_PK_AND_EPH_PK
//CLI_REL_UPSTREAM_DATA
//TRU_REL_DC_CIPHER
//TRU_REL_SHUFFLE_SIG
//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
//TRU_REL_TELL_PK

/**
 * This is the "INIT" message that shares all the public parameters.
 */
func (p *PriFiProtocol) Received_ALL_REL_PARAMETERS(msg ALL_ALL_PARAMETERS) error {

	//this can only happens in the state RELAY_STATE_BEFORE_INIT
	if p.relayState.currentState != RELAY_STATE_BEFORE_INIT && !msg.ForceParams {
		dbg.Lvl1("Relay : Received a ALL_ALL_PARAMETERS, but not in state RELAY_STATE_BEFORE_INIT, ignoring. ")
		return nil
	} else if p.relayState.currentState != RELAY_STATE_BEFORE_INIT && msg.ForceParams {
		dbg.Lvl1("Relay : Received a ALL_ALL_PARAMETERS && ForceParams = true, processing. ")
	} else {
		dbg.Lvl3("Relay : received ALL_ALL_PARAMETERS")
	}

	if p.role != PRIFI_ROLE_RELAY {
		panic("This message wants me to become a relay ! I'm not one !")
	}

	p.relayState = *NewRelayState(msg.NTrustees, msg.NClients, msg.UpCellSize, msg.DownCellSize, msg.RelayWindowSize, msg.RelayUseDummyDataDown, msg.RelayReportingLimit, msg.UseUDP, msg.RelayDataOutputEnabled)

	dbg.Lvlf5("%+v\n", p.relayState)
	dbg.Lvl1("Relay has been initialized by message. ")

	dbg.Print(p.messageSender)
	dbg.Print(p.relayState.nClients)
	dbg.Print(p.relayState.nTrustees)

	//broadcast those parameters
	if msg.StartNow {

		p.relayState.currentState = RELAY_STATE_COLLECTING_TRUSTEES_PKS
		p.ConnectToTrustees()
	}
	dbg.Lvl1("Done")

	return nil
}

func (p *PriFiProtocol) ConnectToTrustees() {

	var msg = &ALL_ALL_PARAMETERS{
		NClients:          p.relayState.nClients,
		NextFreeTrusteeId: 0,
		NTrustees:         p.relayState.nTrustees,
		StartNow:          true,
		ForceParams:       true,
		UpCellSize:        p.relayState.UpstreamCellSize,
	}

	for j := 0; j < p.relayState.nTrustees; j++ {
		dbg.Lvl1("Sending to trustee", j)
		msg.NextFreeTrusteeId = j
		p.messageSender.SendToTrustee(j, msg)
	}
}

func (p *PriFiProtocol) Received_CLI_REL_UPSTREAM_DATA_dummypingpong(msg CLI_REL_UPSTREAM_DATA) error {

	receivedNo := msg.RoundId

	//dbg.Lvl1("I'm", p.Name())
	dbg.Lvl1("I received the CLI_REL_UPSTREAM_DATA with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	if relayStateInt == 0 {
		dbg.Print(rand.Intn(10000))
		dbg.Print(rand.Intn(10000))
		relayStateInt = int32(rand.Intn(10000))
		dbg.Lvl1("setting relaystate to ", relayStateInt)
	} else {
		dbg.Lvl1("keeping relaystate at ", relayStateInt)
	}

	toSend := &REL_CLI_DOWNSTREAM_DATA{relayStateInt, make([]byte, 0), false}

	dbg.Lvl1("sending REL_CLI_DOWNSTREAM_DATA with relayState ", relayStateInt)
	err := p.messageSender.SendToClient(0, toSend)
	if err != nil {
		return err
	}

	return nil
}

/*
 * DC-Net communication operation
 */

func (p *PriFiProtocol) Received_CLI_REL_UPSTREAM_DATA(msg CLI_REL_UPSTREAM_DATA) error {

	//this can only happens in the state RELAY_STATE_COMMUNICATING
	if p.relayState.currentState != RELAY_STATE_COMMUNICATING {
		e := "Relay : Received a CLI_REL_UPSTREAM_DATA, but not in state RELAY_STATE_COMMUNICATING, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received CLI_REL_UPSTREAM_DATA")
	}

	//TODO : add a timeout somewhere here

	//if this is not the message destinated for this round, discard it ! (we are in lock-step)
	if p.relayState.currentDCNetRound.currentRound != msg.RoundId {
		e := "Relay : Client sent DC-net cipher for round , " + strconv.Itoa(int(msg.RoundId)) + " but current round is " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound))
		dbg.Error(e)
		return errors.New(e)

	} else {
		//else, if this is the message we need for this round

		p.relayState.CellCoder.DecodeClient(msg.Data)
		p.relayState.currentDCNetRound.clientCipherCount++

		if p.hasAllCiphers(&p.relayState.currentDCNetRound) {
			p.finalizeUpstreamDataAndSendDownstreamData()
		}
	}

	return nil
}

func (p *PriFiProtocol) Received_TRU_REL_DC_CIPHER(msg TRU_REL_DC_CIPHER) error {

	//this can only happens in the state RELAY_STATE_COMMUNICATING
	if p.relayState.currentState != RELAY_STATE_COMMUNICATING {
		e := "Relay : Received a CLI_REL_UPSTREAM_DATA, but not in state RELAY_STATE_COMMUNICATING, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received CLI_REL_UPSTREAM_DATA")
	}

	//TODO : add rate-control somewhere here
	//TODO : add a timeout somewhere here

	//if this is the message we need for this round
	if p.relayState.currentDCNetRound.currentRound == msg.RoundId {
		p.relayState.CellCoder.DecodeTrustee(msg.Data)
		p.relayState.currentDCNetRound.trusteeCipherCount++

		if p.hasAllCiphers(&p.relayState.currentDCNetRound) {
			p.finalizeUpstreamDataAndSendDownstreamData()
		}
	} else {
		//else, we need to buffer this message somewhere
		if _, ok := p.relayState.bufferedTrusteeCiphers[msg.RoundId]; ok {
			//the roundId already exists, simply add data
			p.relayState.bufferedTrusteeCiphers[msg.RoundId].Data[msg.TrusteeId] = msg.Data
		} else {
			//else, create the key
			newKey := BufferedTrusteeCipher{msg.RoundId, make(map[int][]byte)}
			newKey.Data[msg.TrusteeId] = msg.Data
			p.relayState.bufferedTrusteeCiphers[msg.RoundId] = newKey
		}
	}
	return nil
}

func (p *PriFiProtocol) finalizeUpstreamDataAndSendDownstreamData() error {

	/*
	 * Finish processing the upstream data
	 */

	//we decode the DC-net cell
	upstreamPlaintext := p.relayState.CellCoder.DecodeCell()

	// Process the decoded cell

	//check if we have a latency test message
	if len(upstreamPlaintext) >= 2 {
		pattern := int(binary.BigEndian.Uint16(upstreamPlaintext[0:2]))
		if pattern == 43690 { //1010101010101010
			//then, we simply have to send it down
			p.relayState.DataForClients <- upstreamPlaintext
		}
	}

	if upstreamPlaintext == nil {
		// empty upstream cell
	}

	if len(upstreamPlaintext) != p.relayState.UpstreamCellSize {
		e := "Relay : DecodeCell produced wrong-size payload, " + strconv.Itoa(len(upstreamPlaintext)) + "!=" + strconv.Itoa(p.relayState.UpstreamCellSize)
		dbg.Error(e)
		return errors.New(e)
	}

	if p.relayState.DataOutputEnabled {
		p.relayState.DataFromDCNet <- upstreamPlaintext
	}

	/*
	 * Process the downstream data
	 */

	var downstreamCellContent []byte

	select {

	//either select data from the data we have to send, if any
	case downstreamCellContent = <-p.relayState.DataForClients:

	default:
		downstreamCellContent = make([]byte, 1)
	}

	//if we want to use dummy data down, pad to the correct size
	if p.relayState.UseDummyDataDown && len(downstreamCellContent) < p.relayState.DownstreamCellSize {
		data := make([]byte, p.relayState.DownstreamCellSize)
		copy(data[0:], downstreamCellContent)
		downstreamCellContent = data
	}

	flagResync := false

	if !p.relayState.UseUDP {
		//broadcast to all clients
		for i := 0; i < p.relayState.nClients; i++ {
			//send to the i-th client
			toSend := &REL_CLI_DOWNSTREAM_DATA{p.relayState.currentDCNetRound.currentRound, downstreamCellContent, flagResync}
			err := p.messageSender.SendToClient(i, toSend) //TODO : this should be the client X !
			if err != nil {
				e := "Could not send REL_CLI_DOWNSTREAM_DATA to " + strconv.Itoa(i+1) + "-th client for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)) + ", error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl5("Relay : sent REL_CLI_DOWNSTREAM_DATA to " + strconv.Itoa(i+1) + "-th client for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)))
			}
		}
	} else {
		panic("UDP not supported yet")
	}

	//prepare for the next round
	nextRound := p.relayState.currentDCNetRound.currentRound + 1
	p.relayState.currentDCNetRound = DCNetRound{nextRound, 0, 0}
	p.relayState.CellCoder.DecodeStart(p.relayState.UpstreamCellSize, p.relayState.MessageHistory)

	//if we have buffered messages for next round, use them now
	if _, ok := p.relayState.bufferedTrusteeCiphers[nextRound]; ok {
		for trusteeId, data := range p.relayState.bufferedTrusteeCiphers[nextRound].Data {
			//start decoding using this data
			dbg.Lvl5("Relay : using pre-cached DC-net cipher from trustee " + strconv.Itoa(trusteeId) + " for round " + strconv.Itoa(int(nextRound)))
			p.relayState.CellCoder.DecodeTrustee(data)
			p.relayState.currentDCNetRound.trusteeCipherCount++
		}
		delete(p.relayState.bufferedTrusteeCiphers, nextRound)
	}

	return nil
}

/*
 * PriFi Setup
 */

func (p *PriFiProtocol) Received_TRU_REL_TELL_PK(msg TRU_REL_TELL_PK) error {

	//this can only happens in the state RELAY_STATE_COLLECTING_TRUSTEES_PKS
	if p.relayState.currentState != RELAY_STATE_COLLECTING_TRUSTEES_PKS {
		e := "Relay : Received a TRU_REL_TELL_PK, but not in state RELAY_STATE_COLLECTING_TRUSTEES_PKS, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received TRU_REL_TELL_PK")
	}

	p.relayState.trustees[msg.TrusteeId] = NodeRepresentation{msg.TrusteeId, true, msg.Pk, msg.Pk}
	p.relayState.nTrusteesPkCollected++

	//if we have them all...
	if p.relayState.nTrusteesPkCollected == p.relayState.nTrustees {

		//prepare the message for the clients
		trusteesPk := make([]abstract.Point, p.relayState.nTrustees)
		for i := 0; i < p.relayState.nTrustees; i++ {
			trusteesPk[i] = p.relayState.trustees[i].PublicKey
		}

		//Send the pack to the clients
		toSend := &REL_CLI_TELL_TRUSTEES_PK{trusteesPk}
		for i := 0; i < p.relayState.nClients; i++ {
			err := p.messageSender.SendToClient(i, toSend)
			if err != nil {
				e := "Could not send REL_CLI_TELL_TRUSTEES_PK (" + strconv.Itoa(i) + "-th client), error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_CLI_TELL_TRUSTEES_PK (" + strconv.Itoa(i) + "-th client)")
			}
		}

		p.relayState.currentState = RELAY_STATE_COLLECTING_CLIENT_PKS
	}

	return nil
}

func (p *PriFiProtocol) Received_CLI_REL_TELL_PK_AND_EPH_PK(msg CLI_REL_TELL_PK_AND_EPH_PK) error {

	//this can only happens in the state RELAY_STATE_COLLECTING_CLIENT_PKS
	if p.relayState.currentState != RELAY_STATE_COLLECTING_CLIENT_PKS {
		e := "Relay : Received a CLI_REL_TELL_PK_AND_EPH_PK, but not in state RELAY_STATE_COLLECTING_CLIENT_PKS, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received CLI_REL_TELL_PK_AND_EPH_PK")
	}

	//collect this client information
	nextId := len(p.relayState.clients)
	newClient := NodeRepresentation{nextId, true, msg.Pk, msg.EphPk}

	p.relayState.clients = append(p.relayState.clients, newClient)

	//TODO : sanity check that we don't have twice the same client

	dbg.Lvl3("Relay : Received a CLI_REL_TELL_PK_AND_EPH_PK, registered client ID" + strconv.Itoa(nextId))

	//if we have collected all clients, continue
	if len(p.relayState.clients) == p.relayState.nClients {

		//prepare the arrays; pack the public keys and ephemeral public keys
		pks := make([]abstract.Point, p.relayState.nClients)
		ephPks := make([]abstract.Point, p.relayState.nClients)
		for i := 0; i < p.relayState.nClients; i++ {
			pks[i] = p.relayState.clients[i].PublicKey
			ephPks[i] = p.relayState.clients[i].EphemeralPublicKey
		}

		G := p.relayState.clients[0].PublicKey
		//G := config.CryptoSuite.Point() // LUDOVIC BARMAN- HERE IS THE PROBLEM

		//prepare the empty shuffle
		emptyG_s := make([]abstract.Point, p.relayState.nTrustees)
		emptyEphPks_s := make([][]abstract.Point, p.relayState.nTrustees)
		emptyProof_s := make([][]byte, p.relayState.nTrustees)
		emptySignature_s := make([][]byte, p.relayState.nTrustees)
		p.relayState.currentShuffleTranscript = NeffShuffleState{pks, emptyG_s, emptyEphPks_s, emptyProof_s, 0, emptySignature_s, 0}

		//send to the 1st trustee

		dbg.Print("Relay sending REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE iteration 0")
		dbg.Print(pks)
		dbg.Print(ephPks)
		dbg.Print(G)
		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.messageSender.SendToTrustee(0, toSend) //TODO : this should be the trustee X !
		if err != nil {
			e := "Could not send REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (0-th iteration), error is " + err.Error()
			dbg.Error(e)
			return errors.New(e)
		} else {
			dbg.Lvl3("Relay : sent REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE (0-th iteration)")
		}

		//changing state
		p.relayState.currentState = RELAY_STATE_COLLECTING_SHUFFLES
	}

	return nil
}

func (p *PriFiProtocol) Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(msg TRU_REL_TELL_NEW_BASE_AND_EPH_PKS) error {

	//this can only happens in the state RELAY_STATE_COLLECTING_SHUFFLES
	if p.relayState.currentState != RELAY_STATE_COLLECTING_SHUFFLES {
		e := "Relay : Received a TRU_REL_TELL_NEW_BASE_AND_EPH_PKS, but not in state RELAY_STATE_COLLECTING_SHUFFLES, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received TRU_REL_TELL_NEW_BASE_AND_EPH_PKS")
	}

	//store this shuffle's result in our transcript
	j := p.relayState.currentShuffleTranscript.nextFreeId_Proofs
	p.relayState.currentShuffleTranscript.G_s[j] = msg.NewBase
	p.relayState.currentShuffleTranscript.ephPubKeys_s[j] = msg.NewEphPks
	p.relayState.currentShuffleTranscript.proof_s[j] = msg.Proof

	p.relayState.currentShuffleTranscript.nextFreeId_Proofs = j + 1

	//if we're still waiting on some trustees, send them the new shuffle
	if p.relayState.currentShuffleTranscript.nextFreeId_Proofs != p.relayState.nTrustees {

		pks := p.relayState.currentShuffleTranscript.ClientPublicKeys
		ephPks := msg.NewEphPks
		G := msg.NewBase

		//send to the i-th trustee

		dbg.Print("Relay sending REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE iteration " + strconv.Itoa(j+1))
		dbg.Print(pks)
		dbg.Print(ephPks)
		dbg.Print(G)

		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.messageSender.SendToTrustee(j+1, toSend) //TODO : this should be the trustee X !
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
		G_s := p.relayState.currentShuffleTranscript.G_s
		ephPublicKeys_s := p.relayState.currentShuffleTranscript.ephPubKeys_s
		proof_s := p.relayState.currentShuffleTranscript.proof_s

		//when receiving the next message (and after processing it), trustees will start sending data. Prepare to buffer it
		p.relayState.bufferedTrusteeCiphers = make(map[int32]BufferedTrusteeCipher)

		//broadcast to all trustees
		for j := 0; j < p.relayState.nTrustees; j++ {
			//send to the j-th trustee
			toSend := &REL_TRU_TELL_TRANSCRIPT{G_s, ephPublicKeys_s, proof_s}
			err := p.messageSender.SendToTrustee(j, toSend) //TODO : this should be the trustee X !
			if err != nil {
				e := "Could not send REL_TRU_TELL_TRANSCRIPT to " + strconv.Itoa(j+1) + "-th trustee, error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_TRU_TELL_TRANSCRIPT to " + strconv.Itoa(j+1) + "-th trustee")
			}
		}

		//changing state
		p.relayState.currentState = RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES
	}

	return nil
}

func (p *PriFiProtocol) Received_TRU_REL_SHUFFLE_SIG(msg TRU_REL_SHUFFLE_SIG) error {

	//this can only happens in the state RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES
	if p.relayState.currentState != RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES {
		e := "Relay : Received a TRU_REL_SHUFFLE_SIG, but not in state RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received TRU_REL_SHUFFLE_SIG")
	}

	//sanity check
	if msg.TrusteeId < 0 || msg.TrusteeId > len(p.relayState.currentShuffleTranscript.signatures_s) {
		e := "Relay : One of the following check failed : msg.TrusteeId >= 0 && msg.TrusteeId < len(p.relayState.currentShuffleTranscript.signatures_s) ; msg.TrusteeId = " + strconv.Itoa(msg.TrusteeId) + ";"
		dbg.Error(e)
		return errors.New(e)
	}

	//store this shuffle's signature in our transcript
	p.relayState.currentShuffleTranscript.signatures_s[msg.TrusteeId] = msg.Sig
	p.relayState.currentShuffleTranscript.signature_count++

	//if we have all the signatures
	if p.relayState.currentShuffleTranscript.signature_count == p.relayState.nTrustees {

		//We could verify here before broadcasting to the clients, for performance (but this does not add security)

		//prepare the message for the clients
		lastPermutationIndex := p.relayState.nTrustees - 1
		G := p.relayState.currentShuffleTranscript.G_s[lastPermutationIndex]
		ephPks := p.relayState.currentShuffleTranscript.ephPubKeys_s[lastPermutationIndex]
		signatures := p.relayState.currentShuffleTranscript.signatures_s

		//broadcast to all clients
		for i := 0; i < p.relayState.nClients; i++ {
			//send to the i-th client
			toSend := &REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG{G, ephPks, signatures}
			err := p.messageSender.SendToClient(i, toSend) //TODO : this should be the client X !
			if err != nil {
				e := "Could not send REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG to " + strconv.Itoa(i+1) + "-th client, error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG to " + strconv.Itoa(i+1) + "-th client")
			}
		}

		//prepare to collect the ciphers
		p.relayState.currentDCNetRound = DCNetRound{0, 0, 0}
		p.relayState.CellCoder.DecodeStart(p.relayState.UpstreamCellSize, p.relayState.MessageHistory)

		//changing state
		p.relayState.currentState = RELAY_STATE_COMMUNICATING
	}

	return nil
}
