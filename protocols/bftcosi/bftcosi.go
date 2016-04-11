// Package bftcosi store a novel way of scaling BFT for high scale internet applications
//esoecially blockchains
package bftcosi

import (
	"math"
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

var verificationRegister = make(map[string]interface{})

// RegisterVerification can be used to pass a verification function from another
// protocol which uses BFTCoSi (for example: ByzCoin). The protocol's verification
// function doesn't take any arguments and is identified using the protocols name
func RegisterVerification(protocol string, cb func()) {
	verificationRegister[protocol] = cb
}

// BFTCoSi is the main struct for running the protocol
type BFTCoSi struct {
	// the node we are represented-in
	*sda.Node
	Msg []byte
	// ProtoName is the protocol which passes the message to BFTCoSi. Can be
	// empty. ProtoName will be used to call the corresponding verification
	// function which was passed to RegisterVerification beforehand.
	ProtoName string
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

	// Call back when we start the announcement of the prepare phase
	onAnnouncementPrepare func()
	// callback when we finished the response of the prepare phase
	onResponsePrepareDone func()
	// callback when we finished the challenge of the commit phase
	onChallengeCommit func()

	onResponseCommitDone func()
	// view change setup and measurement

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
func NewBFTCoSiProtocol(n *sda.Node) (*BFTCoSi, error) {
	// create the bftcosi
	bz := new(BFTCoSi)
	bz.Node = n
	bz.suite = n.Suite()
	bz.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bz.commit = cosi.NewCosi(n.Suite(), n.Private())
	bz.verifyChan = make(chan bool)
	bz.doneProcessing = make(chan bool, 2)
	bz.doneSigning = make(chan bool, 1)

	//bz.endProto, _ = end.NewEndProtocol(n)
	bz.aggregatedPublic = n.EntityList().Aggregate
	bz.threshold = int(2.0 * math.Ceil(float64(len(bz.Tree().List())) / 3.0))

	// register channels
	n.RegisterChannel(&bz.announceChan)
	n.RegisterChannel(&bz.commitChan)
	n.RegisterChannel(&bz.challengePrepareChan)
	n.RegisterChannel(&bz.challengeCommitChan)
	n.RegisterChannel(&bz.responseChan)

	n.OnDoneCallback(bz.nodeDone)

	go bz.listen()
	return bz, nil
}

// NewBFTCoSiRootProtocol returns a new bftcosi struct with the block to sign
// that will be sent to all others nodes
func NewBFTCoSiRootProtocol(n *sda.Node, msg []byte) (*BFTCoSi, error) {
	bz, err := NewBFTCoSiProtocol(n)
	bz.Msg = msg
	if err != nil {
		return nil, err
	}
	return bz, err
}

// Start will start both rounds "prepare" and "commit" at same time. The
// "commit" round will wait the end of the "prepare" round during its challenge
// phase.
func (bz *BFTCoSi) Start() error {
	if err := bz.startAnnouncementPrepare(); err != nil {
		return err
	}
	return bz.startAnnouncementCommit()
}

// Dispatch listen on the different channels
func (bz *BFTCoSi) Dispatch() error {
	return nil
}
func (bz *BFTCoSi) listen() {
	for {
		var err error
		select {
		case msg := <-bz.announceChan:
			// Announcement
			err = bz.handleAnnouncement(msg.Announce)
		case msg := <-bz.commitChan:
			// Commitment
			err = bz.handleCommit(msg.Commitment)
			
		case msg := <-bz.challengePrepareChan:
			// Challenge
			err = bz.handleChallengePrepare(&msg.ChallengePrepare)
			
		case msg := <-bz.challengeCommitChan:
			err = bz.handleChallengeCommit(&msg.ChallengeCommit)
			
		case msg := <-bz.responseChan:
			// Response
				switch msg.Response.TYPE {
				case RoundPrepare:
					err = bz.handleResponsePrepare(&msg.Response)
				case RoundCommit:
					err = bz.handleResponseCommit(&msg.Response)
				}
		case <-bz.doneProcessing:
			// we are done
			dbg.Lvl2(bz.Name(), "BFTCoSi Dispatches stop.")

			return
		}
		if err != nil {
			dbg.Error("Error handling messages:", err)
		}
	}
}

// startAnnouncementPrepare create its announcement for the prepare round and
// sends it down the tree.
func (bz *BFTCoSi) startAnnouncementPrepare() error {
	if bz.onAnnouncementPrepare != nil {
		go bz.onAnnouncementPrepare()
	}

	ann := bz.prepare.CreateAnnouncement()
	bza := &Announce{
		TYPE:         RoundPrepare,
		Announcement: ann,
	}
	dbg.Lvl3("BFTCoSi Start Announcement (PREPARE)")
	return bz.sendAnnouncement(bza)
}

// startAnnouncementCommit create the announcement for the commit phase and
// sends it down the tree.
func (bz *BFTCoSi) startAnnouncementCommit() error {
	ann := bz.commit.CreateAnnouncement()
	bza := &Announce{
		TYPE:         RoundCommit,
		Announcement: ann,
	}
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Announcement (COMMIT)")
	return bz.sendAnnouncement(bza)
}

func (bz *BFTCoSi) sendAnnouncement(bza *Announce) error {
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bza)
	}
	return err
}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bz *BFTCoSi) handleAnnouncement(ann Announce) error {
	var announcement = new(Announce)

	switch ann.TYPE {
	case RoundPrepare:
		announcement = &Announce{
			TYPE:         RoundPrepare,
			Announcement: bz.prepare.Announce(ann.Announcement),
		}
		dbg.Lvl3(bz.Name(), "BFTCoSi Handle Announcement PREPARE")

		if bz.IsLeaf() {
			return bz.startCommitmentPrepare()
		}
	case RoundCommit:
		announcement = &Announce{
			TYPE:         RoundCommit,
			Announcement: bz.commit.Announce(ann.Announcement),
		}
		dbg.Lvl3(bz.Name(), "BFTCoSi Handle Announcement COMMIT")

		if bz.IsLeaf() {
			return bz.startCommitmentCommit()
		}
	}

	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, announcement)
	}
	return err
}

