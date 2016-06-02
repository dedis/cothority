// Package bftcosi store a novel way of scaling BFT for high scale internet
// applications especially blockchains
package bftcosi

import (
	"math"
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// VerificationFunction can be passes to each protocol node. It will be called
// (in a go routine) during the (start/handle) challenge prepare phase of the
// protocol
type VerificationFunction func([]byte) bool

// ProtocolBFTCoSi is the main struct for running the protocol
type ProtocolBFTCoSi struct {
	// the node we are represented-in
	*sda.TreeNodeInstance
	Msg []byte
	// the suite we use
	suite abstract.Suite
	// aggregated public key of the peers
	aggregatedPublic abstract.Point
	// prepare-round cosi
	prepare *cosi.Cosi
	// commit-round cosi
	commit *cosi.Cosi
	// channel for announcement
	announceChan chan announceChan
	// channel for commitment
	commitChan chan commitChan
	// Two channels for the challenge through the 2 rounds: difference is that
	// during the commit round, we need the previous signature of the "prepare"
	// round.
	// channel for challenge during the prepare phase
	challengePrepareChan chan challengePrepareChan
	// channel for challenge during the commit phase
	challengeCommitChan chan challengeCommitChan
	// channel for response
	responseChan chan responseChan
	// channel to notify when we are done
	done chan bool
	// channel to notify when the prepare round is finished
	prepareFinishedChan chan bool
	// channel used to wait for the verification of the block
	verifyChan chan bool

	// exceptions given during the rounds that is used in the signature
	tempExceptions []cosi.Exception

	// last block computed
	lastBlock string
	// temporary buffer of "prepare" commitments
	tempPrepareCommit []*cosi.Commitment
	tpcMut            sync.Mutex
	// temporary buffer of "commit" commitments
	tempCommitCommit []*cosi.Commitment
	tccMut           sync.Mutex
	// temporary buffer of "prepare" responses
	tempPrepareResponse []*cosi.Response
	tprMut              sync.Mutex
	// temporary buffer of "commit" responses
	tempCommitResponse []*cosi.Response
	tcrMut             sync.Mutex

	// refusal to sign for the commit phase or not. This flag is set during the
	// Challenge of the commit phase and will be used during the response of the
	// commit phase to put an exception or to sign.
	signRefusal bool

	// onDoneCallback is the callback that will be called at the end of the
	// protocol when all nodes have finished. Either at the end of the response
	// phase of the commit round or at the end of a view change.
	onDoneCallback func()

	// onSignatureDone is the callback that will be called when a signature has
	// been generated ( at the end of the response phase of the commit round)
	onSignatureDone func(*BFTSignature)

	// verificationFun will be called
	// during the (start/handle) challenge prepare phase of the protocol
	verificationFun VerificationFunction

	// bool set to true when the final signature is produced
	doneSigning chan bool
	// lock associated
	doneLock sync.Mutex
	// threshold for how much exception
	threshold int
	// done processing is used to stop the processing of the channels
	doneProcessing chan bool

	// finale signature that this BFTCoSi round has produced
	finalSignature *BFTSignature
}

// NewBFTCoSiProtocol returns a new bftcosi struct
func NewBFTCoSiProtocol(n *sda.TreeNodeInstance, verify VerificationFunction) (*ProtocolBFTCoSi, error) {
	// initialize the bftcosi node/protocol-instance
	bft := &ProtocolBFTCoSi{
		TreeNodeInstance: n,
		suite:            n.Suite(),
		prepare:          cosi.NewCosi(n.Suite(), n.Private()),
		commit:           cosi.NewCosi(n.Suite(), n.Private()),
		verifyChan:       make(chan bool),
		doneProcessing:   make(chan bool, 2),
		doneSigning:      make(chan bool, 1),
		verificationFun:  verify,
		aggregatedPublic: n.EntityList().Aggregate,
		threshold:        int(2.0 * math.Ceil(float64(len(n.Tree().List()))/3.0)),
	}

	// register channels
	if err := n.RegisterChannel(&bft.announceChan); err != nil {
		return nil, err
	}
	if err := n.RegisterChannel(&bft.commitChan); err != nil {
		return nil, err
	}
	if err := n.RegisterChannel(&bft.challengePrepareChan); err != nil {
		return nil, err
	}
	if err := n.RegisterChannel(&bft.challengeCommitChan); err != nil {
		return nil, err
	}
	if err := n.RegisterChannel(&bft.responseChan); err != nil {
		return nil, err
	}

	n.OnDoneCallback(bft.nodeDone)

	return bft, nil
}

// Start will start both rounds "prepare" and "commit" at same time. The
// "commit" round will wait till the end of the "prepare" round during its
// challenge phase.
func (bft *ProtocolBFTCoSi) Start() error {
	if err := bft.startAnnouncementPrepare(); err != nil {
		return err
	}
	return bft.startAnnouncementCommit()
}

// Dispatch listens on all channels and implements the sda.ProtocolInstance
// interface.
func (bft *ProtocolBFTCoSi) Dispatch() error {
	for {
		var err error
		select {
		case msg := <-bft.announceChan:
			// Announcement
			err = bft.handleAnnouncement(msg.Announce)
		case msg := <-bft.commitChan:
			// Commitment
			err = bft.handleCommit(msg.Commitment)

		case msg := <-bft.challengePrepareChan:
			// Challenge
			err = bft.handleChallengePrepare(&msg.ChallengePrepare)

		case msg := <-bft.challengeCommitChan:
			err = bft.handleChallengeCommit(&msg.ChallengeCommit)

		case msg := <-bft.responseChan:
			// Response
			switch msg.Response.TYPE {
			case RoundPrepare:
				err = bft.handleResponsePrepare(&msg.Response)
			case RoundCommit:
				err = bft.handleResponseCommit(&msg.Response)
			}
		case <-bft.doneProcessing:
			// we are done
			dbg.Lvl2(bft.Name(), "BFTCoSi Dispatches stop.")

			return nil
		}
		if err != nil {
			dbg.Error("Error handling messages:", err)
		}
	}
}

// startAnnouncementPrepare create its announcement for the prepare round and
// sends it down the tree.
func (bft *ProtocolBFTCoSi) startAnnouncementPrepare() error {
	ann := bft.prepare.CreateAnnouncement()
	a := &Announce{
		TYPE:         RoundPrepare,
		Announcement: ann,
	}
	dbg.Lvl3("BFTCoSi Start Announcement (PREPARE)")
	return bft.sendAnnouncement(a)
}

// startAnnouncementCommit create the announcement for the commit phase and
// sends it down the tree.
func (bft *ProtocolBFTCoSi) startAnnouncementCommit() error {
	ann := bft.commit.CreateAnnouncement()
	a := &Announce{
		TYPE:         RoundCommit,
		Announcement: ann,
	}
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Announcement (COMMIT)")
	return bft.sendAnnouncement(a)
}

func (bft *ProtocolBFTCoSi) sendAnnouncement(a *Announce) error {
	return bft.SendToChildrenInParallel(a)
}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bft *ProtocolBFTCoSi) handleAnnouncement(ann Announce) error {
	announcement := &Announce{
		Announcement: bft.prepare.Announce(ann.Announcement),
	}

	switch ann.TYPE {
	case RoundPrepare:
		dbg.Lvl3(bft.Name(), "BFTCoSi Handle Announcement PREPARE")
		if bft.IsLeaf() {
			return bft.startCommitmentPrepare()
		}
		announcement.TYPE = RoundPrepare
	case RoundCommit:
		dbg.Lvl3(bft.Name(), "BFTCoSi Handle Announcement COMMIT")
		if bft.IsLeaf() {
			return bft.startCommitmentCommit()
		}
		announcement.TYPE = RoundCommit
	}

	return bft.SendToChildrenInParallel(announcement)
}

