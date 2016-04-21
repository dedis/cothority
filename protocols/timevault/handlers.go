package timevault

import (
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
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
	if (secret.numConfs == len(tv.Node.List())) && (msg.SID == SID(fmt.Sprintf("%s%d", TVSS, tv.Node.Index()))) {
		tv.secretsDone <- true
	}

	return nil
}

func (tv *TimeVault) handleRevInit(m WRevInitMsg) error {
	msg := m.RevInitMsg

	// TODO: Authenticity of this query should be checked to prevent that anybody can trigger share revealing
	s, ok := tv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	reply := &RevShareMsg{
		Src:   tv.Node.Index(),
		SID:   msg.SID,
		Share: s.secret.Share,
		Index: s.secret.Index,
	}
	return tv.Node.SendTo(tv.Node.List()[msg.Src], reply)

}

func (tv *TimeVault) handleRevShare(m WRevShareMsg) error {
	msg := m.RevShareMsg

	rs := tv.recoveredSecrets[msg.SID]
	rs.PriShares.SetShare(msg.Index, *msg.Share)
	rs.mtx.Lock()
	rs.NumShares++
	rs.mtx.Unlock()
	dbg.Lvl2(fmt.Sprintf("Node %d: %s shares %d/%d", tv.Node.Index(), msg.SID, rs.NumShares, len(tv.Node.List())))
	if rs.NumShares == tv.info.T {
		sec := rs.PriShares.Secret()
		tv.secretsChan <- sec
	}

	return nil
}
