// Package byzcoin store a novel way of implementing the Bitcoin protocol using
// CoSi for signing sidechains.
package byzcoin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
	"github.com/dedis/crypto/abstract"
)

// ByzCoin is the main struct for running the protocol
type ByzCoin struct {
	// the node we are represented-in
	*sda.TreeNodeInstance
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
	verifyBlockChan chan bool

	//  block to pass up between the two rounds (prepare + commits)
	tempBlock *blockchain.TrBlock
	// exceptions given during the rounds that is used in the signature
	tempExceptions []cosi.Exception

	// transactions is the slice of transactions that contains transactions
	// coming from clients
	transactions []blkparser.Tx
	// last block computed
	lastBlock string
	// last key block computed
	lastKeyBlock string
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
	onSignatureDone func(*BlockSignature)

	// rootTimeout is the timeout given to the root. It will be passed down the
	// tree so every nodes knows how much time to wait. This root is a very nice
	// malicious node.
	rootTimeout uint64
	timeoutChan chan uint64
	// onTimeoutCallback is the function that will be called if a timeout
	// occurs.
	onTimeoutCallback func()
	// function to let callers of the protocol (or the server) add functionality
	// to certain parts of the protocol; mainly used in simulation to do
	// measurements. Hence functions will not be called in go routines

	// root fails:
	rootFailMode uint
	// Call back when we start the announcement of the prepare phase
	onAnnouncementPrepare func()
	// callback when we finished the response of the prepare phase
	onResponsePrepareDone func()
	// callback when we finished the challenge of the commit phase
	onChallengeCommit func()

	onResponseCommitDone func()
	// view change setup and measurement
	viewchangeChan chan struct {
		*sda.TreeNode
		viewChange
	}
	vcMeasure *monitor.TimeMeasure
	// bool set to true when the final signature is produced
	doneSigning chan bool
	// lock associated
	doneLock sync.Mutex
	// threshold for how much exception
	threshold int
	// threshold for how much view change acceptance we need
	// basically n - threshold
	viewChangeThreshold int
	// how many view change request have we received
	vcCounter int
	// done processing is used to stop the processing of the channels
	doneProcessing chan bool

	// finale signature that this ByzCoin round has produced
	finalSignature *BlockSignature
}

// NewByzCoinProtocol returns a new byzcoin struct
func NewByzCoinProtocol(n *sda.TreeNodeInstance) (*ByzCoin, error) {
	// create the byzcoin
	bz := new(ByzCoin)
	bz.TreeNodeInstance = n
	bz.suite = n.Suite()
	bz.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bz.commit = cosi.NewCosi(n.Suite(), n.Private())
	bz.verifyBlockChan = make(chan bool)
	bz.doneProcessing = make(chan bool, 2)
	bz.doneSigning = make(chan bool, 1)
	bz.timeoutChan = make(chan uint64, 1)

	//bz.endProto, _ = end.NewEndProtocol(n)
	bz.aggregatedPublic = n.EntityList().Aggregate
	bz.threshold = int(math.Ceil(float64(len(bz.Tree().List())) / 3.0))
	bz.viewChangeThreshold = int(math.Ceil(float64(len(bz.Tree().List())) * 2.0 / 3.0))

	// register channels
	if err := n.RegisterChannel(&bz.announceChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.commitChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.challengePrepareChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.challengeCommitChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.responseChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.viewchangeChan); err != nil {
		return bz, err
	}

	n.OnDoneCallback(bz.nodeDone)

	go bz.listen()
	return bz, nil
}

// NewByzCoinRootProtocol returns a new byzcoin struct with the block to sign
// that will be sent to all others nodes
func NewByzCoinRootProtocol(n *sda.TreeNodeInstance, transactions []blkparser.Tx, timeOutMs uint64, failMode uint) (*ByzCoin, error) {
	bz, err := NewByzCoinProtocol(n)
	if err != nil {
		return nil, err
	}
	bz.tempBlock, err = GetBlock(transactions, bz.lastBlock, bz.lastKeyBlock)
	bz.rootFailMode = failMode
	bz.rootTimeout = timeOutMs
	return bz, err
}

// Start will start both rounds "prepare" and "commit" at same time. The
// "commit" round will wait the end of the "prepare" round during its challenge
// phase.
func (bz *ByzCoin) Start() error {
	if err := bz.startAnnouncementPrepare(); err != nil {
		return err
	}
	return bz.startAnnouncementCommit()
}