// startPrepareCommitment send the first commitment up the tree for the prepare
// round.
func (bft *ProtocolBFTCoSi) startCommitmentPrepare() error {
	cm := bft.prepare.CreateCommitment()
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Commitment PREPARE")
	return bft.SendToParent(&Commitment{TYPE: RoundPrepare, Commitment: cm})
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bft *ProtocolBFTCoSi) startCommitmentCommit() error {
	cm := bft.commit.CreateCommitment()

	dbg.Lvl3(bft.Name(), "BFTCoSi Start Commitment COMMIT")
	return bft.SendToParent(&Commitment{TYPE: RoundCommit, Commitment: cm})
}

// handle the arrival of a commitment
func (bft *ProtocolBFTCoSi) handleCommit(comm Commitment) error {
	typedCommitment := &Commitment{}
	var commitment *cosi.Commitment
	// store it and check if we have enough commitments
	switch comm.TYPE {
	case RoundPrepare:
		bft.tpcMut.Lock()
		bft.tempPrepareCommit = append(bft.tempPrepareCommit, comm.Commitment)
		if len(bft.tempPrepareCommit) < len(bft.Children()) {
			bft.tpcMut.Unlock()
			return nil
		}
		commitment = bft.prepare.Commit(bft.tempPrepareCommit)
		bft.tpcMut.Unlock()
		if bft.IsRoot() {
			return bft.startChallengePrepare()
		}

		dbg.Lvl3(bft.Name(), "BFTCoSi handle Commit PREPARE")
	case RoundCommit:
		bft.tccMut.Lock()
		bft.tempCommitCommit = append(bft.tempCommitCommit, comm.Commitment)
		if len(bft.tempCommitCommit) < len(bft.Children()) {
			bft.tccMut.Unlock()
			return nil
		}
		commitment = bft.commit.Commit(bft.tempCommitCommit)
		bft.tccMut.Unlock()
		if bft.IsRoot() {
			// do nothing:
			// stop the processing of the round, wait the end of
			// the "prepare" round: calls startChallengeCommit
			return nil
		}

		dbg.Lvl3(bft.Name(), "BFTCoSi handle Commit COMMIT")
	}
	// set same RoundType as for the received commitment:
	typedCommitment.TYPE = comm.TYPE
	typedCommitment.Commitment = commitment
	return bft.SendToParent(typedCommitment)
}

