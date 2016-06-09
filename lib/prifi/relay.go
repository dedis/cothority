package prifi

/**
 * PriFi Relay
 * ************
 * This regroups the behavior of the PriFi relay.
 * Needs to be instantiated via the PriFiProtocol in prifi.go
 * Then, this file simple handle the answer to the different message kind :
 *
 * - ALL_ALL_PARAMETERS (specialized into ALL_REL_PARAMETERS) - used to initialize the relay over the network / overwrite its configuration
 * - TRU_REL_TELL_PK - when a trustee connects, he tells us his public key
 * - CLI_REL_TELL_PK_AND_EPH_PK - when they receive the list of the trustees, each clients tells his identity. when we have all client's IDs, we send them to the trustees to shuffle (Schedule protocol)
 * - TRU_REL_TELL_NEW_BASE_AND_EPH_PKS - when we receive the result of one shuffle, we forward it to the next trustee
 * - TRU_REL_SHUFFLE_SIG - when the shuffle has been done by all trustee, we send the transcript, and they answer with a signature, which we broadcast to the clients
 * - CLI_REL_UPSTREAM_DATA - data for the DC-net
 * - TRU_REL_DC_CIPHER - data for the DC-net
 *
 * TODO : We should timeout if some client did not send anything after a while
 * TODO : given the number of already-buffered Ciphers (per trustee), we need to tell him to slow down
 * TODO : if something went wrong before, this flag should be used to warn the clients that the config has changed
 * TODO : sanity check that we don't have twice the same client
 */

import (
	"encoding/binary"
	"errors"
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

// possible state the trustees are in. This restrict the kind of messages they can receive at a given point
const (
	RELAY_STATE_BEFORE_INIT int16 = iota
	RELAY_STATE_COLLECTING_TRUSTEES_PKS
	RELAY_STATE_COLLECTING_CLIENT_PKS
	RELAY_STATE_COLLECTING_SHUFFLES
	RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES
	RELAY_STATE_COMMUNICATING
)

//this regroups the information about one client/trustee
type NodeRepresentation struct {
	Id                 int
	Connected          bool
	PublicKey          abstract.Point
	EphemeralPublicKey abstract.Point
}

//this is where the Neff Shuffles are accumulated during the Schedule protocol
type NeffShuffleState struct {
	ClientPublicKeys  []abstract.Point
	G_s               []abstract.Point
	ephPubKeys_s      [][]abstract.Point
	proof_s           [][]byte
	nextFreeId_Proofs int
	signatures_s      [][]byte
	signature_count   int
}

//is counts how many (upstream) messages we received for a given DC-net round
type DCNetRound struct {
	currentRound       int32
	trusteeCipherCount int
	clientCipherCount  int
}

//test if we received all DC-net ciphers (1 per client, 1 per trustee)
func (dcnet *DCNetRound) hasAllCiphers(p *PriFiProtocol) bool {
	if p.relayState.nClients == dcnet.clientCipherCount && p.relayState.nTrustees == dcnet.trusteeCipherCount {
		return true
	}
	return false
}

//holds the ciphertexts received in advance from the trustees
type BufferedCipher struct {
	RoundId int32
	Data    map[int][]byte
}

//the mutable variable held by the client
type RelayState struct {
	//RelayPort				string
	//PublicKey				abstract.Point
	//privateKey			abstract.Secret
	//trusteesHosts			[]string

	bufferedTrusteeCiphers   map[int32]BufferedCipher
	bufferedClientCiphers    map[int32]BufferedCipher
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
	nTrusteesPkCollected     int
	privateKey               abstract.Secret
	PublicKey                abstract.Point
	ReportingLimit           int
	trustees                 []NodeRepresentation
	UpstreamCellSize         int
	UseDummyDataDown         bool
	UseUDP                   bool
	WindowSize               int
}

/**
 * Used to initialize the state of this relay. Must be called before anything else.
 */
func NewRelayState(nTrustees int, nClients int, upstreamCellSize int, downstreamCellSize int, windowSize int, useDummyDataDown bool, reportingLimit int, useUDP bool, dataOutputEnabled bool) *RelayState {
	params := new(RelayState)
	params.Name = "Relay"
	params.CellCoder = config.Factory()
	params.clients = make([]NodeRepresentation, 0)
	params.DataForClients = make(chan []byte)
	params.DataFromDCNet = make(chan []byte)
	params.DataOutputEnabled = dataOutputEnabled
	params.DownstreamCellSize = downstreamCellSize
	//params.MessageHistory =
	params.nClients = nClients
	params.nTrustees = nTrustees
	params.nTrusteesPkCollected = 0
	params.ReportingLimit = reportingLimit
	params.trustees = make([]NodeRepresentation, nTrustees)
	params.UpstreamCellSize = upstreamCellSize
	params.UseDummyDataDown = useDummyDataDown
	params.UseUDP = useUDP
	params.WindowSize = windowSize

	//prepare the crypto parameters
	rand := config.CryptoSuite.Cipher([]byte(params.Name))
	base := config.CryptoSuite.Point().Base()

	//generate own parameters
	params.privateKey = config.CryptoSuite.Secret().Pick(rand)
	params.PublicKey = config.CryptoSuite.Point().Mul(base, params.privateKey)

	//sets the new state
	params.currentState = RELAY_STATE_COLLECTING_TRUSTEES_PKS

	return params
}

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

	p.relayState = *NewRelayState(msg.NTrustees, msg.NClients, msg.UpCellSize, msg.DownCellSize, msg.RelayWindowSize, msg.RelayUseDummyDataDown, msg.RelayReportingLimit, msg.UseUDP, msg.RelayDataOutputEnabled)

	dbg.Lvlf5("%+v\n", p.relayState)
	dbg.Lvl1("Relay has been initialized by message. ")

	//broadcast those parameters to the other nodes, then tell the trustees which ID they are.
	if msg.StartNow {
		p.relayState.currentState = RELAY_STATE_COLLECTING_TRUSTEES_PKS
		p.ConnectToTrustees()
	}
	dbg.Lvl1("Relay setup done, and setup sent to the trustees.")

	return nil
}