// Dispatch listen on the different channels
func (bz *ByzCoin) Dispatch() error {
	return nil
}
func (bz *ByzCoin) listen() {
	// FIXME handle different failure modes
	fail := (bz.rootFailMode != 0) && bz.IsRoot()
	var timeoutStarted bool
	for {
		var err error
		select {
		case msg := <-bz.announceChan:
			// Announcement
			err = bz.handleAnnouncement(msg.Announce)
		case msg := <-bz.commitChan:
			// Commitment
			if !fail {
				err = bz.handleCommit(msg.Commitment)
			}
		case msg := <-bz.challengePrepareChan:
			// Challenge
			if !fail {
				err = bz.handleChallengePrepare(&msg.ChallengePrepare)
			}
		case msg := <-bz.challengeCommitChan:
			if !fail {
				err = bz.handleChallengeCommit(&msg.ChallengeCommit)
			}
		case msg := <-bz.responseChan:
			// Response
			if !fail {
				switch msg.Response.TYPE {
				case RoundPrepare:
					err = bz.handleResponsePrepare(&msg.Response)
				case RoundCommit:
					err = bz.handleResponseCommit(&msg.Response)
				}
			}
		case timeout := <-bz.timeoutChan:
			// start the timer
			if timeoutStarted {
				continue
			}
			timeoutStarted = true
			go bz.startTimer(timeout)
		case msg := <-bz.viewchangeChan:
			// receive view change
			err = bz.handleViewChange(msg.TreeNode, &msg.viewChange)
		case <-bz.doneProcessing:
			// we are done
			dbg.Lvl2(bz.Name(), "ByzCoin Dispatches stop.")
			bz.tempBlock = nil
			return
		}
		if err != nil {
			dbg.Error("Error handling messages:", err)
		}
	}
}

// startAnnouncementPrepare create its announcement for the prepare round and
// sends it down the tree.
func (bz *ByzCoin) startAnnouncementPrepare() error {
	if bz.onAnnouncementPrepare != nil {
		go bz.onAnnouncementPrepare()
	}

	ann := bz.prepare.CreateAnnouncement()
	bza := &Announce{
		TYPE:         RoundPrepare,
		Announcement: ann,
		Timeout:      bz.rootTimeout,
	}
	dbg.Lvl3("ByzCoin Start Announcement (PREPARE)")
	return bz.sendAnnouncement(bza)
}

// startAnnouncementCommit create the announcement for the commit phase and
// sends it down the tree.
func (bz *ByzCoin) startAnnouncementCommit() error {
	ann := bz.commit.CreateAnnouncement()
	bza := &Announce{
		TYPE:         RoundCommit,
		Announcement: ann,
	}
	dbg.Lvl3(bz.Name(), "ByzCoin Start Announcement (COMMIT)")
	return bz.sendAnnouncement(bza)
}

