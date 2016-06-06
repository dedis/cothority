package prifi

import ()

type PriFiProtocol struct {
	messageSender MessageSender
}

type MessageSender interface {
	SendToClient(i int, msg interface{}) error

	SendToTrustee(i int, msg interface{}) error

	SendToRelay(msg interface{}) error
}

func (prifi *PriFiProtocol) ReceivedMessage(msg interface{}) error {

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
	case CLI_REL_TELL_PK_AND_EPH_PK:
		return prifi.Received_CLI_REL_TELL_PK_AND_EPH_PK(typedMsg)
	case CLI_REL_UPSTREAM_DATA:
		return prifi.Received_CLI_REL_UPSTREAM_DATA(typedMsg)
	case REL_CLI_DOWNSTREAM_DATA:
		return prifi.Received_REL_CLI_DOWNSTREAM_DATA(typedMsg)
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

func NewPriFiRelay(msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{msgSender}

	return &prifi
}

func NewPriFiClient(clientId int, msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{msgSender}

	return &prifi
}

func NewPriFiTrustee(trusteeId int, msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{msgSender}

	return &prifi
}