/**
 * This initializes the trustees with default parameters.
 * TODO : if they are not constants anymore, we need a way to change those fields. For now, trustees don't need much information
 */
func (p *PriFiProtocol) ConnectToTrustees() error {

	//craft default parameters
	var msg = &ALL_ALL_PARAMETERS{
		NClients:          p.relayState.nClients,
		NextFreeTrusteeId: 0,
		NTrustees:         p.relayState.nTrustees,
		StartNow:          true,
		ForceParams:       true,
		UpCellSize:        p.relayState.UpstreamCellSize,
	}

	//Send those parameters to all trustees
	for j := 0; j < p.relayState.nTrustees; j++ {

		//The ID is unique !
		msg.NextFreeTrusteeId = j
		err := p.messageSender.SendToTrustee(j, msg)
		if err != nil {
			e := "Could not send ALL_TRU_PARAMETERS to Trustee " + strconv.Itoa(j) + ", error is " + err.Error()
			dbg.Error(e)
			return errors.New(e)
		} else {
			dbg.Lvl3("Relay : sent ALL_TRU_PARAMETERS to Trustee " + strconv.Itoa(j) + ".")
		}
	}

	return nil
}

/**
 * This is part of PriFi's main loop. This is what happens in one round, for the relay.
 * We receive some upstream data. If we have collected data from all entities for this round, we can call DecodeCell() and get the output.
 * If we get data for another round (in the future) we should buffer it.
 * TODO : We should timeout if some client did not send anything after a while
 * If we finished a round (we had collected all data, and called DecodeCell()), we need to finish the round by sending some data down.
 * Either we send something from the SOCKS/VPN buffer, or we answer the latency-test message if we received any, or we send 1 bit.
 */
func (p *PriFiProtocol) Received_CLI_REL_UPSTREAM_DATA(msg CLI_REL_UPSTREAM_DATA) error {

	//this can only happens in the state RELAY_STATE_COMMUNICATING
	if p.relayState.currentState != RELAY_STATE_COMMUNICATING {
		e := "Relay : Received a CLI_REL_UPSTREAM_DATA, but not in state RELAY_STATE_COMMUNICATING, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		//return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received CLI_REL_UPSTREAM_DATA")
	}

	//TODO : add a timeout somewhere here

	//if this is not the message destinated for this round, discard it ! (we are in lock-step)
	if p.relayState.currentDCNetRound.currentRound != msg.RoundId {
		e := "Relay : Client sent DC-net cipher for round , " + strconv.Itoa(int(msg.RoundId)) + " but current round is " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound))
		dbg.Error(e)

		//else, we need to buffer this message somewhere
		if _, ok := p.relayState.bufferedClientCiphers[msg.RoundId]; ok {
			//the roundId already exists, simply add data
			p.relayState.bufferedClientCiphers[msg.RoundId].Data[msg.ClientId] = msg.Data
		} else {
			//else, create the key in the map, and store the data
			newKey := BufferedCipher{msg.RoundId, make(map[int][]byte)}
			newKey.Data[msg.ClientId] = msg.Data
			p.relayState.bufferedClientCiphers[msg.RoundId] = newKey
		}

		//return errors.New(e)

	} else {
		//else, if this is the message we need for this round

		p.relayState.CellCoder.DecodeClient(msg.Data)
		p.relayState.currentDCNetRound.clientCipherCount++

		dbg.Lvl3("Relay collecting cells for round", p.relayState.currentDCNetRound.currentRound, ", ", p.relayState.currentDCNetRound.clientCipherCount, "/", p.relayState.nClients, ", ", p.relayState.currentDCNetRound.trusteeCipherCount, "/", p.relayState.nTrustees)

		if p.relayState.currentDCNetRound.hasAllCiphers(p) {

			dbg.Lvl3("Relay has collected all ciphers (2), decoding...")
			p.finalizeUpstreamData()

			//sleep so it does not go too fast for debug
			time.Sleep(1000 * time.Millisecond)

			//send the data down (to finalize this round)
			p.sendDownstreamData()
		}
	}

	return nil
}

