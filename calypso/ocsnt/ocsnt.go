package ocsnt

import (
	"crypto/sha256"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

func init() {
	_, err := onet.GlobalProtocolRegister(NameOCSNT, NewOCSNT)
	log.ErrFatal(err)
}

// OCSNT is only used to re-encrypt a public point. Before calling `Start`,
// DKG and U must be initialized by the caller.
type OCSNT struct {
	*onet.TreeNodeInstance
	Shared    *dkgprotocol.SharedSecret // Shared represents the private key
	Poly      *share.PubPoly            // Represents all public keys
	U         kyber.Point               // U is the encrypted secret
	Xc        kyber.Point               // The client's public key
	Total     int                       // Number of nodes in the LTS group
	Threshold int                       // How many replies are needed to re-create the secret
	// VerificationData is given to the VerifyRequest and has to hold everything
	// needed to verify the request is valid.
	VerificationData []byte
	Failures         int // How many failures occured so far
	// Can be set by the service to decide whether or not to
	// do the reencryption
	Verify VerifyPartialRequest
	// Reencrypted receives a 'true'-value when the protocol finished successfully,
	// or 'false' if not enough shares have been collected.
	Reencrypted chan bool
	Uis         []*share.PubShare // re-encrypted shares
	XhatEnc     kyber.Point
	// Unique identifier for the reencryption request
	DKID    string
	IsReenc bool
	// private fields
	ui         *share.PubShare
	isDone     bool
	replies    []PartialReencryptReply
	readyMesgs []ReadyReply
	timeout    *time.Timer
	doneOnce   sync.Once
}

// NewOCSNT initialises the structure for use in one round
func NewOCSNT(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	total := len(n.Roster().List)
	o := &OCSNT{
		TreeNodeInstance: n,
		Reencrypted:      make(chan bool, 1),
		Total:            total,
		Threshold:        total - (total-1)/3,
		isDone:           false,
	}
	err := o.RegisterHandlers(o.partialReencrypt, o.partialReencryptReply, o.readyReply)
	if err != nil {
		return nil, xerrors.Errorf("registring handlers: %v", err)
	}
	return o, nil
}

// Start asks all children to reply with a shared reencryption
func (o *OCSNT) Start() error {
	log.Lvl3("Starting Protocol")
	if o.Shared == nil {
		o.finish(false)
		return xerrors.New("please initialize Shared first")
	}
	if o.U == nil {
		o.finish(false)
		return xerrors.New("please initialize U first")
	}
	prc := &PartialReencrypt{
		IsReenc: o.IsReenc,
		DKID:    o.DKID,
		U:       o.U,
		Xc:      o.Xc,
	}
	if len(o.VerificationData) > 0 {
		prc.VerificationData = &o.VerificationData
	}
	if o.Verify != nil {
		if !o.Verify(prc) {
			o.finish(false)
			return xerrors.New("refused to reencrypt")
		}
	}
	o.timeout = time.AfterFunc(2*time.Minute, func() {
		log.Lvl1("OCSNT protocol timeout")
		o.finish(false)
	})
	errs := o.Broadcast(prc)
	if len(errs) > (o.Total-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast")
	}
	var xc kyber.Point
	if o.IsReenc {
		xc = o.Xc
	} else {
		xc = cothority.Suite.Point().Null()
	}
	log.LLvl4("XC IS", xc.String(), o.U.String())
	ui, ei, fi := o.generateShare(o.U, xc)
	o.ui = ui
	errs = o.Broadcast(&PartialReencryptReply{
		Ui: ui,
		Ei: ei,
		Fi: fi,
	})
	if len(errs) > (o.Total-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast PartialReencryptReply")
	}
	return nil
}

// PartialReencrypt is received by every node to give his part of
// the share
func (o *OCSNT) partialReencrypt(prc structPartialReencrypt) error {
	log.LLvl3(o.Name() + ": starting partialreencrypt")
	//defer o.Done()

	total := len(o.Roster().List)
	o.Total = total
	o.Threshold = total - (total-1)/3
	o.DKID = prc.PartialReencrypt.DKID
	o.U = prc.U
	o.Xc = prc.Xc

	if o.Verify != nil {
		if !o.Verify(&prc.PartialReencrypt) {
			log.Lvl2(o.ServerIdentity(), "refused to do the partial reencryption")
			errs := o.Broadcast(&PartialReencryptReply{})
			if len(errs) > (o.Total-1)/3 {
				log.Errorf("Some nodes failed with error(s) %v", errs)
				return xerrors.New("too many nodes failed in broadcast empty PartialReencryptReply")
			}
			return nil
		}
	}

	var xc kyber.Point
	if prc.IsReenc {
		xc = o.Xc
	} else {
		xc = cothority.Suite.Point().Null()
	}
	log.LLvl4("XC IS", xc.String(), o.U.String())
	ui, ei, fi := o.generateShare(o.U, xc)
	o.ui = ui
	//ui := o.getUI(o.U, o.Xc)
	//// Calculating proofs
	//si := cothority.Suite.Scalar().Pick(o.Suite().RandomStream())
	//uiHat := cothority.Suite.Point().Mul(si, cothority.Suite.Point().Add(o.U, o.Xc))
	//hiHat := cothority.Suite.Point().Mul(si, nil)
	//hash := sha256.New()
	//ui.V.MarshalTo(hash)
	//uiHat.MarshalTo(hash)
	//hiHat.MarshalTo(hash)
	//ei := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))

	errs := o.Broadcast(&PartialReencryptReply{
		Ui: ui,
		Ei: ei,
		Fi: fi,
		//Fi: cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, o.Shared.V)),
	})
	if len(errs) > (o.Total-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast PartialReencryptReply")
	}
	return nil
}

func (o *OCSNT) partialReencryptReply(prr structPartialReencryptReply) error {
	log.LLvl3(o.ServerIdentity(), "received", prr.PartialReencryptReply.Ei.String(), "from", prr.ServerIdentity)
	if prr.PartialReencryptReply.Ui == nil {
		log.Lvl2("Node", prr.ServerIdentity, "refused to reply")
		o.Failures++
		if o.Failures > o.Total-o.Threshold {
			log.Lvl2(prr.ServerIdentity, "couldn't get enough shares")
			o.finish(false)
		}
		return nil
	}
	o.replies = append(o.replies, prr.PartialReencryptReply)

	// minus one to exclude myself
	if !o.isDone && len(o.replies) >= int(o.Threshold-1) {
		o.Uis = make([]*share.PubShare, len(o.List()))
		//o.Uis[0] = o.getUI(o.U, xc)
		log.LLvl3(o.ServerIdentity(), "MY UI IS", o.ui)
		o.Uis[o.ui.I] = o.ui
		var xc kyber.Point
		if o.IsReenc {
			xc = o.Xc
		} else {
			xc = cothority.Suite.Point().Null()
		}

		log.LLvl4("XC IS", xc.String(), o.U.String())
		for _, r := range o.replies {
			// Verify proofs
			ufi := cothority.Suite.Point().Mul(r.Fi, cothority.Suite.Point().Add(o.U, xc))
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
				log.LLvl3("RECEIVED INVALID SHARE FROM", r.Ui.I)
				log.Lvl1("Received invalid share from node", r.Ui.I)
			}
		}
		xhatEnc, err := share.RecoverCommit(cothority.Suite, o.Uis, o.Threshold, len(o.Roster().List))
		if err != nil {
			log.Errorf("%s couldn't recover secret", prr.ServerIdentity)
			o.finish(false)
		} else {
			o.XhatEnc = xhatEnc
			o.isDone = true
			log.LLvl3(o.ServerIdentity(), "computed xhatenc:", o.XhatEnc.String())
			if !o.IsRoot() {
				err := o.SendToParent(&ReadyReply{})
				if err != nil {
					log.Errorf("%s: error sending readyreply to parent", o.ServerIdentity())
				}
				o.finish(true)
			}
			// If root don't finish here!
			//o.finish(true)
		}
	}

	// If we are leaving by here it means that we do not have
	// enough replies yet. We must eventually trigger a finish()
	// somehow. It will either happen because we get another
	// reply, and now we have enough, or because we get enough
	// failures and know to give up, or because o.timeout triggers
	// and calls finish(false) in it's callback function.

	return nil
}

