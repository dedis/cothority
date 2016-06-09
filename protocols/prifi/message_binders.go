package prifi

func (p *PriFiSDAWrapper) Received_ALL_ALL_PARAMETERS(msg Struct_ALL_ALL_PARAMETERS) error {
	return p.prifiProtocol.ReceivedMessage(msg.ALL_ALL_PARAMETERS)
}

//client handlers
func (p *PriFiSDAWrapper) Received_REL_CLI_DOWNSTREAM_DATA(msg Struct_REL_CLI_DOWNSTREAM_DATA) error {
	return p.prifiProtocol.ReceivedMessage(msg.REL_CLI_DOWNSTREAM_DATA)
}
func (p *PriFiSDAWrapper) Received_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG(msg Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG) error {
	return p.prifiProtocol.ReceivedMessage(msg.REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG)
}
func (p *PriFiSDAWrapper) Received_REL_CLI_TELL_TRUSTEES_PK(msg Struct_REL_CLI_TELL_TRUSTEES_PK) error {
	return p.prifiProtocol.ReceivedMessage(msg.REL_CLI_TELL_TRUSTEES_PK)
}

//relay handlers
func (p *PriFiSDAWrapper) Received_CLI_REL_TELL_PK_AND_EPH_PK(msg Struct_CLI_REL_TELL_PK_AND_EPH_PK) error {
	return p.prifiProtocol.ReceivedMessage(msg.CLI_REL_TELL_PK_AND_EPH_PK)
}
func (p *PriFiSDAWrapper) Received_CLI_REL_UPSTREAM_DATA(msg Struct_CLI_REL_UPSTREAM_DATA) error {
	return p.prifiProtocol.ReceivedMessage(msg.CLI_REL_UPSTREAM_DATA)
}
func (p *PriFiSDAWrapper) Received_TRU_REL_DC_CIPHER(msg Struct_TRU_REL_DC_CIPHER) error {
	return p.prifiProtocol.ReceivedMessage(msg.TRU_REL_DC_CIPHER)
}
func (p *PriFiSDAWrapper) Received_TRU_REL_SHUFFLE_SIG(msg Struct_TRU_REL_SHUFFLE_SIG) error {
	return p.prifiProtocol.ReceivedMessage(msg.TRU_REL_SHUFFLE_SIG)
}
func (p *PriFiSDAWrapper) Received_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS(msg Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS) error {
	return p.prifiProtocol.ReceivedMessage(msg.TRU_REL_TELL_NEW_BASE_AND_EPH_PKS)
}
func (p *PriFiSDAWrapper) Received_TRU_REL_TELL_PK(msg Struct_TRU_REL_TELL_PK) error {
	return p.prifiProtocol.ReceivedMessage(msg.TRU_REL_TELL_PK)
}

//trustees handlers
func (p *PriFiSDAWrapper) Received_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE(msg Struct_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE) error {
	return p.prifiProtocol.ReceivedMessage(msg.REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE)
}
func (p *PriFiSDAWrapper) Received_REL_TRU_TELL_TRANSCRIPT(msg Struct_REL_TRU_TELL_TRANSCRIPT) error {
	return p.prifiProtocol.ReceivedMessage(msg.REL_TRU_TELL_TRANSCRIPT)
}
