package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"go.dedis.ch/cothority/v4"
	dkgprotocol "go.dedis.ch/cothority/v4/dkg/pedersen"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

func init() {
	onet.GlobalProtocolRegister(NameOCS, NewOCS)
}

// OCS is only used to re-encrypt a public point. Before calling `Start`,
// DKG and U must be initialized by the caller.
type OCS struct {
	*onet.TreeNodeInstance
	Shared    *dkgprotocol.SharedSecret // Shared represents the private key
	Poly      *share.PubPoly            // Represents all public keys
	U         kyber.Point               // U is the encrypted secret
	Xc        kyber.Point               // The client's public key
	Threshold int                       // How many replies are needed to re-create the secret
	// VerificationData is given to the VerifyRequest and has to hold everything
	// needed to verify the request is valid.
	VerificationData []byte
	Failures         int // How many failures occured so far
	// Can be set by the service to decide whether or not to
	// do the reencryption
	Verify VerifyRequest
	// Reencrypted receives a 'true'-value when the protocol finished successfully,
	// or 'false' if not enough shares have been collected.
	Reencrypted chan bool
	Uis         []*share.PubShare // re-encrypted shares
	// private fields
	replies  []ReencryptReply
	timeout  *time.Timer
	doneOnce sync.Once
}

// NewOCS initialises the structure for use in one round
func NewOCS(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OCS{
		TreeNodeInstance: n,
		Reencrypted:      make(chan bool, 1),
		Threshold:        len(n.Roster().List) - (len(n.Roster().List)-1)/3,
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
		o.finish(false)
		return errors.New("please initialize Shared first")
	}
	if o.U == nil {
		o.finish(false)
		return errors.New("please initialize U first")
	}
	rc := &Reencrypt{
		U:  o.U,
		Xc: o.Xc,
	}
	if len(o.VerificationData) > 0 {
		rc.VerificationData = &o.VerificationData
	}
	if o.Verify != nil {
		if !o.Verify(rc) {
			o.finish(false)
			return errors.New("refused to reencrypt")
		}
	}
	o.timeout = time.AfterFunc(1*time.Minute, func() {
		log.Lvl1("OCS protocol timeout")
		o.finish(false)
	})
	errs := o.Broadcast(rc)
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
	defer o.Done()

	ui, err := o.getUI(r.U, r.Xc)
	if err != nil {
		return nil
	}

	if o.Verify != nil {
		if !o.Verify(&r.Reencrypt) {
			log.Lvl2(o.ServerIdentity(), "refused to reencrypt")
			return o.SendToParent(&ReencryptReply{})
		}
	}

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

// reencryptReply is the root-node waiting for all replies and generating
// the reencryption key.
func (o *OCS) reencryptReply(rr structReencryptReply) error {
	if rr.ReencryptReply.Ui == nil {
		log.Lvl2("Node", rr.ServerIdentity, "refused to reply")
		o.Failures++
		if o.Failures > len(o.Roster().List)-o.Threshold {
			log.Lvl2(rr.ServerIdentity, "couldn't get enough shares")
			o.finish(false)
		}
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
		o.finish(true)
	}

	// If we are leaving by here it means that we do not have
	// enough replies yet. We must eventually trigger a finish()
	// somehow. It will either happen because we get another
	// reply, and now we have enough, or because we get enough
	// failures and know to give up, or because o.timeout triggers
	// and calls finish(false) in it's callback function.

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

func (o *OCS) finish(result bool) {
	o.timeout.Stop()
	select {
	case o.Reencrypted <- result:
		// suceeded
	default:
		// would have blocked because some other call to finish()
		// beat us.
	}
	o.doneOnce.Do(func() { o.Done() })
}
