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

	"github.com/dedis/cothority"
	dkgprotocol "github.com/dedis/cothority/dkg"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func init() {
	onet.GlobalProtocolRegister(NameOCSBatch, NewOCSBatch)
}

// OCS is only used to re-encrypt a public point. Before calling `Start`,
// DKG and U must be initialized by the caller.
type OCSBatch struct {
	*onet.TreeNodeInstance
	Shared      map[int]*dkgprotocol.SharedSecret
	RcInput     map[int]RCInput
	Threshold   int // How many replies are needed to re-create the secret
	Verify      VerifyBatchRequest
	Reencrypted chan bool
	Failures    map[int]int

	replies  map[int][]*RCReply
	timeout  *time.Timer
	doneOnce sync.Once
	isDone   map[int]bool
	Uis      map[int][]*share.PubShare
}

func NewOCSBatch(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OCSBatch{
		TreeNodeInstance: n,
		Reencrypted:      make(chan bool, 1),
		Threshold:        len(n.Roster().List) - (len(n.Roster().List)-1)/3,
		Failures:         make(map[int]int),
		replies:          make(map[int][]*RCReply),
		isDone:           make(map[int]bool),
		Uis:              make(map[int][]*share.PubShare),
	}
	err := o.RegisterHandlers(o.rcBatch, o.rcBatchReply)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Start asks all children to reply with a shared reencryption
func (o *OCSBatch) Start() error {
	log.Lvl3("Starting Protocol")
	rcb := RCBatch{RC: make(map[int]*RCData)}
	for idx, rci := range o.RcInput {
		//rc := RCData{Idx: idx, Shared: rci.Shared, U: rci.U, Xc: rci.Xc}
		//rc := &RCData{Shared: rci.Shared, U: rci.U, Xc: rci.Xc}
		rc := &RCData{U: rci.U, Xc: rci.Xc}
		if len(rci.VerificationData) > 0 {
			rc.VerificationData = &rci.VerificationData
		}
		if o.Verify != nil {
			//if !o.Verify(&rc) {
			if !o.Verify(rc) {
				log.Errorf("Refused to reencrypt")
				o.isDone[idx] = true
				continue
			}
		}
		//rcb.RC = append(rcb.RC, rc)
		rcb.RC[idx] = rc
	}
	o.timeout = time.AfterFunc(10*time.Minute, func() {
		log.Lvl1("OCSBatch protocol timeout")
		o.finish(false)
	})
	errs := o.Broadcast(&rcb)
	if len(errs) > (len(o.Roster().List)-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return errors.New("too many nodes failed in broadcast")
	}
	return nil
}

// Reencrypt is received by every node to give his part of
// the share
func (o *OCSBatch) rcBatch(rcb structRCBatch) error {
	log.Lvl3(o.Name() + ": starting reencrypt")
	defer o.Done()

	replies := make(map[int]*RCReply)
	//for _, rc := range rcb.RC {
	for idx, rc := range rcb.RC {
		//log.Infof("CEYHUN BATCH (%d) - U is %s", idx, rc.U)
		//ui := o.getUI(rc.U, rc.Xc, rc.Shared)
		shd := o.Shared[idx]
		ui := o.getUI(rc.U, rc.Xc, shd)
		if o.Verify != nil {
			//if !o.Verify(&rc) {
			if !o.Verify(rc) {
				log.Lvl2(o.ServerIdentity(), "refused to reencrypt")
				//replies[rc.Idx] = &RCReply{}
				replies[idx] = &RCReply{}
				continue
			}
		}
		si := cothority.Suite.Scalar().Pick(o.Suite().RandomStream())
		uiHat := cothority.Suite.Point().Mul(si, cothority.Suite.Point().Add(rc.U, rc.Xc))
		hiHat := cothority.Suite.Point().Mul(si, nil)
		hash := sha256.New()
		ui.V.MarshalTo(hash)
		uiHat.MarshalTo(hash)
		hiHat.MarshalTo(hash)
		ei := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))
		//replies[rc.Idx] = &RCReply{
		replies[idx] = &RCReply{
			Ui: ui,
			Ei: ei,
			Fi: cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, shd.V)),
			//Fi: cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, rc.Shared.V)),
		}
		//log.Infof("CEYHUN BATCH (%d): %s %s", idx, ui.V.String(), ei.String())
		//replies[i].Ui = ui
		//replies[i].Ei = ei
		//replies[i].Fi = cothority.Suite.Scalar().Add(si, cothority.Suite.Scalar().Mul(ei, rc.Shared.V))
	}
	return o.SendToParent(&RCBReply{RCR: replies})
}

func (o *OCSBatch) rcBatchReply(rcbr structRCBReply) error {
	for idx, rcr := range rcbr.RCR {
		//idx := rcr.Idx
		if rcr.Ui == nil {
			log.Lvl2("Node", rcbr.ServerIdentity, "refused to reply for", idx)
			o.Failures[idx] = o.Failures[idx] + 1
			if o.Failures[idx] > len(o.Roster().List)-o.Threshold {
				log.Lvl2(rcbr.ServerIdentity, "couldn't get enough shares for", idx)
				o.isDone[idx] = true
			}
		} else {
			rl := o.replies[idx]
			rl = append(rl, rcr)
			o.replies[idx] = rl
			if len(o.replies[idx]) >= int(o.Threshold-1) {
				rci := o.RcInput[idx]
				uiList := make([]*share.PubShare, len(o.List()))
				uiList[0] = o.getUI(rci.U, rci.Xc, o.Shared[idx])
				//uiList[0] = o.getUI(rci.U, rci.Xc, rci.Shared)
				//log.Info("CEYHUN BATCH ROOT:", uiList[0].V.String())

				for _, r := range o.replies[idx] {
					ufi := cothority.Suite.Point().Mul(r.Fi, cothority.Suite.Point().Add(rci.U, rci.Xc))
					uiei := cothority.Suite.Point().Mul(cothority.Suite.Scalar().Neg(r.Ei), r.Ui.V)
					uiHat := cothority.Suite.Point().Add(ufi, uiei)
					gfi := cothority.Suite.Point().Mul(r.Fi, nil)
					gxi := rci.Poly.Eval(r.Ui.I).V
					hiei := cothority.Suite.Point().Mul(cothority.Suite.Scalar().Neg(r.Ei), gxi)
					hiHat := cothority.Suite.Point().Add(gfi, hiei)
					hash := sha256.New()
					r.Ui.V.MarshalTo(hash)
					uiHat.MarshalTo(hash)
					hiHat.MarshalTo(hash)
					e := cothority.Suite.Scalar().SetBytes(hash.Sum(nil))
					if e.Equal(r.Ei) {
						uiList[r.Ui.I] = r.Ui
					} else {
						log.Lvl1("Received invalid share from node", r.Ui.I)
					}
				}
				o.Uis[idx] = uiList
				o.isDone[idx] = true
			}
		}
	}
	if len(o.isDone) == len(o.RcInput) {
		o.finish(true)
	}
	return nil
}

func (o *OCSBatch) getUI(U, Xc kyber.Point, shd *dkgprotocol.SharedSecret) *share.PubShare {
	v := cothority.Suite.Point().Mul(shd.V, U)
	v.Add(v, cothority.Suite.Point().Mul(shd.V, Xc))
	return &share.PubShare{
		I: shd.Index,
		V: v,
	}
}

func (o *OCSBatch) finish(result bool) {
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
