package protocol

import (
	"crypto/sha256"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"sync"
	"time"
)

func init() {
	onet.GlobalProtocolRegister(NameOTS, NewOTS)
}

type OTS struct {
	*onet.TreeNodeInstance
	Threshold int
	Failures  int

	Xc       kyber.Point
	Share    *pvss.PubVerShare
	Proof    kyber.Point
	PolicyID darc.ID

	VerificationData []byte
	Verify           VerifyRequest

	Reencrypted   chan bool
	Reencryptions []*EGP
	replies       []ReencryptReply
	timeout       *time.Timer
	doneOnce      sync.Once
}

func NewOTS(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	ots := &OTS{
		TreeNodeInstance: n,
		Reencrypted:      make(chan bool, 1),
		Threshold:        len(n.Roster().List) - (len(n.Roster().List)-1)/3,
	}
	err := ots.RegisterHandlers(ots.reencrypt, ots.reencryptReply)
	if err != nil {
		return nil, xerrors.Errorf("registering handlers: %v", err)
	}
	return ots, nil
}

func (o *OTS) Start() error {
	rc := &Reencrypt{
		Xc: o.Xc,
	}
	if len(o.VerificationData) > 0 {
		rc.VerificationData = &o.VerificationData
	}
	o.timeout = time.AfterFunc(1*time.Minute, func() {
		log.Lvl1("OTS protocol timeout")
		o.finish(false)
	})
	if o.Verify != nil {
		o.Share, o.Proof, o.PolicyID = o.Verify(rc, o.Index())
		if o.Share == nil || o.Proof == nil || o.PolicyID == nil {
			o.finish(false)
			return xerrors.Errorf("refused to reencrypt")
		}
	}
	h := createPointH(o.PolicyID)
	sh, err := pvss.DecShare(cothority.Suite, h, o.Public(), o.Proof,
		o.Private(), o.Share)
	if err != nil {
		o.Failures++
		log.Lvl1(o.ServerIdentity(), "cannot reencrypt:", err.Error())
	} else {
		K, Cs, err := elGamalEncrypt(cothority.Suite, o.Xc, sh)
		if err != nil {
			o.Failures++
			log.Lvl1(o.ServerIdentity(), "cannot reencrypt:", err.Error())
		} else {
			o.replies = append(o.replies, ReencryptReply{
				Index: sh.S.I,
				Egp:   &EGP{K: K, Cs: Cs},
			})
		}
	}
	//o.timeout = time.AfterFunc(1*time.Minute, func() {
	//	log.Lvl1("OTS protocol timeout")
	//	o.finish(false)
	//})
	errs := o.Broadcast(rc)
	if len(errs) > (len(o.Roster().List)-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast")
	}
	return nil
}

func (o *OTS) reencrypt(r structReencrypt) error {
	log.Lvl3(o.Name() + ": starting reencrypt")
	defer o.Done()

	if o.Verify != nil {
		o.Share, o.Proof, o.PolicyID = o.Verify(&r.Reencrypt, o.Index())
		if o.Share == nil || o.Proof == nil || o.PolicyID == nil {
			log.Lvl2(o.ServerIdentity(), "refused to reencrypt")
			return cothority.ErrorOrNil(o.SendToParent(&ReencryptReply{}),
				"sending ReencryptReply to parent")
		}
	}
	h := createPointH(o.PolicyID)
	sh, err := pvss.DecShare(cothority.Suite, h, o.Public(), o.Proof,
		o.Private(), o.Share)
	if err != nil {
		log.Lvl2(o.ServerIdentity(), "cannot decrypt share")
		return cothority.ErrorOrNil(o.SendToParent(&ReencryptReply{}),
			"sending ReencryptReply to parent")
	}
	K, Cs, err := elGamalEncrypt(cothority.Suite, r.Xc, sh)
	if err != nil {
		log.Lvl2(o.ServerIdentity(), "cannot reencrypt")
		return cothority.ErrorOrNil(o.SendToParent(&ReencryptReply{}),
			"sending ReencryptReply to parent")
	}
	log.Lvl1(o.Name() + ": sending reply to parent")
	return cothority.ErrorOrNil(
		o.SendToParent(&ReencryptReply{
			Index: sh.S.I,
			Egp: &EGP{
				K:  K,
				Cs: Cs,
			},
		}), "sending ReencryptReply to parent",
	)
}

func (o *OTS) reencryptReply(rr structReencryptReply) error {
	if rr.ReencryptReply.Egp == nil {
		log.Lvl2("Node", rr.ServerIdentity, "refused to reply")
		o.Failures++
		if o.Failures > len(o.Roster().List)-o.Threshold {
			log.Lvl2(rr.ServerIdentity, "couldn't get enough shares")
			o.finish(false)
		}
		return nil
	}
	o.replies = append(o.replies, rr.ReencryptReply)
	if len(o.replies) >= (o.Threshold) {
		o.Reencryptions = make([]*EGP, len(o.List()))
		for _, r := range o.replies {
			o.Reencryptions[r.Index] = r.Egp
		}
		o.finish(true)
	}
	return nil
}

func createPointH(pid darc.ID) kyber.Point {
	hash := sha256.New()
	hash.Write(pid)
	return cothority.Suite.Point().Pick(cothority.Suite.XOF(hash.Sum(nil)))

}

func elGamalEncrypt(suite suites.Suite, pk kyber.Point,
	sh *pvss.PubVerShare) (kyber.Point, []kyber.Point, error) {
	shb, err := protobuf.Encode(sh)
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot encode share: %v", err)
	}
	var Cs []kyber.Point
	k := suite.Scalar().Pick(suite.RandomStream())
	K := suite.Point().Mul(k, nil)
	S := suite.Point().Mul(k, pk)
	for len(shb) > 0 {
		kp := suite.Point().Embed(shb, suite.RandomStream())
		Cs = append(Cs, suite.Point().Add(S, kp))
		shb = shb[min(len(shb), kp.EmbedLen()):]
	}
	return K, Cs, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func (o *OTS) finish(result bool) {
	o.timeout.Stop()
	select {
	case o.Reencrypted <- result:
		// succeeded
	default:
		// would have blocked because some other call to finish()
		// beat us.
	}
	o.doneOnce.Do(func() { o.Done() })
}
