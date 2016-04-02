package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

type SetupMsg struct {
	Src  int
	Type string
	Deal []byte
}

type SigReqMsg struct {
	Src  int
	Type string
	Msg  []byte
}

// WSetupMsg is a SDA-wrapper around SetupMsg
type WSetupMsg struct {
	*sda.TreeNode
	SetupMsg
}

// WSigReqMsg is a SDA-wrapper around SigReqMsg
type WSigReqMsg struct {
	*sda.TreeNode
	SigReqMsg
}

func (jv *JVSS) handleSetup(m WSetupMsg) error {
	msg := m.SetupMsg

	// Initialise shared secret
	jv.initSecret(msg.Type)

	// Unmarshal received deal and store it in the shared secret
	d := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := d.UnmarshalBinary(msg.Deal); err != nil {
		return fmt.Errorf("Node %d could not unmarshal deal received from %d: %v", jv.nodeIdx(), msg.Src, err)
	}
	jv.addDeal(msg.Type, d)

	// Finalise shared secret
	jv.finaliseSecret(msg.Type)

	return nil
}

func (jv *JVSS) handleSigReq(m WSigReqMsg) error {
	msg := m.SigReqMsg

	ps := jv.sigPartial(msg.Type, msg.Msg)
	_ = ps

	dbg.Lvl1(fmt.Sprintf("Node %d: received signing request", jv.nodeIdx()))

	// send back

	return nil
}

func (jv *JVSS) handleVerReq() error { return nil }
