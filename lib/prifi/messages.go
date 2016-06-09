package prifi

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
)

/*
 * Messages used by PriFi.
 * Syntax : SOURCE_DEST_CONTENT_CONTENT
 *
 * Below : Message-Switch that calls the correct function when one of this message arrives.
 */

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

//not used yet :
//REL_CLI_DOWNSTREAM_DATA
//CLI_REL_DOWNSTREAM_NACK

type ALL_ALL_PARAMETERS struct {
	ClientDataOutputEnabled bool
	DoLatencyTests          bool
	DownCellSize            int
	ForceParams             bool
	NClients                int
	NextFreeClientId        int
	NextFreeTrusteeId       int
	NTrustees               int
	RelayDataOutputEnabled  bool
	RelayReportingLimit     int
	RelayUseDummyDataDown   bool
	RelayWindowSize         int
	StartNow                bool
	UpCellSize              int
	UseUDP                  bool
}

type CLI_REL_TELL_PK_AND_EPH_PK struct {
	Pk    abstract.Point
	EphPk abstract.Point
}

type CLI_REL_UPSTREAM_DATA struct {
	ClientId int
	RoundId  int32
	Data     []byte
}

type REL_CLI_DOWNSTREAM_DATA struct {
	RoundId    int32
	Data       []byte
	FlagResync bool
}

type REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG struct {
	Base         abstract.Point
	EphPks       []abstract.Point
	TrusteesSigs [][]byte
}

type REL_CLI_TELL_TRUSTEES_PK struct {
	Pks []abstract.Point
}

type REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE struct {
	Pks    []abstract.Point
	EphPks []abstract.Point
	Base   abstract.Point
}

type REL_TRU_TELL_TRANSCRIPT struct {
	G_s    []abstract.Point
	EphPks [][]abstract.Point
	Proofs [][]byte
}

type TRU_REL_DC_CIPHER struct {
	RoundId   int32
	TrusteeId int
	Data      []byte
}

type TRU_REL_SHUFFLE_SIG struct {
	TrusteeId int
	Sig       []byte
}

type TRU_REL_TELL_NEW_BASE_AND_EPH_PKS struct {
	NewBase   abstract.Point
	NewEphPks []abstract.Point
	Proof     []byte
}

type TRU_REL_TELL_PK struct {
	TrusteeId int
	Pk        abstract.Point
}

/**
 * This function must be called, on the correct host, with messages that are for him.
 * ie. if on this machine, prifi is the instance of a Relay protocol, any call to SendToRelay(m) on any machine
 * should eventually call ReceivedMessage(m) on this machine.
 */
func (prifi *PriFiProtocol) ReceivedMessage(msg interface{}) error {

	if prifi == nil {
		dbg.Print("Received a message ", msg)
		panic("But prifi is nil !")
	}

	var err error

	switch typedMsg := msg.(type) {
	case ALL_ALL_PARAMETERS:
		switch prifi.role {
		case PRIFI_ROLE_RELAY:
			err = prifi.Received_ALL_REL_PARAMETERS(typedMsg)
		case PRIFI_ROLE_CLIENT:
			err = prifi.Received_ALL_CLI_PARAMETERS(typedMsg)
		case PRIFI_ROLE_TRUSTEE:
			err = prifi.Received_ALL_TRU_PARAMETERS(typedMsg)
		default:
			panic("Received parameters, but we have no role yet !")
		}
	case CLI_REL_TELL_PK_AND_EPH_PK:
		err = prifi.Received_CLI_REL_TELL_PK_AND_EPH_PK(typedMsg)
	case CLI_REL_UPSTREAM_DATA:
		err = prifi.Received_CLI_REL_UPSTREAM_DATA(typedMsg)
	case REL_CLI_DOWNSTREAM_DATA:
		err = prifi.Received_REL_CLI_DOWNSTREAM_DATA(typedMsg)
	case REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG:
		err = prifi.Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(typedMsg)
	case REL_CLI_TELL_TRUSTEES_PK:
		err = prifi.Received_REL_CLI_TELL_TRUSTEES_PK(typedMsg)
	case REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE:
		err = prifi.Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE(typedMsg)
	case REL_TRU_TELL_TRANSCRIPT:
		err = prifi.Received_REL_TRU_TELL_TRANSCRIPT(typedMsg)
	case TRU_REL_DC_CIPHER:
		err = prifi.Received_TRU_REL_DC_CIPHER(typedMsg)
	case TRU_REL_SHUFFLE_SIG:
		err = prifi.Received_TRU_REL_SHUFFLE_SIG(typedMsg)
	case TRU_REL_TELL_NEW_BASE_AND_EPH_PKS:
		err = prifi.Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(typedMsg)
	case TRU_REL_TELL_PK:
		err = prifi.Received_TRU_REL_TELL_PK(typedMsg)
	default:
		panic("unrecognized message !")
	}

	//no need to push the error further up. display it here !
	if err != nil {
		dbg.Error("ReceivedMessage: got an error, " + err.Error())
		return err
	}

	return nil
}