/**
 * This message happens when we receive a DC-net cipher from a Trustee.
 * If it's for this round, we call decode on it, and remember we received it.
 * If for a round in the futur, we need to Buffer it.
 * TODO : given the number of already-buffered Ciphers (per trustee), we need to tell him to slow down
 */
func (p *PriFiProtocol) Received_TRU_REL_DC_CIPHER(msg TRU_REL_DC_CIPHER) error {

	//this can only happens in the state RELAY_STATE_COMMUNICATING
	if p.relayState.currentState != RELAY_STATE_COMMUNICATING {
		e := "Relay : Received a TRU_REL_DC_CIPHER, but not in state RELAY_STATE_COMMUNICATING, in state " + strconv.Itoa(int(p.relayState.currentState))
		dbg.Error(e)
		//return errors.New(e)
	} else {
		dbg.Lvl3("Relay : received TRU_REL_DC_CIPHER")
	}

	//TODO @ Mohamad : add rate-control somewhere here

	//if this is the message we need for this round
	if p.relayState.currentDCNetRound.currentRound == msg.RoundId {

		dbg.Lvl3("Relay collecting cells for round", p.relayState.currentDCNetRound.currentRound, ", ", p.relayState.currentDCNetRound.clientCipherCount, "/", p.relayState.nClients, ", ", p.relayState.currentDCNetRound.trusteeCipherCount, "/", p.relayState.nTrustees)

		p.relayState.CellCoder.DecodeTrustee(msg.Data)
		p.relayState.currentDCNetRound.trusteeCipherCount++

		if p.relayState.currentDCNetRound.hasAllCiphers(p) {

			dbg.Lvl3("Relay has collected all ciphers, decoding...")
			p.finalizeUpstreamData()

			//sleep so it does not go too fast for debug
			time.Sleep(1000 * time.Millisecond)

			//send the data down (to finalize this round)
			p.sendDownstreamData()
		}
	} else {
		//else, we need to buffer this message somewhere
		if _, ok := p.relayState.bufferedTrusteeCiphers[msg.RoundId]; ok {
			//the roundId already exists, simply add data
			p.relayState.bufferedTrusteeCiphers[msg.RoundId].Data[msg.TrusteeId] = msg.Data
		} else {
			//else, create the key in the map, and store the data
			newKey := BufferedCipher{msg.RoundId, make(map[int][]byte)}
			newKey.Data[msg.TrusteeId] = msg.Data
			p.relayState.bufferedTrusteeCiphers[msg.RoundId] = newKey
		}
	}
	return nil
}

/**
 * This is simply called when the Relay has received all ciphertext (one per client, one per trustee), and is ready to finalize the
 * DC-net round by XORing everything together.
 * If it's a latency-test message, we send it back to the clients.
 * If we use SOCKS/VPN, give them the data.
 */
