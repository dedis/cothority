// Package bftcosi store a novel way of scaling BFT for high scale internet
// applications especially blockchains
package bftcosi

import (
	"math"
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
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
type ProtocolBFTCoSi struct {
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
func NewBFTCoSiProtocol(n *sda.Node) (*ProtocolBFTCoSi, error) {
	// create the bftcosi
	bft := new(ProtocolBFTCoSi)
	bft.Node = n
	bft.suite = n.Suite()
	bft.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bft.commit = cosi.NewCosi(n.Suite(), n.Private())
	bft.verifyChan = make(chan bool)
	bft.doneProcessing = make(chan bool, 2)
	bft.doneSigning = make(chan bool, 1)

	//bz.endProto, _ = end.NewEndProtocol(n)
	bft.aggregatedPublic = n.EntityList().Aggregate
	bft.threshold = int(2.0 * math.Ceil(float64(len(bft.Tree().List()))/3.0))

	// register channels
	n.RegisterChannel(&bft.announceChan)
	n.RegisterChannel(&bft.commitChan)
	n.RegisterChannel(&bft.challengePrepareChan)
	n.RegisterChannel(&bft.challengeCommitChan)
	n.RegisterChannel(&bft.responseChan)

	n.OnDoneCallback(bft.nodeDone)

	go bft.listen()
	return bft, nil
}

// NewBFTCoSiRootProtocol returns a new bftcosi struct with the block to sign
// that will be sent to all others nodes
func NewBFTCoSiRootProtocol(n *sda.Node, msg []byte) (*ProtocolBFTCoSi, error) {
	bft, err := NewBFTCoSiProtocol(n)
	bft.Msg = msg
	if err != nil {
		return nil, err
	}
	return bft, err
}

// Start will start both rounds "prepare" and "commit" at same time. The
// "commit" round will wait the end of the "prepare" round during its challenge
// phase.
func (bft *ProtocolBFTCoSi) Start() error {
	if err := bft.startAnnouncementPrepare(); err != nil {
		return err
	}
	return bft.startAnnouncementCommit()
}

// Dispatch listen on the different channels
func (bft *ProtocolBFTCoSi) Dispatch() error {
	return nil
}
func (bft *ProtocolBFTCoSi) listen() {
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

			return
		}
		if err != nil {
			dbg.Error("Error handling messages:", err)
		}
	}
}

// startAnnouncementPrepare create its announcement for the prepare round and
// sends it down the tree.
func (bft *ProtocolBFTCoSi) startAnnouncementPrepare() error {
	if bft.onAnnouncementPrepare != nil {
		go bft.onAnnouncementPrepare()
	}

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
	var err error
	for _, tn := range bft.Children() {
		err = bft.SendTo(tn, a)
	}
	return err
}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bft *ProtocolBFTCoSi) handleAnnouncement(ann Announce) error {
	var announcement = new(Announce)

	switch ann.TYPE {
	case RoundPrepare:
		announcement = &Announce{
			TYPE:         RoundPrepare,
			Announcement: bft.prepare.Announce(ann.Announcement),
		}
		dbg.Lvl3(bft.Name(), "BFTCoSi Handle Announcement PREPARE")

		if bft.IsLeaf() {
			return bft.startCommitmentPrepare()
		}
	case RoundCommit:
		announcement = &Announce{
			TYPE:         RoundCommit,
			Announcement: bft.commit.Announce(ann.Announcement),
		}
		dbg.Lvl3(bft.Name(), "BFTCoSi Handle Announcement COMMIT")

		if bft.IsLeaf() {
			return bft.startCommitmentCommit()
		}
	}

	var err error
	for _, tn := range bft.Children() {
		err = bft.SendTo(tn, announcement)
	}
	return err
}

// startPrepareCommitment send the first commitment up the tree for the prepare
// round.
func (bft *ProtocolBFTCoSi) startCommitmentPrepare() error {
	cm := bft.prepare.CreateCommitment()
	err := bft.SendTo(bft.Parent(), &Commitment{TYPE: RoundPrepare, Commitment: cm})
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Commitment PREPARE")
	return err
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bft *ProtocolBFTCoSi) startCommitmentCommit() error {
	cm := bft.commit.CreateCommitment()

	err := bft.SendTo(bft.Parent(), &Commitment{TYPE: RoundCommit, Commitment: cm})
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Commitment COMMIT", err)
	return err
}

