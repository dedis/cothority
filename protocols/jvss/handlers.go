package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

type SetupMsg struct {
	Src  int
	Deal []byte
}

// WMsgSetup is a SDA-wrapper around R4
type WSetupMsg struct {
	*sda.TreeNode
	SetupMsg
}

func (jv *JVSS) handleSetup(m WSetupMsg) error {
	msg := m.SetupMsg

	// Initialise long-term shared secret
	jv.initSecret(jv.ltSecret)

	// Unmarshal received deal and store it in the long-term shared secret
	d := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := d.UnmarshalBinary(msg.Deal); err != nil {
		return fmt.Errorf("Node %d could not unmarshal deal received from %d: %v", jv.nodeIdx(), msg.Src, err)
	}
	jv.addDeal(jv.ltSecret, jv.nodeIdx(), d)

	// Finalise long-term shared secret
	jv.finaliseSecret(jv.ltSecret)

	return nil
}

func (jv *JVSS) handleSigReq() error { return nil }
func (jv *JVSS) handleVerReq() error { return nil }