// startPrepareCommitment send the first commitment up the tree for the prepare
// round.
func (bz *BFTCoSi) startCommitmentPrepare() error {
	cm := bz.prepare.CreateCommitment()
	err := bz.SendTo(bz.Parent(), &Commitment{TYPE: RoundPrepare, Commitment: cm})
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Commitment PREPARE")
	return err
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bz *BFTCoSi) startCommitmentCommit() error {
	cm := bz.commit.CreateCommitment()

	err := bz.SendTo(bz.Parent(), &Commitment{TYPE: RoundCommit, Commitment: cm})
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Commitment COMMIT", err)
	return err
}

// handle the arrival of a commitment
func (bz *BFTCoSi) handleCommit(ann Commitment) error {
	var commitment *Commitment
	// store it and check if we have enough commitments
	switch ann.TYPE {
	case RoundPrepare:
		bz.tpcMut.Lock()
		bz.tempPrepareCommit = append(bz.tempPrepareCommit, ann.Commitment)
		if len(bz.tempPrepareCommit) < len(bz.Children()) {
			bz.tpcMut.Unlock()
			return nil
		}
		commit := bz.prepare.Commit(bz.tempPrepareCommit)
		bz.tpcMut.Unlock()
		if bz.IsRoot() {
			return bz.startChallengePrepare()
		}
		commitment = &Commitment{
			TYPE:       RoundPrepare,
			Commitment: commit,
		}
		dbg.Lvl3(bz.Name(), "BFTCoSi handle Commit PREPARE")
	case RoundCommit:
		bz.tccMut.Lock()
		bz.tempCommitCommit = append(bz.tempCommitCommit, ann.Commitment)
		if len(bz.tempCommitCommit) < len(bz.Children()) {
			bz.tccMut.Unlock()
			return nil
		}
		commit := bz.commit.Commit(bz.tempCommitCommit)
		bz.tccMut.Unlock()
		if bz.IsRoot() {
			// do nothing
			//	bz.startChallengeCommit()
			// stop the processing of the round, wait the end of the "prepare"
			// round. startChallengeCOmmit will be called then.
			return nil
		}
		commitment = &Commitment{
			TYPE:       RoundCommit,
			Commitment: commit,
		}
		dbg.Lvl3(bz.Name(), "BFTCoSi handle Commit COMMIT")
	}
	err := bz.SendTo(bz.Parent(), commitment)
	return err
}

