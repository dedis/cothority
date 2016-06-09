package prifi

import (
	"github.com/dedis/cothority/lib/dbg"
)

/**
 * PriFi - Library
 * ***************
 * This is a network-agnostic PriFi library. Feed it with a MessageSender interface (that knows how to contact the different entities),
 * and call ReceivedMessage(msg) with the received messages.
 * Then, it runs the PriFi anonymous communication network among those entities.
 */

//the mutable variable held by this entity
type PriFiProtocol struct {
	role          int16
	messageSender MessageSender
	clientState   ClientState  //only one of those will be set
	relayState    RelayState   //only one of those will be set
	trusteeState  TrusteeState //only one of those will be set
}

// possible role this entity are in. This restrict the kind of messages it can receive at a given point (roles are mutually exclusive)
const (
	PRIFI_ROLE_UNDEFINED int16 = iota
	PRIFI_ROLE_RELAY
	PRIFI_ROLE_CLIENT
	PRIFI_ROLE_TRUSTEE
)

//this is the interface we need to give this library for it to work.
type MessageSender interface {

	/**
	 * This should deliver the message "msg" to the client i.
	 */
	SendToClient(i int, msg interface{}) error

	/**
	 * This should deliver the message "msg" to the trustee i.
	 */
	SendToTrustee(i int, msg interface{}) error

	/**
	 * This should deliver the message "msg" to the relay.
	 */
	SendToRelay(msg interface{}) error
}

/*
 * call the functions below on the appropriate machine on the network.
 * if you call *without state* (one of the first 3 methods), IT IS NOT SUFFICIENT FOR PRIFI to start; this entity will expect a ALL_ALL_PARAMETERS as a
 * first message to finish initializing itself (this is handly if only the Relay has access to the configuration file).
 * Otherwise, the 3 last methods fully initialize the entity.
 */

func NewPriFiRelay(msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_RELAY,
		messageSender: msgSender,
	}

	return &prifi
}

func NewPriFiClient(msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_CLIENT,
		messageSender: msgSender,
	}
	return &prifi
}

func NewPriFiTrustee(msgSender MessageSender) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_TRUSTEE,
		messageSender: msgSender,
	}
	return &prifi
}

func NewPriFiRelayWithState(msgSender MessageSender, state *RelayState) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_RELAY,
		messageSender: msgSender,
		relayState:    *state,
	}

	dbg.Lvl1("Relay has been initialized by function call. ")
	return &prifi
}

func NewPriFiClientWithState(msgSender MessageSender, state *ClientState) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_CLIENT,
		messageSender: msgSender,
		clientState:   *state,
	}
	dbg.Lvl1("Client has been initialized by function call. ")
	return &prifi
}

func NewPriFiTrusteeWithState(msgSender MessageSender, state *TrusteeState) *PriFiProtocol {
	prifi := PriFiProtocol{
		role:          PRIFI_ROLE_TRUSTEE,
		messageSender: msgSender,
		trusteeState:  *state,
	}

	dbg.Lvl1("Trustee has been initialized by function call. ")
	return &prifi
}

//debug. Prints role
func (prifi *PriFiProtocol) WhoAmI() {

	dbg.Print("###################### WHO AM I ######################")
	if prifi.role == PRIFI_ROLE_RELAY {
		dbg.Print("I' a relay, my name is ", prifi.relayState.Name)
		dbg.Printf("%+v\n", prifi.relayState)
		//dbg.Print("I'm not : ")
		//dbg.Printf("%+v\n", prifi.clientState)
		//dbg.Printf("%+v\n", prifi.trusteeState)
	} else if prifi.role == PRIFI_ROLE_CLIENT {
		dbg.Print("I' a client, my name is ", prifi.clientState.Name)
		dbg.Printf("%+v\n", prifi.clientState)
		//dbg.Print("I'm not : ")
		//dbg.Printf("%+v\n", prifi.relayState)
		//dbg.Printf("%+v\n", prifi.trusteeState)
	} else if prifi.role == PRIFI_ROLE_TRUSTEE {
		dbg.Print("I' a trustee, my name is ", prifi.trusteeState.Name)
		dbg.Printf("%+v\n", prifi.trusteeState)
		//dbg.Print("I'm not : ")
		//dbg.Printf("%+v\n", prifi.clientState)
		//dbg.Printf("%+v\n", prifi.relayState)
	}
	dbg.Print("###################### -------- ######################")
}
