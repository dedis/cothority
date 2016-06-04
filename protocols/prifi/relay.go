package prifi

import (
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

//Messages to handle :
//CLI_REL_TELL_PK_AND_EPH_PK
//CLI_REL_UPSTREAM_DATA
//TRU_REL_DC_CIPHER
//TRU_REL_SHUFFLE_SIG
//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
//TRU_REL_TELL_PK

func (p *PriFiProtocolHandlers) Received_CLI_REL_TELL_PK_AND_EPH_PK(msg Struct_CLI_REL_TELL_PK_AND_EPH_PK) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_CLI_REL_UPSTREAM_DATA(msg Struct_CLI_REL_UPSTREAM_DATA) error {

	receivedNo := msg.RoundId

	dbg.Lvl1("I'm", p.Name())
	dbg.Lvl1("I received the CLI_REL_UPSTREAM_DATA with content", receivedNo)

	time.Sleep(1000 * time.Millisecond)

	toSend := &REL_CLI_DOWNSTREAM_DATA{receivedNo + 1, make([]byte, 0)}

	for _, c := range p.Children() {
		dbg.Lvl1("I'm", p.Name(), ", sending REL_CLI_DOWNSTREAM_DATA to ", c.Entity.Public)
		err := p.SendTo(c, toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_DC_CIPHER(msg Struct_TRU_REL_DC_CIPHER) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_SHUFFLE_SIG(msg Struct_TRU_REL_SHUFFLE_SIG) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(msg Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS) error {

	return nil
}

func (p *PriFiProtocolHandlers) Received_TRU_REL_TELL_PK(msg Struct_TRU_REL_TELL_PK) error {

	return nil
}