// startPrepareChallenge create the challenge and send its down the tree
func (bz *BFTCoSi) startChallengePrepare() error {
	// make the challenge out of it
	// FIXME trblock := bz.tempBlock

	//prep:= &MsgPrepare{Msg : bz.Msg}
	h := bz.Suite().Hash().Sum(bz.Msg) //should be the prep

	ch, err := bz.prepare.CreateChallenge(h)
	if err != nil {
		return err
	}
	bizChal := &ChallengePrepare{
		TYPE:      RoundPrepare,
		Challenge: ch,
		Msg:       bz.Msg,
	}

	// TODO verification
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Challenge PREPARE")
	// send to children
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bizChal)
	}
	return err
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bz *BFTCoSi) startChallengeCommit() error {
	if bz.onChallengeCommit != nil {
		bz.onChallengeCommit()
	}
	// create the challenge out of it

	// TODO hash MsgCommit

	//com:= &MsgCommit{Msg : bz.Msg}
	h := bz.Suite().Hash().Sum(bz.Msg) //should be the com
	chal, err := bz.commit.CreateChallenge(h) 
	if err != nil {
		return err
	}

	// send challenge + signature
	bzc := &ChallengeCommit{
		TYPE:      RoundCommit,
		Challenge: chal,
		Signature: bz.prepare.Signature(),
	}
	dbg.Lvl3("BFTCoSi Start Challenge COMMIT")
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bzc)
	}
	return err
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bz *BFTCoSi) handleChallengePrepare(ch *ChallengePrepare) error {
	bz.Msg = ch.Msg
	// TODO find a way to pass the verification function or give back the
	// data for verification (using a channel)
	//check that the challenge is correctly formed 
	// acknowledge the challenge and send its down
	chal := bz.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	dbg.Lvl3(bz.Name(), "BFTCoSi handle Challenge PREPARE")
	// go to response if leaf
	if bz.IsLeaf() {
		return bz.startResponsePrepare()
	}
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, ch)
	}
	return err
}

// handleCommitChallenge will verify the signature + check if no more than 1/3
// of participants refused to sign.
func (bz *BFTCoSi) handleChallengeCommit(ch *ChallengeCommit) error {
	ch.Challenge = bz.commit.Challenge(ch.Challenge)

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bz.suite,
		bz.aggregatedPublic, bz.Msg, ch.Signature,
		ch.Exceptions); err != nil {
		dbg.Error(bz.Name(), "Verification of the signature failed:", err)
		bz.signRefusal = true
	}

	// Verify if we have no more than 1/3 failed nodes

	if len(ch.Exceptions) > int(bz.threshold) {
		dbg.Errorf("More than 1/3 (%d/%d) refused to sign ! ABORT", len(ch.Exceptions), len(bz.EntityList().List))
		bz.signRefusal = true
	}

	// store the exceptions for later usage
	bz.tempExceptions = ch.Exceptions
	dbg.Lvl3("BFTCoSi handle Challenge COMMIT")
	if bz.IsLeaf() {
		return bz.startResponseCommit()
	}

	//TODO verify that the challenge is correctly formed.

	// send it down
	for _, tn := range bz.Children() {
		if err := bz.SendTo(tn, ch); err != nil {
			dbg.Error(err)
		}
	}
	return nil
}

// startPrepareResponse wait the verification of the block and then start the
// challenge process
func (bz *BFTCoSi) startResponsePrepare() error {
	// create response
	resp, err := bz.prepare.CreateResponse()
	if err != nil {
		return err
	}
	// wait the verification
	bzr, ok := bz.waitResponseVerification()
	if ok {
		// apend response only if OK
		bzr.Response = resp
	}
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Response PREPARE")
	// send to parent
	return bz.SendTo(bz.Parent(), bzr)
}

// startCommitResponse will create the response for the commit phase and send it
// up. It will not create the response if it decided the signature is wrong from
// the prepare phase.
func (bz *BFTCoSi) startResponseCommit() error {
	bzr := &Response{
		TYPE: RoundCommit,
	}
	// if i dont want to sign
	if bz.signRefusal {
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{
			Public:     bz.Public(),
			Commitment: bz.commit.GetCommitment(),
		})
	} else {
		// otherwise i create the response
		resp, err := bz.commit.CreateResponse()
		if err != nil {
			return err
		}
		bzr.Response = resp
	}
	dbg.Lvl3(bz.Name(), "BFTCoSi Start Response COMMIT")
	// send to parent
	err := bz.SendTo(bz.Parent(), bzr)
	bz.Done()
	return err
}

