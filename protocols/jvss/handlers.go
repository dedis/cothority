package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

// SecInitMsg are used to initialise new shared secrets both long- and
// short-term.
type SecInitMsg struct {
	Src  int
	SID  string
	Deal []byte
}

// SecConfMsg are used to confirm to other peers that we have finished setting
// up the shared secret.
type SecConfMsg struct {
	Src int
	SID string
}

// SigReqMsg are used to send signing requests.
type SigReqMsg struct {
	Src int
	SID string
	Msg []byte
}

// SigRespMsg are used to reply to signing requests.
type SigRespMsg struct {
	Src int
	SID string
	Sig *poly.SchnorrPartialSig
}

// WSecInitMsg is a SDA-wrapper around SecInitMsg.
type WSecInitMsg struct {
	*sda.TreeNode
	SecInitMsg
}

// WSecConfMsg is a SDA-wrapper around SecConfMsg.
type WSecConfMsg struct {
	*sda.TreeNode
	SecConfMsg
}

// WSigReqMsg is a SDA-wrapper around SigReqMsg.
type WSigReqMsg struct {
	*sda.TreeNode
	SigReqMsg
}

// WSigRespMsg is a SDA-wrapper around SigRespMsg.
type WSigRespMsg struct {
	*sda.TreeNode
	SigRespMsg
}

func (jv *JVSS) handleSecInit(m WSecInitMsg) error {
	msg := m.SecInitMsg

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

func (jv *JVSS) handleSecConf(m WSecConfMsg) error {
	msg := m.SecConfMsg

	secret := jv.secrets[msg.SID]
	secret.mtx.Lock()
	secret.numConfs++
	secret.mtx.Unlock()

	// Check if we have enough confirmations to proceed
	jv.checkConfirmations(secret.numConfs, len(jv.nodeList), msg.SID)
	dbg.Lvl1(fmt.Sprintf("Node %d: %s confirmations %d/%d", jv.nodeIdx(), msg.SID, secret.numConfs, len(jv.nodeList)))

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
		Src: jv.nodeIdx(),
		SID: msg.SID,
		Sig: ps,
	}

	dbg.Lvl1(fmt.Sprintf("Node %d: sending partial signature ...", jv.nodeIdx()))
	node := jv.nodeList[msg.Src]
	if err := jv.SendTo(node, resp); err != nil {
		return fmt.Errorf("Error sending msg to node %d: %v", msg.Src, err)
	}
	dbg.Lvl1(fmt.Sprintf("Node %d: ... done", jv.nodeIdx()))

	// Cleanup short-term shared secret
	delete(jv.secrets, msg.SID)

	return nil
}

func (jv *JVSS) handleSigResp(m WSigRespMsg) error {
	msg := m.SigRespMsg

	// Collect partial signatures
	if err := jv.schnorr.AddPartialSig(msg.Sig); err != nil {
		dbg.Lvl1(fmt.Sprintf("Error Node %d: %s signatures (from %d)", jv.nodeIdx(), msg.SID, msg.Src))
		dbg.Lvl1(err)
		return err
	}
	secret := jv.secrets[msg.SID]
	secret.numSigs++
	dbg.Lvl1(fmt.Sprintf("Node %d: %s signatures %d/%d (from %d)", jv.nodeIdx(), msg.SID, secret.numSigs, len(jv.nodeList), msg.Src))

	// Create Schnorr signature once we received enough replies
	if jv.info.T <= secret.numSigs {
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
