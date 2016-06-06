package prifi

import (
	"errors"
	"time"

	"bytes"
	"strconv"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/prifi/config"
	"github.com/dedis/cothority/lib/prifi/crypto"
	"github.com/dedis/cothority/lib/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
	crypto_proof "github.com/dedis/crypto/proof"
	"github.com/dedis/crypto/shuffle"
)

const (
	TRUSTEE_STATE_INITIALIZING int16 = iota
	TRUSTEE_STATE_SHUFFLE_DONE
	TRUSTEE_STATE_READY
)
const (
	TRUSTEE_KILL_SEND_PROCESS int16 = iota
	TRUSTEE_RATE_STOPPED
	TRUSTEE_RATE_HALF
	TRUSTEE_RATE_FULL
)

const TRUSTEE_SLEEP_TIME = 1 * time.Second

//State information to hold :
var trusteeState TrusteeState

type TrusteeState struct {
	Id            int
	Name          string
	TrusteeId     int
	PayloadLength int
	//activeConnection net.Conn //those are kept by the SDA stack

	PublicKey  abstract.Point  //those are kept by the SDA stack
	privateKey abstract.Secret //those are kept by the SDA stack

	nClients  int
	nTrustees int

	ClientPublicKeys []abstract.Point
	sharedSecrets    []abstract.Point

	MessageHistory abstract.Cipher
	CellCoder      dcnet.CellCoder

	neffShuffleToVerify NeffShuffleResult

	currentState int16
	sendingRate  chan int16
}

type NeffShuffleResult struct {
	base  abstract.Point
	pks   []abstract.Point
	proof []byte
}

/**
 * Used to initialize the state of this trustee. Must be called before anything else.
 */
func NewTrusteeState(trusteeId int, nClients int, nTrustees int, payloadLength int) *TrusteeState {
	params := new(TrusteeState)

	params.Id = trusteeId
	params.Name = "Trustee-" + strconv.Itoa(trusteeId)
	params.TrusteeId = trusteeId
	params.nClients = nClients
	params.nTrustees = nTrustees
	params.PayloadLength = payloadLength

	//prepare the crypto parameters
	rand := config.CryptoSuite.Cipher([]byte(params.Name))
	base := config.CryptoSuite.Point().Base()

	//generate own parameters
	params.privateKey = config.CryptoSuite.Secret().Pick(rand)                 //NO, this should be kept by SDA
	params.PublicKey = config.CryptoSuite.Point().Mul(base, params.privateKey) //NO, this should be kept by SDA

	//placeholders for pubkeys and secrets
	params.ClientPublicKeys = make([]abstract.Point, nClients)
	params.sharedSecrets = make([]abstract.Point, nClients)

	//sets the cell coder, and the history
	params.neffShuffleToVerify = NeffShuffleResult{}
	params.CellCoder = config.Factory()

	params.currentState = TRUSTEE_STATE_INITIALIZING
	params.sendingRate = make(chan int16)

	return params
}

//Messages to handle :
//REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
//REL_TRU_TELL_TRANSCRIPT

/**
 * This method sends DC-net ciphers to the relay, once started. One can control the rate by sending data to "rateChan".
 */
func (p *PriFiProtocol) Send_TRU_REL_DC_CIPHER(rateChan chan int16) {

	stop := false
	currentRate := TRUSTEE_RATE_STOPPED
	roundId := int32(0)

	for !stop {
		select {
		case newRate := <-rateChan:
			currentRate = newRate
			dbg.Error("Trustee " + strconv.Itoa(trusteeState.Id) + " : rate changed from " + strconv.Itoa(int(currentRate)) + " to " + strconv.Itoa(int(newRate)))

			if newRate == TRUSTEE_KILL_SEND_PROCESS {
				stop = true
			}

		default:
			if currentRate == TRUSTEE_RATE_FULL {
				roundId, _ = sendData(p, roundId)

			} else if currentRate == TRUSTEE_RATE_HALF {
				roundId, _ = sendData(p, roundId)
				time.Sleep(TRUSTEE_SLEEP_TIME)

			} else if currentRate == TRUSTEE_RATE_STOPPED {
				time.Sleep(TRUSTEE_SLEEP_TIME)
			}

		}
	}

}

/**
 * Auxiliary function used by Send_TRU_REL_DC_CIPHER
 */
