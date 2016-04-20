package timevault

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
	SID  SID
	Deal []byte
}

// SecConfMsg are used to confirm to other peers that we have finished setting
// up the shared secret.
type SecConfMsg struct {
	Src int
	SID SID
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

func (tv *TimeVault) handleSecInit(m WSecInitMsg) error {
	msg := m.SecInitMsg

	// Initialise shared secret
	if err := tv.initSecret(msg.SID); err != nil {
		return err
	}

	// Unmarshal received deal
	deal := new(poly.Deal).UnmarshalInit(tv.info.T, tv.info.R, tv.info.N, tv.keyPair.Suite)
	if err := deal.UnmarshalBinary(msg.Deal); err != nil {
		return err
	}

	// Buffer received deal for later
	secret, ok := tv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}
	secret.deals[msg.Src] = deal

	// Finalise shared secret
	if err := tv.finaliseSecret(msg.SID); err != nil {
		return err
	}

	return nil
}

func (tv *TimeVault) handleSecConf(m WSecConfMsg) error {
	msg := m.SecConfMsg

	secret, ok := tv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	secret.mtx.Lock()
	secret.numConfs++
	secret.mtx.Unlock()

	dbg.Lvl2(fmt.Sprintf("Node %d: %s confirmations %d/%d", tv.Node.Index(), msg.SID, secret.numConfs, len(tv.Node.List())))

	// Check if we have enough confirmations to proceed
	s0 := SID(fmt.Sprintf("%s-0-%d", TVSS, tv.Node.Index()))
	s1 := SID(fmt.Sprintf("%s-1-%d", TVSS, tv.Node.Index()))
	if (secret.numConfs == len(tv.Node.List())) && ((msg.SID == s0) || (msg.SID == s1)) {
		tv.secretsDone <- true
	}

	return nil
}
