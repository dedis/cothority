package prifi

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/prifi/config"
	"github.com/dedis/cothority/protocols/prifi/dcnet"
	"github.com/dedis/crypto/abstract"
	crypto_proof "github.com/dedis/crypto/proof"
	"github.com/dedis/crypto/shuffle"
)

//State information to hold :
var trusteeState TrusteeState

type TrusteeState struct {
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

	CellCoder dcnet.CellCoder //TODO : Code it here

	MessageHistory abstract.Cipher
}

//Messages to handle :
//REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
//REL_TRU_TELL_TRANSCRIPT

func (p *PriFiProtocolHandlers) Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE(msg Struct_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE) error {

	clientsPks := msg.Pks
	clientsEphemeralPks := msg.EphPks
	base := msg.Base

	//sanity check
	if len(clientsPks) < 2 || len(clientsEphemeralPks) < 2 || len(clientsPks) != len(clientsEphemeralPks) {
		e := "One of the following check failed : len(clientsPks)>1, len(clientsEphemeralPks)>1, len(clientsPks)==len(clientsEphemeralPks)"
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
	rand := config.CryptoSuite.Cipher([]byte(trusteeState.Name)) //TODO: this should be random
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
	err = p.SendTo(p.Parent(), toSend) //TODO : this should be the root ! make sure of it
	if err != nil {
		e := "Could not send REL_CLI_DOWNSTREAM_DATA, error is " + err.Error()
		dbg.Error(e)
		return errors.New(e)
	}

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_TRU_TELL_TRANSCRIPT(msg Struct_REL_TRU_TELL_TRANSCRIPT) error {

	return nil
}