func (bz *ByzCoin) sendAnnouncement(bza *Announce) error {
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bza)
	}
	return err
}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bz *ByzCoin) handleAnnouncement(ann Announce) error {
	var announcement = new(Announce)

	switch ann.TYPE {
	case RoundPrepare:
		announcement = &Announce{
			TYPE:         RoundPrepare,
			Announcement: bz.prepare.Announce(ann.Announcement),
			Timeout:      ann.Timeout,
		}

		bz.timeoutChan <- ann.Timeout
		// give the timeout
		dbg.Lvl3(bz.Name(), "ByzCoin Handle Announcement PREPARE")

		if bz.IsLeaf() {
			return bz.startCommitmentPrepare()
		}
	case RoundCommit:
		announcement = &Announce{
			TYPE:         RoundCommit,
			Announcement: bz.commit.Announce(ann.Announcement),
		}
		dbg.Lvl3(bz.Name(), "ByzCoin Handle Announcement COMMIT")

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
func (bz *ByzCoin) startCommitmentPrepare() error {
	cm := bz.prepare.CreateCommitment()
	err := bz.SendTo(bz.Parent(), &Commitment{TYPE: RoundPrepare, Commitment: cm})
	dbg.Lvl3(bz.Name(), "ByzCoin Start Commitment PREPARE")
	return err
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bz *ByzCoin) startCommitmentCommit() error {
	cm := bz.commit.CreateCommitment()

	err := bz.SendTo(bz.Parent(), &Commitment{TYPE: RoundCommit, Commitment: cm})
	dbg.Lvl3(bz.Name(), "ByzCoin Start Commitment COMMIT", err)
	return err
}

// handle the arrival of a commitment
func (bz *ByzCoin) handleCommit(ann Commitment) error {
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
		dbg.Lvl3(bz.Name(), "ByzCoin handle Commit PREPARE")
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
		dbg.Lvl3(bz.Name(), "ByzCoin handle Commit COMMIT")
	}
	err := bz.SendTo(bz.Parent(), commitment)
	return err
}

// startPrepareChallenge create the challenge and send its down the tree
func (bz *ByzCoin) startChallengePrepare() error {
	// make the challenge out of it
	trblock := bz.tempBlock
	marshalled, err := json.Marshal(trblock)
	if err != nil {
		return err
	}
	ch, err := bz.prepare.CreateChallenge(marshalled)
	if err != nil {
		return err
	}
	bizChal := &ChallengePrepare{
		TYPE:      RoundPrepare,
		Challenge: ch,
		TrBlock:   trblock,
	}

	go VerifyBlock(bz.tempBlock, bz.lastBlock, bz.lastKeyBlock, bz.verifyBlockChan)
	dbg.Lvl3(bz.Name(), "ByzCoin Start Challenge PREPARE")
	// send to children
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bizChal)
	}
	return err
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bz *ByzCoin) startChallengeCommit() error {
	if bz.onChallengeCommit != nil {
		bz.onChallengeCommit()
	}
	// create the challenge out of it
	marshalled := bz.tempBlock.HashSum()
	chal, err := bz.commit.CreateChallenge(marshalled)
	if err != nil {
		return err
	}

	// send challenge + signature
	bzc := &ChallengeCommit{
		TYPE:      RoundCommit,
		Challenge: chal,
		Signature: bz.prepare.Signature(),
	}
	dbg.Lvl3("ByzCoin Start Challenge COMMIT")
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bzc)
	}
	return err
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bz *ByzCoin) handleChallengePrepare(ch *ChallengePrepare) error {
	bz.tempBlock = ch.TrBlock
	// start the verification of the block
	go VerifyBlock(bz.tempBlock, bz.lastBlock, bz.lastKeyBlock, bz.verifyBlockChan)
	// acknowledge the challenge and send its down
	chal := bz.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	dbg.Lvl3(bz.Name(), "ByzCoin handle Challenge PREPARE")
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
func (bz *ByzCoin) handleChallengeCommit(ch *ChallengeCommit) error {
	// marshal the block
	marshalled, err := json.Marshal(bz.tempBlock)
	if err != nil {
		return err
	}
	ch.Challenge = bz.commit.Challenge(ch.Challenge)

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bz.suite, bz.aggregatedPublic, marshalled, ch.Signature, ch.Exceptions); err != nil {
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
	dbg.Lvl3("ByzCoin handle Challenge COMMIT")
	if bz.IsLeaf() {
		return bz.startResponseCommit()
	}

	// send it down
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, ch)
	}
	return nil
}

// startPrepareResponse wait the verification of the block and then start the
// challenge process
func (bz *ByzCoin) startResponsePrepare() error {
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
	dbg.Lvl3(bz.Name(), "ByzCoin Start Response PREPARE")
	// send to parent
	return bz.SendTo(bz.Parent(), bzr)
}

// startCommitResponse will create the response for the commit phase and send it
// up. It will not create the response if it decided the signature is wrong from
// the prepare phase.
func (bz *ByzCoin) startResponseCommit() error {
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
	dbg.Lvl3(bz.Name(), "ByzCoin Start Response COMMIT")
	// send to parent
	err := bz.SendTo(bz.Parent(), bzr)
	bz.Done()
	return err
}

// handleResponseCommit handles the responses for the commit round during the
// response phase.
func (bz *ByzCoin) handleResponseCommit(bzr *Response) error {
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
	dbg.Lvl3(bz.Name(), "ByzCoin handle Response COMMIT (refusal=", bz.signRefusal, ")")
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

func (bz *ByzCoin) handleResponsePrepare(bzr *Response) error {
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

	dbg.Lvl3("ByzCoin Handle Response PREPARE")
	// if I'm root, we are finished, let's notify the "commit" round
	if bz.IsRoot() {
		// notify listeners (simulation) we finished
		if bz.onResponsePrepareDone != nil {
			bz.onResponsePrepareDone()
		}
		return bz.startChallengeCommit()
	}
	// send up
	return bz.SendTo(bz.Parent(), bzrReturn)
}

// computePrepareResponse wait the end of the verification and returns the
// ByzCoinResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bz *ByzCoin) waitResponseVerification() (*Response, bool) {
	bzr := &Response{
		TYPE: RoundPrepare,
	}
	// wait the verification
	verified := <-bz.verifyBlockChan
	if !verified {
		// append our exception
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{
			Public:     bz.Public(),
			Commitment: bz.prepare.GetCommitment(),
		})
		bz.sendAndMeasureViewchange()
		return bzr, false
	}

	return bzr, true
}

