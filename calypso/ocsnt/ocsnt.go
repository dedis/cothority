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
	ui      *share.PubShare
	isDone  bool
	replies []PartialReencryption
	//readyMesgs []Ready
	SuccessCount int
	FailCount    int
	timeout      *time.Timer
	doneOnce     sync.Once
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
	err := o.RegisterHandlers(o.reencrypt, o.reencryptReply, o.ready)
	if err != nil {
		return nil, xerrors.Errorf("registring handlers: %v", err)
	}
	//o.timeout = time.AfterFunc(1*time.Minute, func() {
	//log.Lvl1("OCSNT protocol timeout")
	//o.finish(false)
	//})
	return o, nil
}

// Start asks all children to reply with a shared reencryption
func (o *OCSNT) Start() error {
	log.LLvl3("================", o.ServerIdentity(), "in Start() ================")
	if o.Shared == nil {
		o.finish(false)
		return xerrors.New("please initialize Shared first")
	}
	if o.U == nil {
		o.finish(false)
		return xerrors.New("please initialize U first")
	}
	sr := &StartReencrypt{
		IsReenc: o.IsReenc,
		DKID:    o.DKID,
		U:       o.U,
		Xc:      o.Xc,
	}
	if len(o.VerificationData) > 0 {
		sr.VerificationData = &o.VerificationData
	}
	if o.Verify != nil {
		if !o.Verify(sr) {
			o.finish(false)
			return xerrors.New("refused to reencrypt")
		}
	}
	o.timeout = time.AfterFunc(1*time.Minute, func() {
		log.Lvl1("OCSNT protocol timeout")
		o.finish(false)
	})
	sr.Pr = o.generatePartial()
	log.LLvlf1("Before sending partialencrypt: %d, %x, %x, %x", sr.Pr.Ui.I, sr.Pr.Ui.V.String(), sr.Pr.Ei.String(), sr.Pr.Fi.String())
	// Broadcast StartReencrypt message to other nodes
	errs := o.Broadcast(sr)
	log.LLvl1("Broadcast errors:", errs)
	if len(errs) > (o.Total-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast")
	}
	return nil
}

// PartialReencrypt is received by every node to give his part of
// the share
func (o *OCSNT) reencrypt(ssr structStartReencrypt) error {
	log.Lvl3(o.ServerIdentity(), "received", ssr.StartReencrypt.Pr.Ei.String(), "from", ssr.ServerIdentity)
	if ssr.StartReencrypt.Pr.Ui == nil {
		log.Lvl2("Node", ssr.ServerIdentity, "refused to reply")
		o.Failures++
		if o.Failures > o.Total-o.Threshold {
			log.Lvl2(ssr.ServerIdentity, "couldn't get enough shares")
			o.finish(false)
		}
		return nil
	}
	o.replies = append(o.replies, ssr.StartReencrypt.Pr)

	if o.Verify != nil {
		if !o.Verify(&ssr.StartReencrypt) {
			log.Lvl2(o.ServerIdentity(), "refused to do the partial reencryption")
			errs := o.Broadcast(&PartialReencryption{})
			log.LLvl1("Broadcast errors:", errs)
			if len(errs) > (o.Total-1)/3 {
				log.Errorf("Some nodes failed with error(s) %v", errs)
				return xerrors.New("too many nodes failed in broadcast empty PartialReencryptReply")
			}
			return nil
		}
	}

	o.IsReenc = ssr.IsReenc
	o.DKID = ssr.DKID
	o.U = ssr.U
	o.Xc = ssr.Xc
	pr := o.generatePartial()
	errs := o.Broadcast(&pr)
	log.LLvl1("Broadcast errors:", errs)
	if len(errs) > (o.Total-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast PartialReencryptReply")
	}
	return nil
}