// handleResponseCommit handles the responses for the commit round during the
// response phase.
func (bz *BFTCoSi) handleResponseCommit(bzr *Response) error {
	// check if we have enough
	// FIXME possible data race
	bz.tcrMut.Lock()
	bz.tempCommitResponse = append(bz.tempCommitResponse, bzr.Response)

	if len(bz.tempCommitResponse) < len(bz.Children()) {
		bz.tcrMut.Unlock()
		return nil
	}

	if bz.signRefusal {
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{
			Public:     bz.Public(),
			Commitment: bz.commit.GetCommitment(),
		})
		bz.tcrMut.Unlock()
	} else {
		resp, err := bz.commit.Response(bz.tempCommitResponse)
		bz.tcrMut.Unlock()
		if err != nil {
			return err
		}
		bzr.Response = resp
	}

	// notify we have finished to participate in this signature
	bz.doneSigning <- true
	dbg.Lvl3(bz.Name(), "BFTCoSi handle Response COMMIT (refusal=", bz.signRefusal, ")")
	// if root we have finished
	if bz.IsRoot() {
		sig := bz.Signature()
		if bz.onResponseCommitDone != nil {
			bz.onResponseCommitDone()
		}
		if bz.onSignatureDone != nil {
			bz.onSignatureDone(sig)
		}
		bz.Done()
		return nil
	}

	// otherwise , send the response up
	err := bz.SendTo(bz.Parent(), bzr)
	bz.Done()
	return err
}

func (bz *BFTCoSi) handleResponsePrepare(bzr *Response) error {
	// check if we have enough
	bz.tprMut.Lock()
	bz.tempPrepareResponse = append(bz.tempPrepareResponse, bzr.Response)
	if len(bz.tempPrepareResponse) < len(bz.Children()) {
		bz.tprMut.Unlock()
		return nil
	}

	// wait for verification
	bzrReturn, ok := bz.waitResponseVerification()
	if ok {
		// append response
		resp, err := bz.prepare.Response(bz.tempPrepareResponse)
		bz.tprMut.Unlock()
		if err != nil {
			return err
		}
		bzrReturn.Response = resp
	} else {
		bz.tprMut.Unlock()
	}

	dbg.Lvl3("BFTCoSi Handle Response PREPARE")
	// if I'm root, we are finished, let's notify the "commit" round
	if bz.IsRoot() {
		// notify listeners (simulation) we finished
		if bz.onResponsePrepareDone != nil {
			bz.onResponsePrepareDone()
		}
		bz.startChallengeCommit()

		return nil
	}
	// send up
	return bz.SendTo(bz.Parent(), bzrReturn)
}

// computePrepareResponse wait the end of the verification and returns the
// BFTCoSiResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bz *BFTCoSi) waitResponseVerification() (*Response, bool) {
	bzr := &Response{
		TYPE: RoundPrepare,
	}
	// wait the verification
	verified := <-bz.verifyChan
	if !verified {
		// append our exception
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{
			Public:     bz.Public(),
			Commitment: bz.prepare.GetCommitment(),
		})

		return bzr, false
	}

	return bzr, true
}

// Signature will generate the final signature, the output of the BFTCoSi
// protocol.
func (bz *BFTCoSi) Signature() *BFTSignature {
	return &BFTSignature{
		Sig:        bz.commit.Signature(),
		Msg:        bz.Msg,
		Exceptions: bz.tempExceptions,
	}
}

// RegisterOnDone registers a callback to call when the bftcosi protocols has
// really finished (after a view change maybe)
func (bz *BFTCoSi) RegisterOnDone(fn func()) {
	bz.onDoneCallback = fn
}

// RegisterOnSignatureDone register a callback to call when the bftcosi
// protocol reached a signature on the block
func (bz *BFTCoSi) RegisterOnSignatureDone(fn func(*BFTSignature)) {
	bz.onSignatureDone = fn
}


// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bz *BFTCoSi) nodeDone() bool {
	dbg.Lvl3(bz.Name(), "nodeDone()      ----- ")
	bz.doneProcessing <- true
	dbg.Lvl3(bz.Name(), "nodeDone()      +++++  ", bz.onDoneCallback)
	if bz.onDoneCallback != nil {
		bz.onDoneCallback()
	}
	return true
}

type MsgPrepare struct {
	Msg []byte
}

type MsgCommit struct {
	Msg []byte
} // TODO write binary marshaller for this and for MsgPrepare (adding some info to
// diff. between both
