// Package bftcosi store a novel way of scaling BFT for high scale internet
// applications especially blockchains
package bftcosi

import (
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// VerificationFunction can be passes to each protocol node. It will be called
// (in a go routine) during the (start/handle) challenge prepare phase of the
// protocol. The passed message is the same as sent in the challenge phase.
type VerificationFunction func(Msg []byte, Data []byte) bool

// ProtocolBFTCoSi is the main struct for running the protocol
type ProtocolBFTCoSi struct {
	// the node we are represented-in
	*sda.TreeNodeInstance
	// The message that will be signed by the BFTCosi
	Msg []byte
	// Data going along the msg to the verification
	Data []byte
	// the suite we use
	suite abstract.Suite
	// aggregated public key of the peers
	AggregatedPublic abstract.Point
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
	tempCommitPrepare []*cosi.Commitment
	tcMut             sync.Mutex
	// temporary buffer of "commit" commitments
	tempCommitCommit []*cosi.Commitment
	// temporary buffer of "prepare" responses
	tempResponsePrepare []*cosi.Response
	trpMut              sync.Mutex
	// temporary buffer of "commit" responses
	tempResponseCommit []*cosi.Response
	trcMut             sync.Mutex

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
	VerificationFunction VerificationFunction

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
		TreeNodeInstance:     n,
		suite:                n.Suite(),
		prepare:              cosi.NewCosi(n.Suite(), n.Private()),
		commit:               cosi.NewCosi(n.Suite(), n.Private()),
		verifyChan:           make(chan bool),
		doneProcessing:       make(chan bool, 2),
		doneSigning:          make(chan bool, 1),
		VerificationFunction: verify,
		AggregatedPublic:     n.EntityList().Aggregate,
		threshold:            len(n.Tree().List()) * 2 / 3,
		Msg:                  make([]byte, 0),
		Data:                 make([]byte, 0),
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
	if err := bft.startAnnouncement(RoundPrepare); err != nil {
		return err
	}
	return bft.startAnnouncement(RoundCommit)
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
			err = bft.handleCommitment(msg.Commitment)

		case msg := <-bft.challengePrepareChan:
			// Challenge
			err = bft.handleChallengePrepare(&msg.ChallengePrepare)

		case msg := <-bft.challengeCommitChan:
			err = bft.handleChallengeCommit(&msg.ChallengeCommit)

		case msg := <-bft.responseChan:
			// Response
			err = bft.startResponse(msg.Response.TYPE, &msg.Response)
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

func (bft *ProtocolBFTCoSi) getCosi(t RoundType) *cosi.Cosi {
	if t == RoundPrepare {
		return bft.prepare
	}
	return bft.commit
}

// startAnnouncementPrepare create its announcement for the prepare round and
// sends it down the tree.
func (bft *ProtocolBFTCoSi) startAnnouncement(t RoundType) error {
	ann := bft.getCosi(t).CreateAnnouncement()
	a := &Announce{
		TYPE:         t,
		Announcement: ann,
	}
	dbg.Lvl4("RoundType:", t)
	return bft.SendToChildrenInParallel(a)
}

// handleAnnouncement passes the announcement to the right CoSi struct.
func (bft *ProtocolBFTCoSi) handleAnnouncement(ann Announce) error {
	dbg.Lvl4(bft.Name(), "RoundType:", ann.TYPE)
	if bft.IsLeaf() {
		return bft.startCommitment(ann.TYPE)
	}
	return bft.SendToChildrenInParallel(&ann)
}

// startCommitment sends the first commitment to the parent node
func (bft *ProtocolBFTCoSi) startCommitment(t RoundType) error {
	cm := bft.getCosi(t).CreateCommitment()
	dbg.Lvl4(bft.Name(), "RoundType:", t)
	return bft.SendToParent(&Commitment{TYPE: t, Commitment: cm})
}

// handleCommitment collects all commitments from children and passes them
// to the parent or starts the challenge-round if it's the root.
func (bft *ProtocolBFTCoSi) handleCommitment(comm Commitment) error {
	dbg.Lvl4(bft.Name(), "RoundType:", comm.TYPE)

	var commitment *cosi.Commitment
	// store it and check if we have enough commitments
	bft.tcMut.Lock()
	defer bft.tcMut.Unlock()
	switch comm.TYPE {
	case RoundPrepare:
		bft.tempCommitPrepare = append(bft.tempCommitPrepare, comm.Commitment)
		if len(bft.tempCommitPrepare) < len(bft.Children()) {
			return nil
		}
		commitment = bft.prepare.Commit(bft.tempCommitPrepare)
		if bft.IsRoot() {
			return bft.startChallenge(RoundPrepare)
		}
	case RoundCommit:
		bft.tempCommitCommit = append(bft.tempCommitCommit, comm.Commitment)
		if len(bft.tempCommitCommit) < len(bft.Children()) {
			return nil
		}
		commitment = bft.commit.Commit(bft.tempCommitCommit)
		if bft.IsRoot() {
			// do nothing:
			// stop the processing of the round, wait the end of
			// the "prepare" round: calls startChallengeCommit
			return nil
		}

	}
	// set same RoundType as for the received commitment:
	typedCommitment := &Commitment{
		TYPE:       comm.TYPE,
		Commitment: commitment,
	}
	return bft.SendToParent(typedCommitment)
}

// startChallenge creates the challenge and sends it to its children
func (bft *ProtocolBFTCoSi) startChallenge(t RoundType) error {
	h := bft.Msg
	if t == RoundPrepare {
		// create challenge from message's hash:
		hash := bft.Suite().Hash()
		hash.Write(bft.Msg)
		h = hash.Sum(nil)
	}

	ch, err := bft.getCosi(t).CreateChallenge(h)
	if err != nil {
		return err
	}

	dbg.Lvl4(bft.Name(), "RoundType:", t)
	if t == RoundPrepare {
		bftChal := &ChallengePrepare{
			Challenge: ch,
			Msg:       bft.Msg,
			Data:      bft.Data,
		}

		return bft.handleChallengePrepare(bftChal)
	} else {
		// send challenge + signature
		cc := &ChallengeCommit{
			Challenge:  ch,
			Signature:  bft.prepare.Signature(),
			Exceptions: bft.tempExceptions,
		}
		return bft.handleChallengeCommit(cc)
	}
}

// handleChallengePrepare collects the challenge-messages
func (bft *ProtocolBFTCoSi) handleChallengePrepare(ch *ChallengePrepare) error {
	if !bft.IsRoot() {
		bft.Msg = ch.Msg
		bft.Data = ch.Data
		// start the verification of the message
		// acknowledge the challenge and send it down
		chal := bft.prepare.Challenge(ch.Challenge)
		ch.Challenge = chal
	}
	go func() {
		bft.verifyChan <- bft.VerificationFunction(bft.Msg, bft.Data)
	}()

	dbg.Lvl4(bft.Name(), "BFTCoSi handle Challenge PREPARE")
	// go to response if leaf
	if bft.IsLeaf() {
		return bft.startResponse(RoundPrepare, nil)
	}
	return bft.SendToChildrenInParallel(ch)
}

// handleChallengeCommit verifies the signature and checks if not more than
// the threshold of participants refused to sign
func (bft *ProtocolBFTCoSi) handleChallengeCommit(ch *ChallengeCommit) error {
	if !bft.IsRoot() {
		ch.Challenge = bft.commit.Challenge(ch.Challenge)
	}
	hash := bft.Suite().Hash()
	hash.Write(bft.Msg)
	h := hash.Sum(nil)

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bft.suite,
		bft.AggregatedPublic, h, ch.Signature,
		ch.Exceptions); err != nil {
		dbg.Lvl2(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
	}

	// Check if we have no more than threshold failed nodes
	if len(ch.Exceptions) >= int(bft.threshold) {
		dbg.Lvlf2("%s: More than threshold (%d/%d) refused to sign - aborting.",
			bft.Entity(), len(ch.Exceptions), len(bft.EntityList().List))
		bft.signRefusal = true
	}

	// store the exceptions for later usage
	bft.tempExceptions = ch.Exceptions
	dbg.Lvl4("BFTCoSi handle Challenge COMMIT")

	if bft.IsLeaf() {
		return bft.handleResponseCommit(nil)
	}

	return bft.SendToChildrenInParallel(ch)
}

// startResponse dispatches the response to the correct round-type
func (bft *ProtocolBFTCoSi) startResponse(t RoundType, r *Response) error {
	if t == RoundPrepare {
		return bft.handleResponsePrepare(r)
	}
	return bft.handleResponseCommit(r)
}

// handleResponsePrepare waits for the verification of the block and then starts the
// challenge process.
// If 'r' is nil, it will starts the response process.
func (bft *ProtocolBFTCoSi) handleResponsePrepare(r *Response) error {
	if r != nil {
		// check if we have enough responses
		bft.trpMut.Lock()
		defer bft.trpMut.Unlock()
		bft.tempResponsePrepare = append(bft.tempResponsePrepare, r.Response)
		bft.tempExceptions = append(bft.tempExceptions, r.Exceptions...)
		if len(bft.tempResponsePrepare) < len(bft.Children()) {
			return nil
		}
	}

	// wait for verification
	bzrReturn, ok := bft.waitResponseVerification()
	// append response
	if !ok {
		dbg.Lvl3(bft.Entity(), "Refused to sign")
	}

	// Return if we're not root
	if !bft.IsRoot() {
		return bft.SendTo(bft.Parent(), bzrReturn)
	}

	// Verify the signature is correct
	hash := bft.Suite().Hash()
	hash.Write(bft.Msg)
	h := hash.Sum(nil)
	if err := cosi.VerifyCosiSignatureWithException(bft.suite,
		bft.AggregatedPublic, h, bft.prepare.Signature(),
		bzrReturn.Exceptions); err != nil {
		dbg.Error(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
	}
	// Start the challenge of the 'commit'-round
	if err := bft.startChallenge(RoundCommit); err != nil {
		dbg.Error(err)
	}

	return nil
}

// handleResponseCommit collects all commits from the children and either
// passes the aggregate commit to its parent or starts a response-round
func (bft *ProtocolBFTCoSi) handleResponseCommit(r *Response) error {
	if r != nil {
		// check if we have enough
		bft.trcMut.Lock()
		defer bft.trcMut.Unlock()
		bft.tempResponseCommit = append(bft.tempResponseCommit, r.Response)

		if len(bft.tempResponseCommit) < len(bft.Children()) {
			return nil
		}
	} else {
		r = &Response{TYPE: RoundCommit,
			Response: &cosi.Response{
				Response: bft.suite.Secret().Zero(),
			}}
	}

	if bft.signRefusal {
		r.Exceptions = append(r.Exceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.commit.GetCommitment(),
		})
	} else {
		var err error
		if bft.IsLeaf() {
			r.Response, err = bft.commit.CreateResponse()
		} else {
			r.Response, err = bft.commit.Response(true, bft.tempResponseCommit)
		}
		if err != nil {
			return err
		}
	}

	// notify we have finished to participate in this signature
	bft.doneSigning <- true
	dbg.Lvl4(bft.Name(), "BFTCoSi handle Response COMMIT (refusal=", bft.signRefusal, ")")
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

// waitResponseVerification waits till the end of the verification and returns
// the BFTCoSiResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification failed
func (bft *ProtocolBFTCoSi) waitResponseVerification() (*Response, bool) {
	dbg.Lvl4(bft.Name(), "Waiting for response verification:")
	// wait the verification
	verified := <-bft.verifyChan

	resp, err := bft.prepare.Response(verified, bft.tempResponsePrepare)
	if err != nil {
		return nil, false
	}

	if !verified {
		// Add our exception
		bft.tempExceptions = append(bft.tempExceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.prepare.GetCommitment(),
		})
		dbg.Lvl4(bft.Name(), "Response verification: failed")
	}

	r := &Response{
		TYPE:       RoundPrepare,
		Exceptions: bft.tempExceptions,
		Response:   resp,
	}

	dbg.Lvl4("Response verification:", verified, bft.Name())
	return r, verified
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bft *ProtocolBFTCoSi) nodeDone() bool {
	dbg.Lvl4(bft.Name(), "nodeDone()")
	bft.doneProcessing <- true
	if bft.onDoneCallback != nil {
		// only true for the root
		bft.onDoneCallback()
	}
	return true
}