func (o *OCSNT) reencryptReply(spr structPartialReencryption) error {
	log.Lvl3(o.ServerIdentity(), "received", spr.PartialReencryption.Ei.String(), "from", spr.ServerIdentity)
	if spr.PartialReencryption.Ui == nil {
		log.Lvl2("Node", spr.ServerIdentity, "refused to reply")
		o.Failures++
		if o.Failures > o.Total-o.Threshold {
			log.Lvl2(spr.ServerIdentity, "couldn't get enough shares")
			o.finish(false)
		}
		return nil
	}
	o.replies = append(o.replies, spr.PartialReencryption)

	// minus one to exclude myself
	//if !o.isDone && len(o.replies) >= int(o.Threshold-1) {
	if len(o.replies) >= int(o.Threshold-1) {
		var xc kyber.Point
		o.Uis = make([]*share.PubShare, len(o.List()))
		o.Uis[o.ui.I] = o.ui
		if o.IsReenc {
			xc = o.Xc
		} else {
			xc = cothority.Suite.Point().Null()
		}
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
				log.Lvl1("Received invalid share from node", r.Ui.I)
			}
		}
		xhatEnc, err := share.RecoverCommit(cothority.Suite, o.Uis, o.Threshold, len(o.Roster().List))
		if err != nil {
			log.Errorf("%s couldn't recover secret", spr.ServerIdentity)
			err := o.SendToParent(&Ready{Success: false})
			if err != nil {
				log.Errorf("%s: error sending readyreply to parent", o.ServerIdentity())
			}
			o.finish(false)
		} else {
			o.XhatEnc = xhatEnc
			//o.isDone = true
			log.Lvl3(o.ServerIdentity(), "computed xhatenc:", o.XhatEnc.String())
			if !o.IsRoot() {
				err := o.SendToParent(&Ready{Success: true})
				if err != nil {
					log.Errorf("%s: error sending readyreply to parent", o.ServerIdentity())
				}
				o.finish(true)
			}
			// If root don't finish here!
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

func (o *OCSNT) ready(sr structReady) error {
	//o.readyMesgs = append(o.readyMesgs, sr.Ready)
	// minus one to exclude myself
	//if len(o.readyMesgs) >= int(o.Threshold-1) {
	//log.Lvl2(o.ServerIdentity(), "collected sufficient number of ready messages")
	//o.finish(true)
	//}
	log.Lvl3(o.ServerIdentity(), "received readyfrom", sr.ServerIdentity)
	if sr.Ready.Success {
		o.SuccessCount++
	} else {
		o.FailCount++
	}
	if o.SuccessCount >= o.Threshold {
		log.Lvl2(o.ServerIdentity, "collected sufficient number of success messages")
		o.finish(true)
	}
	if o.FailCount > o.Total-o.Threshold {
		log.Lvl2(o.ServerIdentity, "collected more than the failure-threshold number of failure messages")
		o.finish(false)
	}
	return nil
}

func (o *OCSNT) generatePartial() PartialReencryption {
	var xc kyber.Point
	if o.IsReenc {
		xc = o.Xc
	} else {
		xc = cothority.Suite.Point().Null()
	}
	o.ui = o.getUI(o.U, xc)
	// Calculating proofs
	si := cothority.Suite.Scalar().Pick(o.Suite().RandomStream())
	uiHat := cothority.Suite.Point().Mul(si, cothority.Suite.Point().Add(o.U, xc))
	hiHat := cothority.Suite.Point().Mul(si, nil)
	hash := sha256.New()
	o.ui.V.MarshalTo(hash)
	uiHat.MarshalTo(hash)
	hiHat.MarshalTo(hash)
	ei := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))
	fi := cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, o.Shared.V))
	return PartialReencryption{Ui: o.ui, Ei: ei, Fi: fi}
}

func (o *OCSNT) getUI(U, Xc kyber.Point) *share.PubShare {
	v := cothority.Suite.Point().Mul(o.Shared.V, U)
	v.Add(v, cothority.Suite.Point().Mul(o.Shared.V, Xc))
	return &share.PubShare{
		I: o.Shared.Index,
		V: v,
	}
}

// WIP: Should all nodes have a timeout or just the leader?
func (o *OCSNT) finish(result bool) {
	// Other nodes do not have a timeout set
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
	log.Lvl1("=============================== I'm finishing:", o.ServerIdentity())
	o.doneOnce.Do(func() { o.Done() })
}