// handle the arrival of a commitment
func (bft *ProtocolBFTCoSi) handleCommit(ann Commitment) error {
	var commitment *Commitment
	// store it and check if we have enough commitments
	switch ann.TYPE {
	case RoundPrepare:
		bft.tpcMut.Lock()
		bft.tempPrepareCommit = append(bft.tempPrepareCommit, ann.Commitment)
		if len(bft.tempPrepareCommit) < len(bft.Children()) {
			bft.tpcMut.Unlock()
			return nil
		}
		commit := bft.prepare.Commit(bft.tempPrepareCommit)
		bft.tpcMut.Unlock()
		if bft.IsRoot() {
			return bft.startChallengePrepare()
		}
		commitment = &Commitment{
			TYPE:       RoundPrepare,
			Commitment: commit,
		}
		dbg.Lvl3(bft.Name(), "BFTCoSi handle Commit PREPARE")
	case RoundCommit:
		bft.tccMut.Lock()
		bft.tempCommitCommit = append(bft.tempCommitCommit, ann.Commitment)
		if len(bft.tempCommitCommit) < len(bft.Children()) {
			bft.tccMut.Unlock()
			return nil
		}
		commit := bft.commit.Commit(bft.tempCommitCommit)
		bft.tccMut.Unlock()
		if bft.IsRoot() {
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
		dbg.Lvl3(bft.Name(), "BFTCoSi handle Commit COMMIT")
	}
	err := bft.SendTo(bft.Parent(), commitment)
	return err
}

// startPrepareChallenge create the challenge and send its down the tree
func (bft *ProtocolBFTCoSi) startChallengePrepare() error {
	// make the challenge out of it
	// FIXME trblock := bz.tempBlock

	prep := &MsgPrepare{Msg: bft.Msg}
	h := prep.Hash()

	ch, err := bft.prepare.CreateChallenge(h)
	if err != nil {
		return err
	}
	bizChal := &ChallengePrepare{
		TYPE:      RoundPrepare,
		Challenge: ch,
		Msg:       bft.Msg,
	}

	// TODO verification

	dbg.Lvl3(bft.Name(), "BFTCoSi Start Challenge PREPARE")
	// send to children
	for _, tn := range bft.Children() {
		err = bft.SendTo(tn, bizChal)
	}
	return err
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bft *ProtocolBFTCoSi) startChallengeCommit() error {
	if bft.onChallengeCommit != nil {
		bft.onChallengeCommit()
	}
	// create the challenge out of it
	com := &MsgCommit{Msg: bft.Msg}
	h := com.Hash() //should be the com
	chal, err := bft.commit.CreateChallenge(h)
	if err != nil {
		return err
	}

	// send challenge + signature
	bzc := &ChallengeCommit{
		TYPE:      RoundCommit,
		Challenge: chal,
		Signature: bft.prepare.Signature(),
	}
	dbg.Lvl3("BFTCoSi Start Challenge COMMIT")
	for _, tn := range bft.Children() {
		err = bft.SendTo(tn, bzc)
	}
	return err
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bft *ProtocolBFTCoSi) handleChallengePrepare(ch *ChallengePrepare) error {
	bft.Msg = ch.Msg
	// TODO find a way to pass the verification function or give back the
	// data for verification (using a channel)
	//check that the challenge is correctly formed
	// acknowledge the challenge and send its down
	chal := bft.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	dbg.Lvl3(bft.Name(), "BFTCoSi handle Challenge PREPARE")
	// go to response if leaf
	if bft.IsLeaf() {
		return bft.startResponsePrepare()
	}
	var err error
	for _, tn := range bft.Children() {
		err = bft.SendTo(tn, ch)
	}
	return err
}

// handleCommitChallenge will verify the signature + check if no more than 1/3
// of participants refused to sign.
func (bft *ProtocolBFTCoSi) handleChallengeCommit(ch *ChallengeCommit) error {
	ch.Challenge = bft.commit.Challenge(ch.Challenge)

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bft.suite,
		bft.aggregatedPublic, bft.Msg, ch.Signature,
		ch.Exceptions); err != nil {
		dbg.Error(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
	}

	// Verify if we have no more than 1/3 failed nodes

	if len(ch.Exceptions) > int(bft.threshold) {
		dbg.Errorf("More than 1/3 (%d/%d) refused to sign ! ABORT", len(ch.Exceptions), len(bft.EntityList().List))
		bft.signRefusal = true
	}

	// store the exceptions for later usage
	bft.tempExceptions = ch.Exceptions
	dbg.Lvl3("BFTCoSi handle Challenge COMMIT")
	if bft.IsLeaf() {
		return bft.startResponseCommit()
	}

	//TODO verify that the challenge is correctly formed.

	// send it down
	for _, tn := range bft.Children() {
		if err := bft.SendTo(tn, ch); err != nil {
			dbg.Error(err)
		}
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
		// apend response only if OK
		r.Response = resp
	}
	dbg.Lvl3(bft.Name(), "BFTCoSi Start Response PREPARE")
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
	// FIXME possible data race
	bft.tcrMut.Lock()
	bft.tempCommitResponse = append(bft.tempCommitResponse, r.Response)

	if len(bft.tempCommitResponse) < len(bft.Children()) {
		bft.tcrMut.Unlock()
		return nil
	}

	if bft.signRefusal {
		r.Exceptions = append(r.Exceptions, cosi.Exception{
			Public:     bft.Public(),
			Commitment: bft.commit.GetCommitment(),
		})
		bft.tcrMut.Unlock()
	} else {
		resp, err := bft.commit.Response(bft.tempCommitResponse)
		bft.tcrMut.Unlock()
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
		if bft.onResponseCommitDone != nil {
			bft.onResponseCommitDone()
		}
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
	bft.tempPrepareResponse = append(bft.tempPrepareResponse, r.Response)
	if len(bft.tempPrepareResponse) < len(bft.Children()) {
		bft.tprMut.Unlock()
		return nil
	}

	// wait for verification
	bzrReturn, ok := bft.waitResponseVerification()
	if ok {
		// append response
		resp, err := bft.prepare.Response(bft.tempPrepareResponse)
		bft.tprMut.Unlock()
		if err != nil {
			return err
		}
		bzrReturn.Response = resp
	} else {
		bft.tprMut.Unlock()
	}

	dbg.Lvl3("BFTCoSi Handle Response PREPARE")
	// if I'm root, we are finished, let's notify the "commit" round
	if bft.IsRoot() {
		// notify listeners (simulation) we finished
		if bft.onResponsePrepareDone != nil {
			bft.onResponsePrepareDone()
		}
		bft.startChallengeCommit()

		return nil
	}
	// send up
	return bft.SendTo(bft.Parent(), bzrReturn)
}

// computePrepareResponse wait the end of the verification and returns the
// BFTCoSiResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bft *ProtocolBFTCoSi) waitResponseVerification() (*Response, bool) {
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

		return r, false
	}

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
	dbg.Lvl3(bft.Name(), "nodeDone()      ----- ")
	bft.doneProcessing <- true
	dbg.Lvl3(bft.Name(), "nodeDone()      +++++  ", bft.onDoneCallback)
	if bft.onDoneCallback != nil {
		bft.onDoneCallback()
	}
	return true
}

const commitPrefix = 0X0
const preparePrefix = 0x1

type MsgPrepare struct {
	Msg []byte
}

func (mp *MsgPrepare) Hash() []byte {
	h := network.Suite.Hash()
	temp := append(mp.Msg, preparePrefix)
	h.Write(temp)

	return h.Sum(nil)
}

type MsgCommit struct {
	Msg []byte
}

func (mc *MsgCommit) Hash() []byte {
	h := network.Suite.Hash()
	temp := append(mc.Msg, commitPrefix)
	h.Write(temp)

	return h.Sum(nil)
}
