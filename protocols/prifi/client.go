package prifi

import (
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

//Messages to handle :
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK

func (p *PriFiProtocolHandlers) Received_REL_CLI_DOWNSTREAM_DATA(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl2("I'm", p.Name())
	dbg.Lvl2("I received the REL_CLI_DOWNSTREAM_DATA with content", receivedNo)

	toSend := &CLI_REL_UPSTREAM_DATA{receivedNo + 1, make([]byte, 0)}

	time.Sleep(1000 * time.Millisecond)

	dbg.Lvl2("I'm", p.Entity().Public, ", sending CLI_REL_UPSTREAM_DATA to ", p.Parent().Entity.String)

	return p.SendTo(p.Parent(), toSend)
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_REL_CLI_TELL_TRUSTEES_PK(msg Struct_REL_CLI_TELL_TRUSTEES_PK) error {

	return nil
}
