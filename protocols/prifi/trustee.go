package prifi

import (
	"github.com/dedis/crypto/abstract"
	"github.com/lbarman/prifi/dcnet"
)

//State information to hold :
type TrusteeState struct {
	Name          string
	TrusteeId     int
	PayloadLength int
	//activeConnection net.Conn //those are kept by the SDA stack

	//PublicKey  abstract.Point //those are kept by the SDA stack
	//privateKey abstract.Secret //those are kept by the SDA stack

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

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_TRU_TELL_TRANSCRIPT(msg Struct_REL_TRU_TELL_TRANSCRIPT) error {

	return nil
}
