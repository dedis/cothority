package timevault

import (
	"fmt"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
)

// SecInitMsg are used to initialise new shared secrets.
type SecInitMsg struct {
	Src      int
	SID      SID
	Deal     []byte
	Duration time.Duration
}

// SecConfMsg are used to confirm to other peers that we have finished setting
// up the shared secret.
type SecConfMsg struct {
	Src int
	SID SID
}

// RevInitMsg is used to prompt peers to reveal their shares
type RevInitMsg struct {
	Src int
	SID SID
}

// RevShareMsg is used to reveal the secret share of the given peer
type RevShareMsg struct {
	Src   int
	SID   SID
	Share *abstract.Secret
	Index int
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

// WRevInitMsg is a SDA-wrapper around RevInitMsg
type WRevInitMsg struct {
	*sda.TreeNode
	RevInitMsg
}

// WRevShareMsg is a SDA-wrapper around RevShareMsg
type WRevShareMsg struct {
	*sda.TreeNode
	RevShareMsg
}

func (tv *TimeVault) handleSecInit(m WSecInitMsg) error {
	msg := m.SecInitMsg

	// Initialise shared secret
	if err := tv.initSecret(msg.SID, msg.Duration); err != nil {
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

	dbg.Lvl2(fmt.Sprintf("Node %d: %s confirmations %d/%d", tv.TreeNodeInstance.Index(), msg.SID, secret.numConfs, len(tv.TreeNodeInstance.List())))

	// Check if we have enough confirmations to proceed
	if secret.numConfs == len(tv.TreeNodeInstance.List()) {
		tv.secretsDone <- true
	}

	return nil
}

func (tv *TimeVault) handleRevInit(m WRevInitMsg) error {
	msg := m.RevInitMsg

	secret, ok := tv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	secret.mtx.Lock()
	defer secret.mtx.Unlock()
	if !secret.expired {
		return fmt.Errorf("Error, secret has not yet expired")
	}

	reply := &RevShareMsg{
		Src:   tv.TreeNodeInstance.Index(),
		SID:   msg.SID,
		Share: secret.secret.Share,
		Index: secret.secret.Index,
	}
	return tv.TreeNodeInstance.SendTo(tv.TreeNodeInstance.List()[msg.Src], reply)

}

func (tv *TimeVault) handleRevShare(m WRevShareMsg) error {
	msg := m.RevShareMsg

	rs := tv.recoveredSecrets[msg.SID]
	rs.priShares.SetShare(msg.Index, *msg.Share)
	rs.mtx.Lock()
	rs.numShares++
	rs.mtx.Unlock()
	dbg.Lvl2(fmt.Sprintf("Node %d: %s shares %d/%d", tv.TreeNodeInstance.Index(), msg.SID, rs.numShares, len(tv.TreeNodeInstance.List())))
	if rs.numShares == tv.info.T {
		sec := rs.priShares.Secret()
		rs.secretsChan <- sec
	}

	return nil
}
