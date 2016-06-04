package prifi

import (
	"math/rand"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

var clientState int32 = 0

//Messages to handle :
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK

func (p *PriFiProtocolHandlers) Received_REL_CLI_DOWNSTREAM_DATA(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl2("I'm", p.Name())
	dbg.Lvl2("I received the REL_CLI_DOWNSTREAM_DATA with content", receivedNo)

	if clientState == 0 {
		clientState = int32(rand.Intn(10000))
		dbg.Lvl2("I'm", p.Name(), ", setting clientstate to ", clientState)
	} else {
		dbg.Lvl2("I'm", p.Name(), ", keeping clientstate at ", clientState)
	}

	toSend := &CLI_REL_UPSTREAM_DATA{clientState, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("I'm", p.Name(), ", sending CLI_REL_UPSTREAM_DATA with clientState ", clientState)

	return p.SendTo(p.Parent(), toSend)
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_TRUSTEES_PK(msg Struct_REL_CLI_TELL_TRUSTEES_PK) error {

	return nil
}
