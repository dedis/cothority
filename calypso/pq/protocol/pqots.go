package protocol

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"sync"
	"time"
)

func init() {
	onet.GlobalProtocolRegister(NamePQOTS, NewPQOTS)
}

type PQOTS struct {
	*onet.TreeNodeInstance
	Xc        kyber.Point
	Share     *share.PriShare
	Threshold int
	Failures  int

	VerificationData []byte
	Verify           VerifyRequest
	GetShare         GetShare

	Reencrypted   chan bool
	Reencryptions []*EGP
	replies       []ReencryptReply
	timeout       *time.Timer
	doneOnce      sync.Once
}

func NewPQOTS(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	pqOts := &PQOTS{
		TreeNodeInstance: n,
		Reencrypted:      make(chan bool, 1),
		Threshold:        len(n.Roster().List) - (len(n.Roster().List)-1)/3,
	}
	err := pqOts.RegisterHandlers(pqOts.reencrypt, pqOts.reencryptReply)
	if err != nil {
		return nil, xerrors.Errorf("registering handlers: %v", err)
	}
	return pqOts, nil
}

func (pqOts *PQOTS) Start() error {
	rc := &Reencrypt{
		Xc: pqOts.Xc,
	}
	if len(pqOts.VerificationData) > 0 {
		rc.VerificationData = &pqOts.VerificationData
	}
	if pqOts.Verify != nil {
		if !pqOts.Verify(rc) {
			pqOts.finish(false)
			return xerrors.New("refused to reencrypt")
		}
	}
	if pqOts.GetShare != nil {
		sh, err := pqOts.GetShare(pqOts.VerificationData)
		if err != nil {
			pqOts.finish(false)
			return xerrors.Errorf("cannot get share: %v", err)
		}
		pqOts.Share = sh
	}
	K, Cs, err := elGamalEncrypt(cothority.Suite, pqOts.Xc, pqOts.Share)
	if err != nil {
		pqOts.Failures++
		log.Lvl1(pqOts.ServerIdentity(), "cannot reencrypt:", err.Error())
	} else {
		pqOts.replies = append(pqOts.replies, ReencryptReply{
			Index: pqOts.Share.I,
			Egp:   &EGP{K: K, Cs: Cs},
		})
	}
	pqOts.timeout = time.AfterFunc(1*time.Minute, func() {
		log.Lvl1("PQOTS protocol timeout")
		pqOts.finish(false)
	})
	errs := pqOts.Broadcast(rc)
	if len(errs) > (len(pqOts.Roster().List)-1)/3 {
		log.Errorf("Some nodes failed with error(s) %v", errs)
		return xerrors.New("too many nodes failed in broadcast")
	}
	return nil
}

func (pqOts *PQOTS) reencrypt(r structReencrypt) error {
	log.Lvl3(pqOts.Name() + ": starting reencrypt")
	defer pqOts.Done()

	if pqOts.Verify != nil {
		if !pqOts.Verify(&r.Reencrypt) {
			log.Lvl2(pqOts.ServerIdentity(), "refused to reencrypt")
			return cothority.ErrorOrNil(pqOts.SendToParent(&ReencryptReply{}),
				"sending ReencryptReply to parent")
		}
	}
	log.Lvl1(pqOts.Name() + ": verified")
	if pqOts.GetShare != nil {
		sh, err := pqOts.GetShare(*r.VerificationData)
		if err != nil {
			log.Errorf("%v couldn't find share: %v", pqOts.ServerIdentity(),
				err)
			return cothority.ErrorOrNil(pqOts.SendToParent(
				&ReencryptReply{}), "sending ReencryptReply to parent")
		}
		pqOts.Share = sh
	}
	log.Lvl1(pqOts.Name()+": received share ", pqOts.Share.I)
	K, Cs, err := elGamalEncrypt(cothority.Suite, r.Xc, pqOts.Share)
	if err != nil {
		log.Lvl2(pqOts.ServerIdentity(), "cannot reencrypt")
		return cothority.ErrorOrNil(pqOts.SendToParent(&ReencryptReply{}),
			"sending ReencryptReply to parent")
	}
	log.Lvl1(pqOts.Name() + ": sending reply to parent")
	return cothority.ErrorOrNil(
		pqOts.SendToParent(&ReencryptReply{
			Index: pqOts.Share.I,
			Egp: &EGP{
				K:  K,
				Cs: Cs,
			},
		}), "sending ReencryptReply to parent",
	)
}

func (pqOts *PQOTS) reencryptReply(rr structReencryptReply) error {
	if rr.ReencryptReply.Egp == nil {
		log.Lvl2("Node", rr.ServerIdentity, "refused to reply")
		pqOts.Failures++
		if pqOts.Failures > len(pqOts.Roster().List)-pqOts.Threshold {
			log.Lvl2(rr.ServerIdentity, "couldn't get enough shares")
			pqOts.finish(false)
		}
		return nil
	}
	log.Lvl1("wrapping up reencrpyt reply")
	pqOts.replies = append(pqOts.replies, rr.ReencryptReply)

	//if len(pqOts.replies) >= (pqOts.Threshold - 1) {
	if len(pqOts.replies) >= (pqOts.Threshold) {
		pqOts.Reencryptions = make([]*EGP, len(pqOts.List()))
		for _, r := range pqOts.replies {
			pqOts.Reencryptions[r.Index] = r.Egp
		}
		pqOts.finish(true)
	}
	return nil
}

func elGamalEncrypt(suite suites.Suite, pk kyber.Point,
	sh *share.PriShare) (kyber.Point, []kyber.Point, error) {
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

	//shp := suite.Point().Embed(shb, suite.RandomStream())
	//k := suite.Scalar().Pick(suite.RandomStream())
	//K := suite.Point().Mul(k, nil)
	//S := suite.Point().Mul(k, pk)
	//C := S.Add(S, shp)
	//return K, C, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func (pqOts *PQOTS) finish(result bool) {
	pqOts.timeout.Stop()
	select {
	case pqOts.Reencrypted <- result:
		// succeeded
	default:
		// would have blocked because some other call to finish()
		// beat us.
	}
	pqOts.doneOnce.Do(func() { pqOts.Done() })
}
