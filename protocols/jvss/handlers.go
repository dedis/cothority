package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

// SetupMsg are used for setting up new (long- and short-term) shared secrets
type SetupMsg struct {
	Src  int
	SID  string
	Deal []byte
}

// SigReqMsg are used to send signing requests
type SigReqMsg struct {
	Src int
	SID string
	Msg []byte
}

// SigRespMsg are used to reply to signing requests
type SigRespMsg struct {
	Src  int
	SID  string
	PSig *poly.SchnorrPartialSig
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

// WSigRespMsg is a SDA-wrapper around SigRespMsg
type WSigRespMsg struct {
	*sda.TreeNode
	SigRespMsg
}

func (jv *JVSS) handleSetup(m WSetupMsg) error {
	msg := m.SetupMsg

	// Initialise shared secret
	if err := jv.initSecret(msg.SID); err != nil {
		return err
	}

	// Unmarshal received deal
	d := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := d.UnmarshalBinary(msg.Deal); err != nil {
		return fmt.Errorf("Node %d could not unmarshal deal received from %d: %v", jv.nodeIdx(), msg.Src, err)
	}

	// Save received deal in the corresponding shared secret
	if err := jv.addDeal(msg.SID, d); err != nil {
		return err
	}

	// Finalise shared secret
	if err := jv.finaliseSecret(msg.SID); err != nil {
		return err
	}

	return nil
}

func (jv *JVSS) handleSigReq(m WSigReqMsg) error {
	msg := m.SigReqMsg

	// Create partial signature
	ps, err := jv.sigPartial(msg.SID, msg.Msg)
	if err != nil {
		return err
	}

	// Send it back to initiator
	resp := &SigRespMsg{
		Src:  jv.nodeIdx(),
		SID:  msg.SID,
		PSig: ps,
	}

	node := jv.nodeList[msg.Src]
	if err := jv.SendTo(node, resp); err != nil {
		return fmt.Errorf("Error sending msg to node %d: %v", msg.Src, err)
	}

	// Cleanup short-term shared secret
	delete(jv.secrets, msg.SID)

	return nil
}

func (jv *JVSS) handleSigResp(m WSigRespMsg) error {
	msg := m.SigRespMsg

	// Collect partial signatures
	if err := jv.schnorr.AddPartialSig(msg.PSig); err != nil {
		return err
	}
	sts := jv.secrets[msg.SID]
	sts.numPSigs++

	// Create Schnorr signature once we received enough replies
	if jv.info.T <= sts.numPSigs {
		sig, err := jv.schnorr.Sig()
		if err != nil {
			return fmt.Errorf("Error creating Schnorr signature: %v", err)
		}
		jv.sigChan <- sig

		// Cleanup short-term shared secret
		delete(jv.secrets, msg.SID)
	}

	return nil
}