// startPrepareChallenge create the challenge and send its down the tree
func (bft *ProtocolBFTCoSi) startChallengePrepare() error {
	// create challenge from message's hash:
	hash := bft.Suite().Hash()
	hash.Write(bft.Msg)
	h := hash.Sum(nil)

	ch, err := bft.prepare.CreateChallenge(h)
	if err != nil {
		return err
	}

	bftChal := &ChallengePrepare{
		TYPE:      RoundPrepare,
		Challenge: ch,
		Msg:       bft.Msg,
	}

	go func() {
		bft.verifyChan <- bft.verificationFun(bft.Msg)
	}()

	dbg.Lvl3(bft.Name(), "BFTCoSi Start Challenge PREPARE")
	return bft.SendToChildrenInParallel(bftChal)
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bft *ProtocolBFTCoSi) startChallengeCommit() error {
	c, err := bft.commit.CreateChallenge(bft.Msg)
	if err != nil {
		return err
	}

	// send challenge + signature
	cc := &ChallengeCommit{
		TYPE:      RoundCommit,
		Challenge: c,
		Signature: bft.prepare.Signature(),
	}
	dbg.Lvl3("BFTCoSi Start Challenge COMMIT")
	return bft.SendToChildrenInParallel(cc)
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bft *ProtocolBFTCoSi) handleChallengePrepare(ch *ChallengePrepare) error {
	bft.Msg = ch.Msg
	// start the verification of the message
	go func() {
		bft.verifyChan <- bft.verificationFun(bft.Msg)
	}()
	// acknowledge the challenge and send its down
	chal := bft.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	dbg.Lvl3(bft.Name(), "BFTCoSi handle Challenge PREPARE")
	// go to response if leaf
	if bft.IsLeaf() {
		return bft.startResponsePrepare()
	}

	return bft.SendToChildrenInParallel(ch)
}

// handleCommitChallenge will verify the signature + check if no more than 1/3
// of participants refused to sign.
func (bft *ProtocolBFTCoSi) handleChallengeCommit(ch *ChallengeCommit) error {
	ch.Challenge = bft.commit.Challenge(ch.Challenge)
	hash := bft.Suite().Hash()
	hash.Write(bft.Msg)
	h := hash.Sum(nil)

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bft.suite,
		bft.aggregatedPublic, h, ch.Signature,
		ch.Exceptions); err != nil {
		dbg.Error(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
	}

	// Check if we have no more than 1/3 failed nodes
	if len(ch.Exceptions) > int(bft.threshold) {
		dbg.Errorf("More than 1/3 (%d/%d) refused to sign ! ABORT",
			len(ch.Exceptions), len(bft.EntityList().List))
		bft.signRefusal = true
	}

	// store the exceptions for later usage
	bft.tempExceptions = ch.Exceptions
	dbg.Lvl3("BFTCoSi handle Challenge COMMIT")
	if bft.IsLeaf() {
		return bft.startResponseCommit()
	}

	if err := bft.SendToChildrenInParallel(ch); err != nil {
		dbg.Error(err)
	}
	return nil
}

// startPrepareResponse wait the verification of the block and then start the
// challenge process
func (bft *ProtocolBFTCoSi) startResponsePrepare() error {
	// create response
	resp, err := bft.prepare.CreateResponse()
	if err != nil {
		return err
	}
	// wait the verification
	r, ok := bft.waitResponseVerification()
	if ok {
		// append response only if OK
		r.Response = resp
	}
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Response PREPARE with response:", r)
	// send to parent
	return bft.SendTo(bft.Parent(), r)
}