func sendData(p *PriFiProtocol, roundId int32) (int32, error) {
	data := trusteeState.CellCoder.TrusteeEncode(trusteeState.PayloadLength)

	//send the data
	toSend := &TRU_REL_DC_CIPHER{roundId, trusteeState.Id, data}
	err := p.messageSender.SendToRelay(toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send Struct_TRU_REL_DC_CIPHER for round (" + strconv.Itoa(int(roundId)) + ") error is " + err.Error()
		dbg.Error(e)
		return roundId, errors.New(e)
	} else {
		dbg.Lvl5("Trustee " + strconv.Itoa(trusteeState.Id) + " : sent cipher " + strconv.Itoa(int(roundId)))
	}

	return roundId + 1, nil
}

/**
 * This message happens when the connection to a relay is established. It contains the long-term and ephemeral public keys of the clients,
 * a base given by the relay. In addition to deriving the secrets, the trustees uses the ephemeral keys to perform a neff shuffle. He remembers
 * this shuffle, to check the correctness of the chain of shuffle afterwards.
 */
func (p *PriFiProtocol) Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE(msg REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE) error {

	//this can only happens in the state TRUSTEE_STATE_INITIALIZING
	if trusteeState.currentState != TRUSTEE_STATE_INITIALIZING {
		e := "Trustee " + strconv.Itoa(trusteeState.Id) + " : Received a REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE, but not in state TRUSTEE_STATE_INITIALIZING, in state " + strconv.Itoa(int(trusteeState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(trusteeState.Id) + " : Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE")
	}

	//begin parsing the message
	rand := config.CryptoSuite.Cipher([]byte(trusteeState.Name)) //TODO: this should be random
	clientsPks := msg.Pks
	clientsEphemeralPks := msg.EphPks
	base := msg.Base

	//sanity check
	if len(clientsPks) < 2 || len(clientsEphemeralPks) < 2 || len(clientsPks) != len(clientsEphemeralPks) {
		e := "Trustee " + strconv.Itoa(trusteeState.Id) + " : One of the following check failed : len(clientsPks)>1, len(clientsEphemeralPks)>1, len(clientsPks)==len(clientsEphemeralPks)"
		dbg.Error(e)
		return errors.New(e)
	}

	//fill in the clients keys
	for i := 0; i < len(clientsPks); i++ {
		//trusteeState.ClientPublicKeys[i] = clientsPublicKeys[i] not sure this is needed since there is a tree ?
		trusteeState.sharedSecrets[i] = config.CryptoSuite.Point().Mul(clientsPks[i], trusteeState.privateKey)
	}

	//TODO : THIS IS NOT SHUFFLING; THIS IS A PLACEHOLDER FOR THE ACTUAL SHUFFLE. NOT SHUFFLE IS DONE

	//perform the neff-shuffle
	H := trusteeState.PublicKey
	X := clientsEphemeralPks
	Y := X

	_, _, prover := shuffle.Shuffle(config.CryptoSuite, nil, H, X, Y, rand)
	_, err := crypto_proof.HashProve(config.CryptoSuite, "PairShuffle", rand, prover)
	if err != nil {
		e := "Could not neff-shuffle, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	}

	//base2, ephPublicKeys2, proof := NeffShuffle(base, ephPublicKey)
	base2 := base
	ephPublicKeys2 := clientsEphemeralPks
	proof := make([]byte, 50)

	//send the answer
	toSend := &TRU_REL_TELL_NEW_BASE_AND_EPH_PKS{base2, ephPublicKeys2, proof}
	err = p.messageSender.SendToRelay(toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send TRU_REL_TELL_NEW_BASE_AND_EPH_PKS, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(trusteeState.Id) + " : sent TRU_REL_TELL_NEW_BASE_AND_EPH_PKS")
	}

	//remember our shuffle
	trusteeState.neffShuffleToVerify = NeffShuffleResult{base2, ephPublicKeys2, proof}

	//change state
	trusteeState.currentState = TRUSTEE_STATE_SHUFFLE_DONE

	return nil
}

/**
 * This message happens when all trustees have already shuffled. They need to verify all the shuffles, and also that
 * their own shuffle has been included in the chain of shuffles. If that's the case, this trustee signs the *last*
 * shuffle, and send it back to the relay.
 * If everything succeed, starts the goroutine for sending DC-net ciphers to the relay
 */
func (p *PriFiProtocol) Received_REL_TRU_TELL_TRANSCRIPT(msg REL_TRU_TELL_TRANSCRIPT) error {

	//this can only happens in the state TRUSTEE_STATE_SHUFFLE_DONE
	if trusteeState.currentState != TRUSTEE_STATE_SHUFFLE_DONE {
		e := "Trustee " + strconv.Itoa(trusteeState.Id) + " : Received a REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE, but not in state TRUSTEE_STATE_SHUFFLE_DONE, in state " + strconv.Itoa(int(trusteeState.currentState))
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(trusteeState.Id) + " : Received_REL_TRU_TELL_TRANSCRIPT")
	}

	//begin parsing the message
	rand := config.CryptoSuite.Cipher([]byte(trusteeState.Name)) //TODO: this should be random
	G_s := msg.G_s
	ephPublicKeys_s := msg.EphPks
	proof_s := msg.Proofs

	//Todo : verify each individual permutations
	var err error = nil
	for j := 0; j < trusteeState.nTrustees; j++ {

		verify := true
		if j > 0 {
			H := G_s[j]
			X := ephPublicKeys_s[j-1]
			Y := ephPublicKeys_s[j-1]
			Xbar := ephPublicKeys_s[j]
			Ybar := ephPublicKeys_s[j]
			if len(X) > 1 {
				verifier := shuffle.Verifier(config.CryptoSuite, nil, H, X, Y, Xbar, Ybar)
				err = crypto_proof.HashVerify(config.CryptoSuite, "PairShuffle", verifier, proof_s[j])
			}
			if err != nil {
				verify = false
			}
		}
		verify = true

		if !verify {
			if err != nil {
				e := "Could not verify the " + strconv.Itoa(j) + "th neff shuffle, error is " + err.Error()
				dbg.Error(e)
				return errors.New(e)
			} else {
				e := "Could not verify the " + strconv.Itoa(j) + "th neff shuffle, error is unknown."
				dbg.Error(e)
				return errors.New(e)
			}
		}
	}

	//we verify that our shuffle was included
	ownPermutationFound := false
	for j := 0; j < trusteeState.nTrustees; j++ {

		if G_s[j].Equal(trusteeState.neffShuffleToVerify.base) && bytes.Equal(trusteeState.neffShuffleToVerify.proof, proof_s[j]) {

			dbg.Lvl3("Trustee " + strconv.Itoa(trusteeState.Id) + "; Find in transcript : Found indice " + strconv.Itoa(j) + " that seems to match, verifing all the keys...")

			allKeyEqual := true

			for k := 0; k < trusteeState.nClients; k++ {
				if !trusteeState.neffShuffleToVerify.pks[k].Equal(ephPublicKeys_s[j][k]) {
					dbg.Lvl1("Trustee " + strconv.Itoa(trusteeState.Id) + "; Transcript invalid for trustee " + strconv.Itoa(j) + ". Aborting.")
					allKeyEqual = false
					break
				}
			}

			if allKeyEqual {
				ownPermutationFound = true
			}
		}
	}

	if !ownPermutationFound {
		e := "Trustee " + strconv.Itoa(trusteeState.Id) + "; Can't find own transaction. Aborting."
		dbg.Error(e)
		return errors.New(e)
	}

	//prepare the transcript signature. Since it is OK, we're gonna sign the latest permutation
	M := make([]byte, 0)
	G_s_j_bytes, err := G_s[trusteeState.nTrustees-1].MarshalBinary()
	if err != nil {
		e := "Trustee " + strconv.Itoa(trusteeState.Id) + "; Can't marshall base, " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	}
	M = append(M, G_s_j_bytes...)

	for j := 0; j < trusteeState.nClients; j++ {
		pkBytes, err := ephPublicKeys_s[trusteeState.nTrustees-1][j].MarshalBinary()
		if err != nil {
			e := "Trustee " + strconv.Itoa(trusteeState.Id) + "; Can't marshall public key, " + err.Error()
			dbg.Error(e)
			return errors.New(e)
		}
		M = append(M, pkBytes...)
	}

	sig := crypto.SchnorrSign(config.CryptoSuite, rand, M, trusteeState.privateKey)

	dbg.Lvl2("Trustee " + strconv.Itoa(trusteeState.Id) + "; Sending signature")

	//send the answer
	toSend := &TRU_REL_SHUFFLE_SIG{trusteeState.Id, sig}
	err = p.messageSender.SendToRelay(toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send TRU_REL_SHUFFLE_SIG, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	} else {
		dbg.Lvl3("Trustee " + strconv.Itoa(trusteeState.Id) + " : sent TRU_REL_SHUFFLE_SIG")
	}

	//we can forget our shuffle
	//trusteeState.neffShuffleToVerify = NeffShuffleResult{base2, ephPublicKeys2, proof}

	//change state
	trusteeState.currentState = TRUSTEE_STATE_READY

	//everything is ready, we start sending
	go p.Send_TRU_REL_DC_CIPHER(trusteeState.sendingRate)

	return nil
}