func (p *PriFiProtocol) finalizeUpstreamData() error {

	//we decode the DC-net cell
	upstreamPlaintext := p.relayState.CellCoder.DecodeCell()

	//check if we have a latency test message
	if len(upstreamPlaintext) >= 2 {
		pattern := int(binary.BigEndian.Uint16(upstreamPlaintext[0:2]))
		if pattern == 43690 { //1010101010101010
			dbg.Lvl3("Relay : noticed a latency-test message, sending answer...")
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

	dbg.Lvl3("Relay : Outputted upstream cell (finalized round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)) + "), sending downstream data.")

	return nil
}

/**
 * This is simply called when the Relay has processed the upstream cell from all clients, and is ready to finalize the round by sending the data down.
 * If it's a latency-test message, we send it back to the clients.
 * If we use SOCKS/VPN, give them the data.
 */
func (p *PriFiProtocol) sendDownstreamData() error {

	var downstreamCellContent []byte

	select {

	//either select data from the data we have to send, if any
	case downstreamCellContent = <-p.relayState.DataForClients:
		dbg.Lvl3("Relay : We have some real data for the clients. ")

	default:
		downstreamCellContent = make([]byte, 1)
		dbg.Lvl3("Relay : Sending 1bit down. ")
	}

	//if we want to use dummy data down, pad to the correct size
	if p.relayState.UseDummyDataDown && len(downstreamCellContent) < p.relayState.DownstreamCellSize {
		data := make([]byte, p.relayState.DownstreamCellSize)
		copy(data[0:], downstreamCellContent)
		downstreamCellContent = data
	}

	//TODO : if something went wrong before, this flag should be used to warn the clients that the config has changed
	flagResync := false
	dbg.Lvl3("Relay is gonna broadcast messages for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)) + ".")

	if !p.relayState.UseUDP {
		//broadcast to all clients
		for i := 0; i < p.relayState.nClients; i++ {
			//send to the i-th client
			toSend := &REL_CLI_DOWNSTREAM_DATA{p.relayState.currentDCNetRound.currentRound, downstreamCellContent, flagResync}
			err := p.messageSender.SendToClient(i, toSend)
			if err != nil {
				e := "Could not send REL_CLI_DOWNSTREAM_DATA to " + strconv.Itoa(i+1) + "-th client for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)) + ", error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				dbg.Lvl3("Relay : sent REL_CLI_DOWNSTREAM_DATA to " + strconv.Itoa(i+1) + "-th client for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)))
			}
		}
	} else {
		panic("UDP not supported yet")
	}
	dbg.Lvl3("Relay is done broadcasting messages for round " + strconv.Itoa(int(p.relayState.currentDCNetRound.currentRound)) + ".")

	//prepare for the next round
	nextRound := p.relayState.currentDCNetRound.currentRound + 1
	p.relayState.currentDCNetRound = DCNetRound{nextRound, 0, 0}
	p.relayState.CellCoder.DecodeStart(p.relayState.UpstreamCellSize, p.relayState.MessageHistory) //this empties the buffer, making them ready for a new round

	//if we have buffered messages for next round, use them now, so whenever we receive a client message, the trustee's message are counted correctly
	if _, ok := p.relayState.bufferedTrusteeCiphers[nextRound]; ok {
		for trusteeId, data := range p.relayState.bufferedTrusteeCiphers[nextRound].Data {
			//start decoding using this data
			dbg.Lvl3("Relay : using pre-cached DC-net cipher from trustee " + strconv.Itoa(trusteeId) + " for round " + strconv.Itoa(int(nextRound)))
			p.relayState.CellCoder.DecodeTrustee(data)
			p.relayState.currentDCNetRound.trusteeCipherCount++
		}
		delete(p.relayState.bufferedTrusteeCiphers, nextRound)
	}
	if _, ok := p.relayState.bufferedClientCiphers[nextRound]; ok {
		for clientId, data := range p.relayState.bufferedClientCiphers[nextRound].Data {
			//start decoding using this data
			dbg.Lvl3("Relay : using pre-cached DC-net cipher from client " + strconv.Itoa(clientId) + " for round " + strconv.Itoa(int(nextRound)))
			p.relayState.CellCoder.DecodeTrustee(data)
			p.relayState.currentDCNetRound.clientCipherCount++
		}
		delete(p.relayState.bufferedClientCiphers, nextRound)
	}

	dbg.Lvl2("Relay has finished round" + strconv.Itoa(int(nextRound-1)) + ".")
	return nil
}

/**
 * We receive this message when we connect to a Trustee.
 * We do nothing, until we have received one per trustee; Then, we pack them in one message, and broadcast to the clients.
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

	dbg.Lvl2("Relay : received TRU_REL_TELL_PK (" + strconv.Itoa(p.relayState.nTrusteesPkCollected) + "/" + strconv.Itoa(p.relayState.nTrustees) + ")")

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

/**
 * We received this message when the client tells their identity.
 * We do nothing until we have collected one per client; then, we pack them in one message, and send them to the first trustee for
 * him to Neff-Shuffle them.
 */
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

	dbg.Lvl2("Relay : received CLI_REL_TELL_PK_AND_EPH_PK (" + strconv.Itoa(len(p.relayState.clients)) + "/" + strconv.Itoa(p.relayState.nClients) + ")")

	//if we have collected all clients, continue
	if len(p.relayState.clients) == p.relayState.nClients {

		//prepare the arrays; pack the public keys and ephemeral public keys
		pks := make([]abstract.Point, p.relayState.nClients)
		ephPks := make([]abstract.Point, p.relayState.nClients)
		for i := 0; i < p.relayState.nClients; i++ {
			pks[i] = p.relayState.clients[i].PublicKey
			ephPks[i] = p.relayState.clients[i].EphemeralPublicKey
		}

		//G := config.CryptoSuite.Point().Base()
		G := p.relayState.clients[0].PublicKey //TODO : Fix this

		//prepare the empty shuffle
		emptyG_s := make([]abstract.Point, p.relayState.nTrustees)
		emptyEphPks_s := make([][]abstract.Point, p.relayState.nTrustees)
		emptyProof_s := make([][]byte, p.relayState.nTrustees)
		emptySignature_s := make([][]byte, p.relayState.nTrustees)
		p.relayState.currentShuffleTranscript = NeffShuffleState{pks, emptyG_s, emptyEphPks_s, emptyProof_s, 0, emptySignature_s, 0}

		//send to the 1st trustee
		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.messageSender.SendToTrustee(0, toSend)
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

/**
 * We receive this message once a trustee has finished a Neff-Shuffle.
 * In that case, we forward the result to the next trustee.
 * We do nothing until the last trustee sends us this message. When this happens, we pack a transcript, and broadcast to the trustees (they need to sign it)
 */
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

	dbg.Lvl2("Relay : received TRU_REL_TELL_NEW_BASE_AND_EPH_PKS (" + strconv.Itoa(p.relayState.currentShuffleTranscript.nextFreeId_Proofs) + "/" + strconv.Itoa(p.relayState.nTrustees) + ")")

	//if we're still waiting on some trustees, send them the new shuffle
	if p.relayState.currentShuffleTranscript.nextFreeId_Proofs != p.relayState.nTrustees {

		pks := p.relayState.currentShuffleTranscript.ClientPublicKeys
		ephPks := msg.NewEphPks
		G := msg.NewBase

		//send to the i-th trustee
		toSend := &REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE{pks, ephPks, G}
		err := p.messageSender.SendToTrustee(j+1, toSend)
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
		p.relayState.bufferedTrusteeCiphers = make(map[int32]BufferedCipher)
		p.relayState.bufferedClientCiphers = make(map[int32]BufferedCipher)

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

		//prepare to collect the ciphers
		p.relayState.currentDCNetRound = DCNetRound{0, 0, 0}
		p.relayState.CellCoder.DecodeStart(p.relayState.UpstreamCellSize, p.relayState.MessageHistory)

		//changing state
		p.relayState.currentState = RELAY_STATE_COLLECTING_SHUFFLE_SIGNATURES
	}

	return nil
}

/**
 * This happens when we receive the signature from the NeffShuffleS-transcript from one trustee.
 * We do nothing until we have all signatures; when we do, we pack those in one message, as well as the result of the Neff-Shuffle, and send them to the clients.
 * When this is done, we are finally ready to communicate. We wait for the client's messages.
 */
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

	dbg.Lvl2("Relay : received TRU_REL_SHUFFLE_SIG (" + strconv.Itoa(p.relayState.currentShuffleTranscript.signature_count) + "/" + strconv.Itoa(p.relayState.nTrustees) + ")")

	//if we have all the signatures
	if p.relayState.currentShuffleTranscript.signature_count == p.relayState.nTrustees {

		//We could verify here before broadcasting to the clients, for performance (but this does not add security)

		//prepare the message for the clients
		lastPermutationIndex := p.relayState.nTrustees - 1
		G := p.relayState.currentShuffleTranscript.G_s[lastPermutationIndex]
		ephPks := p.relayState.currentShuffleTranscript.ephPubKeys_s[lastPermutationIndex]
		signatures := p.relayState.currentShuffleTranscript.signatures_s

		//changing state
		dbg.Lvl2("Relay : ready to communicate.")
		p.relayState.currentState = RELAY_STATE_COMMUNICATING

		//broadcast to all clients
		for i := 0; i < p.relayState.nClients; i++ {
			//send to the i-th client
			toSend := &REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG{G, ephPks, signatures}
			err := p.messageSender.SendToClient(i, toSend)
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
