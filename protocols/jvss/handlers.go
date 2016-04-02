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

	// Create and broadcast our own deal if not done so before
	jv.setupDeal()

	// Unmarshal received deal and store it
	d := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := d.UnmarshalBinary(msg.Deal); err != nil {
		return fmt.Errorf("Node %d could not unmarshal deal received from %d: %v", jv.nodeIdx(), msg.Src, err)
	}
	jv.addDeal(jv.nodeIdx(), d) // TODO: why give jv.nodeIdx()?

	// Try to setup the shared secret
	jv.setupSharedSecret()

	return nil
}
