package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

type SetupMsg struct {
	Src  int
	SID  string
	Deal []byte
}

type SigReqMsg struct {
	Src int
	SID string
	Msg []byte
}

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
	jv.initSecret(msg.SID)

	// Unmarshal received deal and store it in the shared secret
	d := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := d.UnmarshalBinary(msg.Deal); err != nil {
		return fmt.Errorf("Node %d could not unmarshal deal received from %d: %v", jv.nodeIdx(), msg.Src, err)
	}
	jv.addDeal(msg.SID, d)

	// Finalise shared secret
	jv.finaliseSecret(msg.SID)

	return nil
}

func (jv *JVSS) handleSigReq(m WSigReqMsg) error {
	msg := m.SigReqMsg

	// create and send reply with partial signature back
	resp := &SigRespMsg{
		Src:  jv.nodeIdx(),
		SID:  msg.SID,
		PSig: jv.sigPartial(msg.SID, msg.Msg),
	}

	node := jv.nodeList[msg.Src]
	if err := jv.SendTo(node, resp); err != nil {
		return fmt.Errorf("Error sending msg to node %d: %v", msg.Src, err)
	}

	// cleanup short-term shared secret
	delete(jv.secrets, msg.SID)

	return nil
}

func (jv *JVSS) handleSigResp(m WSigRespMsg) error {
	msg := m.SigRespMsg

	// collect partial signatures
	if err := jv.schnorr.AddPartialSig(msg.PSig); err != nil {
		return err
	}
	sts := jv.secrets[msg.SID]
	sts.numPSigs++

	// create Schnorr signature once we received enough replies
	if jv.info.T <= sts.numPSigs {
		sig, err := jv.schnorr.Sig()
		if err != nil {
			return fmt.Errorf("Error creating Schnorr signature: %v", err)
		}
		jv.sigChan <- sig

		// cleanup short-term shared secret
		delete(jv.secrets, msg.SID)
	}

	return nil
}

func (jv *JVSS) handleVerReq() error { return nil }
