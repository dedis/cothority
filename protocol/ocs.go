package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"crypto/sha256"
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func init() {
	onet.GlobalProtocolRegister(NameOCS, NewOCS)
}

// OCS is only used to re-encrypt a public point. Before calling `Start`,
// DKG and U must be initialized by the caller.
type OCS struct {
	*onet.TreeNodeInstance
	Shared    *SharedSecret  // Shared represents the private key
	Poly      *share.PubPoly // Represents all public keys
	U         kyber.Point    // U is the encrypted secret
	Xc        kyber.Point    // The client's public key
	Threshold uint32
	// Done receives a 'true'-value when the protocol finished successfully.
	Done chan bool
	Uis  []*share.PubShare // re-encrypted shares
	// private fields
	replies []ReencryptReply
	replied bool
}

// NewOCS initialises the structure for use in one round
func NewOCS(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OCS{
		TreeNodeInstance: n,
		Done:             make(chan bool, 1),
		Threshold:        uint32(len(n.Roster().List) - (len(n.Roster().List)-1)/3),
	}

	err := o.RegisterHandlers(o.reencrypt, o.reencryptReply)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Start asks all children to reply with a shared reencryption
func (o *OCS) Start() error {
	log.Lvl3("Starting Protocol")
	if o.Shared == nil {
		return errors.New("please initialize Shared first")
	}
	if o.U == nil {
		return errors.New("please initialize U first")
	}
	errs := o.Broadcast(&Reencrypt{U: o.U, Xc: o.Xc})
	if len(errs) > (len(o.Roster().List)-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return errors.New("too many nodes failed in broadcast")
	}
	return nil
}

// Reencrypt is received by every node to give his part of
// the share
func (o *OCS) reencrypt(r structReencrypt) error {
	log.Lvl3(o.Name() + ": starting reencrypt")
	ui, err := o.getUI(r.U, r.Xc)
	if err != nil {
		return nil
	}

	// TODO: verify if the request is valid

	// Calculating proofs
	si := cothority.Suite.Scalar().Pick(o.Suite().RandomStream())
	uiHat := cothority.Suite.Point().Mul(si, cothority.Suite.Point().Add(r.U, r.Xc))
	hiHat := cothority.Suite.Point().Mul(si, nil)
	hash := sha256.New()
	ui.V.MarshalTo(hash)
	uiHat.MarshalTo(hash)
	hiHat.MarshalTo(hash)
	ei := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))

	return o.SendToParent(&ReencryptReply{
		Ui: ui,
		Ei: ei,
		Fi: cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, o.Shared.V)),
	})
}

// ReencryptReply is the root-node waiting for all replies and generating
// the reencryption key.
func (o *OCS) reencryptReply(rr structReencryptReply) error {
	if o.replied {
		log.Lvl3("not making reencryption reply, already did")
		return nil
	}
	o.replies = append(o.replies, rr.ReencryptReply)

	// minus one to exclude the root
	if len(o.replies) >= int(o.Threshold-1) {
		o.Uis = make([]*share.PubShare, len(o.List()))
		var err error
		o.Uis[0], err = o.getUI(o.U, o.Xc)
		if err != nil {
			return err
		}

		for _, r := range o.replies {
			// Verify proofs
			ufi := cothority.Suite.Point().Mul(r.Fi, cothority.Suite.Point().Add(o.U, o.Xc))
			uiei := cothority.Suite.Point().Mul(cothority.Suite.Scalar().Neg(r.Ei), r.Ui.V)
			uiHat := cothority.Suite.Point().Add(ufi, uiei)

			gfi := cothority.Suite.Point().Mul(r.Fi, nil)
			gxi := o.Poly.Eval(r.Ui.I).V
			hiei := cothority.Suite.Point().Mul(cothority.Suite.Scalar().Neg(r.Ei), gxi)
			hiHat := cothority.Suite.Point().Add(gfi, hiei)
			hash := sha256.New()
			r.Ui.V.MarshalTo(hash)
			uiHat.MarshalTo(hash)
			hiHat.MarshalTo(hash)
			e := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))
			if e.Equal(r.Ei) {
				o.Uis[r.Ui.I] = r.Ui
			} else {
				log.Lvl1("Received invalid share from node", r.Ui.I)
			}
		}
		o.Done <- true
		o.replied = true
	}
	return nil
}

func (o *OCS) getUI(U, Xc kyber.Point) (*share.PubShare, error) {
	v := cothority.Suite.Point().Mul(o.Shared.V, U)
	v.Add(v, cothority.Suite.Point().Mul(o.Shared.V, Xc))
	return &share.PubShare{
		I: o.Shared.Index,
		V: v,
	}, nil
}