func (o *OCSNT) readyReply(rr structReadyReply) error {
	log.LLvl3(o.ServerIdentity(), "received readyReply from", rr.ServerIdentity)
	o.readyMesgs = append(o.readyMesgs, rr.ReadyReply)
	// minus one to exclude myself
	if len(o.replies) >= int(o.Threshold-1) {
		log.LLvl2(o.ServerIdentity(), "collected sufficient number of ready messages")
		o.finish(true)
	}
	return nil
}

func (o *OCSNT) generateShare(U, Xc kyber.Point) (*share.PubShare, kyber.Scalar, kyber.Scalar) {
	ui := o.getUI(U, Xc)
	// Calculating proofs
	si := cothority.Suite.Scalar().Pick(o.Suite().RandomStream())
	uiHat := cothority.Suite.Point().Mul(si, cothority.Suite.Point().Add(U, Xc))
	hiHat := cothority.Suite.Point().Mul(si, nil)
	hash := sha256.New()
	ui.V.MarshalTo(hash)
	uiHat.MarshalTo(hash)
	hiHat.MarshalTo(hash)
	ei := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))
	fi := cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, o.Shared.V))
	return ui, ei, fi
}

func (o *OCSNT) getUI(U, Xc kyber.Point) *share.PubShare {
	v := cothority.Suite.Point().Mul(o.Shared.V, U)
	v.Add(v, cothority.Suite.Point().Mul(o.Shared.V, Xc))
	return &share.PubShare{
		I: o.Shared.Index,
		V: v,
	}
}

func (o *OCSNT) finish(result bool) {
	if o.IsRoot() {
		o.timeout.Stop()
	}
	select {
	case o.Reencrypted <- result:
		// succeeded
	default:
		// would have blocked because some other call to finish()
		// beat us.
	}
	o.doneOnce.Do(func() { o.Done() })
}