// VerifyBlock is a simulation of a real verification block algorithm
func VerifyBlock(block *blockchain.TrBlock, lastBlock, lastKeyBlock string, done chan bool) {
	//We measure the average block verification delays is 174ms for an average
	//block of 500kB.
	//To simulate the verification cost of bigger blocks we multiply 174ms
	//times the size/500*1024
	b, _ := json.Marshal(block)
	s := len(b)
	var n time.Duration
	n = time.Duration(s / (500 * 1024))
	time.Sleep(150 * time.Millisecond * n) //verification of 174ms per 500KB simulated
	// verification of the header
	verified := block.Header.Parent == lastBlock && block.Header.ParentKey == lastKeyBlock
	verified = verified && block.Header.MerkleRoot == blockchain.HashRootTransactions(block.TransactionList)
	verified = verified && block.HeaderHash == blockchain.HashHeader(block.Header)
	// notify it
	dbg.Lvl3("Verification of the block done =", verified)
	done <- verified
}

// GetBlock returns the next block available from the transaction pool.
func GetBlock(transactions []blkparser.Tx, lastBlock, lastKeyBlock string) (*blockchain.TrBlock, error) {
	if len(transactions) < 1 {
		return nil, errors.New("no transaction available")
	}

	trlist := blockchain.NewTransactionList(transactions, len(transactions))
	header := blockchain.NewHeader(trlist, lastBlock, lastKeyBlock)
	trblock := blockchain.NewTrBlock(trlist, header)
	return trblock, nil
}

// Signature will generate the final signature, the output of the ByzCoin
// protocol.
func (bz *ByzCoin) Signature() *BlockSignature {
	return &BlockSignature{
		Sig:        bz.commit.Signature(),
		Block:      bz.tempBlock,
		Exceptions: bz.tempExceptions,
	}
}

// RegisterOnDone registers a callback to call when the byzcoin protocols has
// really finished (after a view change maybe)
func (bz *ByzCoin) RegisterOnDone(fn func()) {
	bz.onDoneCallback = fn
}

// RegisterOnSignatureDone register a callback to call when the byzcoin
// protocol reached a signature on the block
func (bz *ByzCoin) RegisterOnSignatureDone(fn func(*BlockSignature)) {
	bz.onSignatureDone = fn
}

// startTimer starts the timer to decide whether we should request a view change
// after a certain timeout or not. If the signature is done, we don't. otherwise
// we start the view change protocol.
func (bz *ByzCoin) startTimer(millis uint64) {
	if bz.rootFailMode != 0 {
		dbg.Lvl3(bz.Name(), "Started timer (", millis, ")...")
		select {
		case <-bz.doneSigning:
			return
		case <-time.After(time.Millisecond * time.Duration(millis)):
			bz.sendAndMeasureViewchange()
		}
	}
}

// sendAndMeasureViewChange is a method that creates the viewchange request,
// broadcast it and measures the time it takes to accept it.
func (bz *ByzCoin) sendAndMeasureViewchange() {
	dbg.Lvl3(bz.Name(), "Created viewchange measure")
	bz.vcMeasure = monitor.NewTimeMeasure("viewchange")
	vc := newViewChange()
	var err error
	for _, n := range bz.Tree().List() {
		// don't send to ourself
		if n.Id.Equals(bz.TreeNode().Id) {
			continue
		}
		err = bz.SendTo(n, vc)
		if err != nil {
			dbg.Error(bz.Name(), "Error sending view change", err)
		}
	}
}

// viewChange is simply the last hash / id of the previous leader.
type viewChange struct {
	LastBlock [sha256.Size]byte
}

// newViewChange creates a new view change.
func newViewChange() *viewChange {
	res := &viewChange{}
	for i := 0; i < sha256.Size; i++ {
		res.LastBlock[i] = 0
	}
	return res
}

// handleViewChange receives a view change request and if received more than
// 2/3, accept the view change.
func (bz *ByzCoin) handleViewChange(tn *sda.TreeNode, vc *viewChange) error {
	bz.vcCounter++
	// only do it once
	if bz.vcCounter == bz.viewChangeThreshold {
		if bz.vcMeasure != nil {
			bz.vcMeasure.Record()
		}
		if bz.IsRoot() {
			dbg.Lvl3(bz.Name(), "Viewchange threshold reached (2/3) of all nodes")
			go bz.Done()
			//	bz.endProto.Start()
		}
		return nil
	}
	return nil
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bz *ByzCoin) nodeDone() bool {
	dbg.Lvl3(bz.Name(), "nodeDone()      ----- ")
	bz.doneProcessing <- true
	dbg.Lvl3(bz.Name(), "nodeDone()      +++++  ", bz.onDoneCallback)
	if bz.onDoneCallback != nil {
		bz.onDoneCallback()
	}
	return true
}
