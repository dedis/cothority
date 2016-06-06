package prifi

import (
	"github.com/dedis/cothority/lib/dbg"
)

const (
	PRIFI_ROLE_UNDEFINED int16 = iota
	PRIFI_ROLE_RELAY
	PRIFI_ROLE_CLIENT
	PRIFI_ROLE_TRUSTEE
)

type PriFiProtocol struct {
	role          int16
	messageSender MessageSender
}

type MessageSender interface {
	SendToClient(i int, msg interface{}) error

	SendToTrustee(i int, msg interface{}) error

	SendToRelay(msg interface{}) error
}

func (prifi *PriFiProtocol) ReceivedMessage(msg interface{}) error {

	if prifi == nil {
		dbg.Print("Received a message ", msg)
		panic("But prifi is nil !")
	}

	//ALL_ALL_PARAMETERS
	//CLI_REL_TELL_PK_AND_EPH_PK
	//CLI_REL_UPSTREAM_DATA
	//REL_CLI_DOWNSTREAM_DATA
	//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
	//REL_CLI_TELL_TRUSTEES_PK
	//REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
	//REL_TRU_TELL_TRANSCRIPT
	//TRU_REL_DC_CIPHER
	//TRU_REL_SHUFFLE_SIG
	//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
	//TRU_REL_TELL_PK

	switch typedMsg := msg.(type) {
	case ALL_ALL_PARAMETERS:
		switch prifi.role {
		case PRIFI_ROLE_RELAY:
			return prifi.Received_ALL_REL_PARAMETERS(typedMsg)
		case PRIFI_ROLE_CLIENT:
			return prifi.Received_ALL_CLI_PARAMETERS(typedMsg)
		case PRIFI_ROLE_TRUSTEE:
			return prifi.Received_ALL_TRU_PARAMETERS(typedMsg)
		default:
			panic("Received parameters, but we have no role yet !")
		}
	case CLI_REL_TELL_PK_AND_EPH_PK:
		return prifi.Received_CLI_REL_TELL_PK_AND_EPH_PK(typedMsg)
	case CLI_REL_UPSTREAM_DATA:
		return prifi.Received_CLI_REL_UPSTREAM_DATA_dummypingpong(typedMsg)
	case REL_CLI_DOWNSTREAM_DATA:
		return prifi.Received_REL_CLI_DOWNSTREAM_DATA_dummypingpong(typedMsg)
	case REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG:
		return prifi.Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(typedMsg)
	case REL_CLI_TELL_TRUSTEES_PK:
		return prifi.Received_REL_CLI_TELL_TRUSTEES_PK(typedMsg)
	case REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE:
		return prifi.Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE(typedMsg)
	case REL_TRU_TELL_TRANSCRIPT:
		return prifi.Received_REL_TRU_TELL_TRANSCRIPT(typedMsg)
	case TRU_REL_DC_CIPHER:
		return prifi.Received_TRU_REL_DC_CIPHER(typedMsg)
	case TRU_REL_SHUFFLE_SIG:
		return prifi.Received_TRU_REL_SHUFFLE_SIG(typedMsg)
	case TRU_REL_TELL_NEW_BASE_AND_EPH_PKS:
		return prifi.Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(typedMsg)
	case TRU_REL_TELL_PK:
		return prifi.Received_TRU_REL_TELL_PK(typedMsg)
	default:
		panic("unrecognized message !")
	}

	return nil
}

func NewPriFiRelay(msgSender MessageSender, state *RelayState) *PriFiProtocol {
	prifi := PriFiProtocol{PRIFI_ROLE_RELAY, msgSender}
	relayState = *state
	return &prifi
}

func NewPriFiClient(msgSender MessageSender, state *ClientState) *PriFiProtocol {
	prifi := PriFiProtocol{PRIFI_ROLE_CLIENT, msgSender}
	clientState = *state
	return &prifi
}

func NewPriFiTrustee(msgSender MessageSender, state *TrusteeState) *PriFiProtocol {
	prifi := PriFiProtocol{PRIFI_ROLE_TRUSTEE, msgSender}
	trusteeState = *state
	return &prifi
}