// startCommitResponse will create the response for the commit phase and send it
// up. It will not create the response if it decided the signature is wrong from
// the prepare phase.
func (bft *ProtocolBFTCoSi) startResponseCommit() error {
	r := &Response{
		TYPE: RoundCommit,
	}
	// if i dont want to sign
	if bft.signRefusal {
		r.Exceptions = append(r.Exceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.commit.GetCommitment(),
		})
	} else {
		// otherwise i create the response
		resp, err := bft.commit.CreateResponse()
		if err != nil {
			return err
		}
		r.Response = resp
	}
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Response COMMIT")
	// send to parent
	err := bft.SendTo(bft.Parent(), r)
	bft.Done()
	return err
}

// handleResponseCommit handles the responses for the commit round during the
// response phase.
func (bft *ProtocolBFTCoSi) handleResponseCommit(r *Response) error {
	// check if we have enough
	bft.tcrMut.Lock()
	defer bft.tcrMut.Unlock()
	bft.tempCommitResponse = append(bft.tempCommitResponse, r.Response)

	if len(bft.tempCommitResponse) < len(bft.Children()) {
		return nil
	}

	if bft.signRefusal {
		r.Exceptions = append(r.Exceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.commit.GetCommitment(),
		})
	} else {
		resp, err := bft.commit.Response(bft.tempCommitResponse)
		if err != nil {
			return err
		}
		r.Response = resp
	}

	// notify we have finished to participate in this signature
	bft.doneSigning <- true
	dbg.Lvl3(bft.Name(), "BFTCoSi handle Response COMMIT (refusal=", bft.signRefusal, ")")
	// if root we have finished
	if bft.IsRoot() {
		sig := bft.Signature()
		if bft.onSignatureDone != nil {
			bft.onSignatureDone(sig)
		}
		bft.Done()
		return nil
	}

	// otherwise , send the response up
	err := bft.SendTo(bft.Parent(), r)
	bft.Done()
	return err
}

func (bft *ProtocolBFTCoSi) handleResponsePrepare(r *Response) error {
	// check if we have enough
	bft.tprMut.Lock()
	defer bft.tprMut.Unlock()
	bft.tempPrepareResponse = append(bft.tempPrepareResponse, r.Response)
	if len(bft.tempPrepareResponse) < len(bft.Children()) {
		return nil
	}

	// wait for verification
	bzrReturn, ok := bft.waitResponseVerification()
	if ok {
		// append response
		resp, err := bft.prepare.Response(bft.tempPrepareResponse)
		if err != nil {
			return err
		}
		bzrReturn.Response = resp
	}

	dbg.Lvl3("BFTCoSi Handle Response PREPARE")
	if bft.IsRoot() {
		// Notify 'commit'-round as we're root
		if err := bft.startChallengeCommit(); err != nil {
			dbg.Error(err)
		}

		return nil
	}
	return bft.SendTo(bft.Parent(), bzrReturn)
}

// waitResponseVerification waits till  the end of the verification and returns
// the BFTCoSiResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bft *ProtocolBFTCoSi) waitResponseVerification() (*Response, bool) {
	dbg.Lvl3("Waiting for response verification:", bft.Name())
	r := &Response{
		TYPE: RoundPrepare,
	}
	// wait the verification
	verified := <-bft.verifyChan
	if !verified {
		// append our exception
		r.Exceptions = append(r.Exceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.prepare.GetCommitment(),
		})
		dbg.Lvl3("Response verification: failed", bft.Name())
		return r, false
	}

	dbg.Lvl3("Response verification: OK", bft.Name())
	return r, true
}

// Signature will generate the final signature, the output of the BFTCoSi
// protocol.
func (bft *ProtocolBFTCoSi) Signature() *BFTSignature {
	return &BFTSignature{
		Sig:        bft.commit.Signature(),
		Msg:        bft.Msg,
		Exceptions: bft.tempExceptions,
	}
}

// RegisterOnDone registers a callback to call when the bftcosi protocols has
// really finished
func (bft *ProtocolBFTCoSi) RegisterOnDone(fn func()) {
	bft.onDoneCallback = fn
}

// RegisterOnSignatureDone register a callback to call when the bftcosi
// protocol reached a signature on the block
func (bft *ProtocolBFTCoSi) RegisterOnSignatureDone(fn func(*BFTSignature)) {
	bft.onSignatureDone = fn
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bft *ProtocolBFTCoSi) nodeDone() bool {
	dbg.Lvl3(bft.Name(), "nodeDone()")
	bft.doneProcessing <- true
	if bft.onDoneCallback != nil { // only true for the root
		bft.onDoneCallback()
	}
	return true
}
