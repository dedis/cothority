package bizcoin

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
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"
	"github.com/dedis/cothority/protocols/end"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

const (
	NotFail int = iota
	FailCompletely
	FailWrongBlocks
)

type BizCoin struct {
	// the node we are represented-in
	*sda.Node
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
	// lock associated with the transaction pool for concurrent access
	transactionLock *sync.Mutex
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
	rootFailMode          uint
	onAnnouncementPrepare func()
	onResponsePrepareDone func()
	onChallengeCommit     func()
	onChallengeCommitDone func()
	// view change setup and measurement
	viewchangeChan chan struct {
		*sda.TreeNode
		viewChange
	}
	vcMeasure      *monitor.Measure
	doneSigning    chan bool
	doneLock       sync.Mutex
	threshold      int
	vcCounter      int
	doneProcessing chan bool

	endProto *end.EndProtocol

	finalSignature *BlockSignature
}

func NewBizCoinProtocol(n *sda.Node) (*BizCoin, error) {
	// create the bizcoin
	bz := new(BizCoin)
	bz.Node = n
	bz.suite = n.Suite()
	bz.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bz.commit = cosi.NewCosi(n.Suite(), n.Private())
	bz.verifyBlockChan = make(chan bool)
	bz.doneProcessing = make(chan bool, 2)
	bz.doneSigning = make(chan bool, 1)
	bz.timeoutChan = make(chan uint64, 1)

	bz.endProto, _ = end.NewEndProtocol(n)
	bz.endProto.RegisterEndCallback(bz.onEndCallback)
	bz.aggregatedPublic = n.EntityList().Aggregate
	bz.threshold = int(math.Ceil(float64(len(bz.EntityList().List)) / 3.0))

	// register channels
	n.RegisterChannel(&bz.announceChan)
	n.RegisterChannel(&bz.commitChan)
	n.RegisterChannel(&bz.challengePrepareChan)
	n.RegisterChannel(&bz.challengeCommitChan)
	n.RegisterChannel(&bz.responseChan)
	n.RegisterChannel(&bz.viewchangeChan)

	n.OnDoneCallback(bz.nodeDone)

	go bz.listen()
	return bz, nil
}

func NewBizCoinRootProtocol(n *sda.Node, transactions []blkparser.Tx, timeOutMs uint64, failMode uint) (*BizCoin, error) {
	bz, err := NewBizCoinProtocol(n)
	if err != nil {
		return nil, err
	}

	bz.rootFailMode = failMode
	bz.tempBlock, err = bz.getBlock(transactions)
	bz.rootTimeout = timeOutMs
	return bz, err
}

// Start will start both rounds "prepare" and "commit" at same time. The
// "commit" round will wait the end of the "prepare" round during its challenge
// phase.
func (bz *BizCoin) Start() error {

	if err := bz.startAnnouncementPrepare(); err != nil {
		return err
	}
	return bz.startAnnouncementCommit()
}

// Dispatch listen on the different channels
func (bz *BizCoin) Dispatch() error {
	return nil
}
func (bz *BizCoin) listen() {
	// FIXME handle different failure modes
	fail := (bz.rootFailMode != 0) && bz.IsRoot()
	dbg.Print(bz.Name(), " is failing ? =", fail)
	var timeoutStarted bool
	for {
		var err error
		select {
		// Announcement
		case msg := <-bz.announceChan:
			err = bz.handleAnnouncement(msg.BizCoinAnnounce)
			// Commitment
		case msg := <-bz.commitChan:
			if !fail {
				err = bz.handleCommit(msg.BizCoinCommitment)
			}
			// Challenge
		case msg := <-bz.challengePrepareChan:
			if !fail {
				err = bz.handleChallengePrepare(&msg.BizCoinChallengePrepare)
			}
		case msg := <-bz.challengeCommitChan:
			if !fail {
				err = bz.handleChallengeCommit(&msg.BizCoinChallengeCommit)
			}
			// Response
		case msg := <-bz.responseChan:
			if !fail {
				switch msg.BizCoinResponse.TYPE {
				case ROUND_PREPARE:
					err = bz.handleResponsePrepare(&msg.BizCoinResponse)
				case ROUND_COMMIT:
					err = bz.handleResponseCommit(&msg.BizCoinResponse)
				}
			}
			// start the timer
		case timeout := <-bz.timeoutChan:
			if timeoutStarted {
				continue
			}
			timeoutStarted = true
			go bz.startTimer(timeout)
			// receive view change
		case msg := <-bz.viewchangeChan:
			err = bz.handleViewChange(msg.TreeNode, &msg.viewChange)
		case <-bz.doneProcessing:
			dbg.Lvl2(bz.Name(), "BizCoin Dispatches stop.")
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
func (bz *BizCoin) startAnnouncementPrepare() error {
	if bz.onAnnouncementPrepare != nil {
		go bz.onAnnouncementPrepare()
	}

	ann := bz.prepare.CreateAnnouncement()
	bza := &BizCoinAnnounce{
		TYPE:         ROUND_PREPARE,
		Announcement: ann,
		Timeout:      bz.rootTimeout,
	}
	dbg.Lvl3("BizCoin Start Announcement (PREPARE)")
	return bz.sendAnnouncement(bza)
}

// startAnnouncementCommit create the announcement for the commit phase and
// sends it down the tree.
func (bz *BizCoin) startAnnouncementCommit() error {
	ann := bz.commit.CreateAnnouncement()
	bza := &BizCoinAnnounce{
		TYPE:         ROUND_COMMIT,
		Announcement: ann,
	}
	dbg.Lvl3(bz.Name(), "BizCoin Start Announcement (COMMIT)")
	return bz.sendAnnouncement(bza)
}

func (bz *BizCoin) sendAnnouncement(bza *BizCoinAnnounce) error {
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bza)
	}
	return err
}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bz *BizCoin) handleAnnouncement(ann BizCoinAnnounce) error {
	var announcement = new(BizCoinAnnounce)

	switch ann.TYPE {
	case ROUND_PREPARE:
		announcement = &BizCoinAnnounce{
			TYPE:         ROUND_PREPARE,
			Announcement: bz.prepare.Announce(ann.Announcement),
			Timeout:      ann.Timeout,
		}

		bz.timeoutChan <- ann.Timeout
		// give the timeout
		dbg.Lvl3(bz.Name(), "BizCoin Handle Announcement PREPARE")

		if bz.IsLeaf() {
			return bz.startCommitmentPrepare()
		}
	case ROUND_COMMIT:
		announcement = &BizCoinAnnounce{
			TYPE:         ROUND_COMMIT,
			Announcement: bz.commit.Announce(ann.Announcement),
		}
		dbg.Lvl3(bz.Name(), "BizCoin Handle Announcement COMMIT")

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
func (bz *BizCoin) startCommitmentPrepare() error {
	cm := bz.prepare.CreateCommitment()
	err := bz.SendTo(bz.Parent(), &BizCoinCommitment{TYPE: ROUND_PREPARE, Commitment: cm})
	dbg.Lvl3(bz.Name(), "BizCoin Start Commitment PREPARE")
	return err
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bz *BizCoin) startCommitmentCommit() error {
	cm := bz.commit.CreateCommitment()

	err := bz.SendTo(bz.Parent(), &BizCoinCommitment{TYPE: ROUND_COMMIT, Commitment: cm})
	dbg.Lvl3(bz.Name(), "BizCoin Start Commitment COMMIT", err)
	return err
}

// handle the arrival of a commitment
func (bz *BizCoin) handleCommit(ann BizCoinCommitment) error {
	var commitment *BizCoinCommitment
	// store it and check if we have enough commitments
	switch ann.TYPE {
	case ROUND_PREPARE:
		bz.tpcMut.Lock()
		bz.tempPrepareCommit = append(bz.tempPrepareCommit, ann.Commitment)
		if len(bz.tempPrepareCommit) < len(bz.Children()) {
			bz.tpcMut.Unlock()
			return nil
		}
		commit := bz.prepare.Commit(bz.tempPrepareCommit)
		bz.tpcMut.Unlock()
		if bz.IsRoot() {
			dbg.Print(bz.Name(), "handle Commit PREPARE => WIll sTART Challenge !")
			return bz.startChallengePrepare()
		}
		commitment = &BizCoinCommitment{
			TYPE:       ROUND_PREPARE,
			Commitment: commit,
		}
		dbg.Lvl3(bz.Name(), "BizCoin handle Commit PREPARE")
	case ROUND_COMMIT:
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
		commitment = &BizCoinCommitment{
			TYPE:       ROUND_COMMIT,
			Commitment: commit,
		}
		dbg.Lvl3(bz.Name(), "BizCoin handle Commit COMMIT")
	}
	err := bz.SendTo(bz.Parent(), commitment)
	dbg.Print(bz.Name(), "HandleCommit() sent to parent!")
	return err
}

// startPrepareChallenge create the challenge and send its down the tree
func (bz *BizCoin) startChallengePrepare() error {
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
	bizChal := &BizCoinChallengePrepare{
		TYPE:      ROUND_PREPARE,
		Challenge: ch,
		TrBlock:   trblock,
	}

	go bz.verifyBlock(bz.tempBlock)
	dbg.Lvl3(bz.Name(), "BizCoin Start Challenge PREPARE")
	// send to children
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bizChal)
	}
	return err
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bz *BizCoin) startChallengeCommit() error {
	if bz.onChallengeCommit != nil {
		bz.onChallengeCommit()
	}
	// create the challenge out of it
	marshalled, err := json.Marshal(bz.tempBlock)
	if err != nil {
		return err
	}
	chal, err := bz.commit.CreateChallenge(marshalled)
	if err != nil {
		return err
	}

	// send challenge + signature
	bzc := &BizCoinChallengeCommit{
		TYPE:      ROUND_COMMIT,
		Challenge: chal,
		Signature: bz.prepare.Signature(),
	}
	dbg.Lvl3("BizCoin Start Challenge COMMIT")
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bzc)
	}
	return err
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bz *BizCoin) handleChallengePrepare(ch *BizCoinChallengePrepare) error {
	bz.tempBlock = ch.TrBlock
	// start the verification of the block
	go bz.verifyBlock(bz.tempBlock)
	// acknoledge the challenge and send its down
	chal := bz.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	dbg.Lvl3(bz.Name(), "BizCoin handle Challenge PREPARE")
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
func (bz *BizCoin) handleChallengeCommit(ch *BizCoinChallengeCommit) error {
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
	dbg.Lvl3("BizCoin handle Challenge COMMIT")
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
func (bz *BizCoin) startResponsePrepare() error {
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
	dbg.Lvl3(bz.Name(), "BizCoin Start Response PREPARE")
	// send to parent
	return bz.SendTo(bz.Parent(), bzr)
}

// startCommitResponse will create the response for the commit phase and send it
// up. It will not create the response if it decided the signature is wrong from
// the prepare phase.
func (bz *BizCoin) startResponseCommit() error {
	bzr := &BizCoinResponse{
		TYPE: ROUND_COMMIT,
	}
	// if i dont want to sign
	if bz.signRefusal {
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{bz.Public(), bz.commit.GetCommitment()})
	} else {
		// otherwise i create the response
		resp, err := bz.commit.CreateResponse()
		if err != nil {
			return err
		}
		bzr.Response = resp
	}
	dbg.Lvl3(bz.Name(), "BizCoin Start Response COMMIT")
	// send to parent
	err := bz.SendTo(bz.Parent(), bzr)
	bz.Done()
	return err
}

// handleResponseCommit handles the responses for the commit round during the
// response phase.
func (bz *BizCoin) handleResponseCommit(bzr *BizCoinResponse) error {
	// check if we have enough
	// FIXME possible data race
	bz.tcrMut.Lock()
	bz.tempCommitResponse = append(bz.tempCommitResponse, bzr.Response)

	if len(bz.tempCommitResponse) < len(bz.Children()) {
		bz.tcrMut.Unlock()
		return nil
	}

	if bz.signRefusal {
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{bz.Public(), bz.commit.GetCommitment()})
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
	dbg.Lvl3(bz.Name(), "BizCoin handle Response COMMIT (refusal=", bz.signRefusal, ")")
	// if root we have finished
	if bz.IsRoot() {
		sig := bz.Signature()
		if bz.onChallengeCommitDone != nil {
			bz.onChallengeCommitDone()
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

func (bz *BizCoin) handleResponsePrepare(bzr *BizCoinResponse) error {
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

	dbg.Lvl3("BizCoin Handle Response PREPARE")
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
// BizCoinResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bz *BizCoin) waitResponseVerification() (*BizCoinResponse, bool) {
	bzr := &BizCoinResponse{
		TYPE: ROUND_PREPARE,
	}
	// wait the verification
	verified := <-bz.verifyBlockChan
	if !verified {
		// append our exception
		bzr.Exceptions = append(bzr.Exceptions, cosi.Exception{bz.Public(), bz.prepare.GetCommitment()})
		bz.sendAndMeasureViewchange()
		return bzr, false
	}

	return bzr, true
}

// verifyBlock is a simulation of a real verification block algorithm
func (bz *BizCoin) verifyBlock(block *blockchain.TrBlock) {
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
	verified := block.Header.Parent == bz.lastBlock && block.Header.ParentKey == bz.lastKeyBlock
	verified = verified && block.Header.MerkleRoot == blockchain.HashRootTransactions(block.TransactionList)
	verified = verified && block.HeaderHash == blockchain.HashHeader(block.Header)
	// notify it
	dbg.Lvl3("Verification of the block done =", verified)
	bz.verifyBlockChan <- verified
}

// getblock returns the next block available from the transaction pool.
func (bz *BizCoin) getBlock(transactions []blkparser.Tx) (*blockchain.TrBlock, error) {
	if len(transactions) < 1 {
		return nil, errors.New("no transaction available")
	}

	trlist := blockchain.NewTransactionList(transactions, len(transactions))
	header := blockchain.NewHeader(trlist, bz.lastBlock, bz.lastKeyBlock)
	trblock := blockchain.NewTrBlock(trlist, header)
	return trblock, nil
}

// Signature will generate the final signature, the output of the BizCoin
// protocol.
func (bz *BizCoin) Signature() *BlockSignature {
	return &BlockSignature{
		Sig:        bz.commit.Signature(),
		Block:      bz.tempBlock,
		Exceptions: bz.tempExceptions,
	}
}

func (bz *BizCoin) RegisterOnDone(fn func()) {
	bz.onDoneCallback = fn
}

func (bz *BizCoin) RegisterOnSignatureDone(fn func(*BlockSignature)) {
	bz.onSignatureDone = fn
}

func (bz *BizCoin) startTimer(millis uint64) {
	dbg.Lvl3(bz.Name(), "Started timer (", millis, ")...")
	select {
	case <-bz.doneSigning:
		return
	case <-time.After(time.Millisecond * time.Duration(millis)):
		bz.sendAndMeasureViewchange()
	}
}

func (bz *BizCoin) sendAndMeasureViewchange() {
	dbg.Lvl3(bz.Name(), "Created viewchange measure")
	bz.vcMeasure = monitor.NewMeasure("viewchange")
	vc := newViewChange()
	var err error
	for _, n := range bz.Tree().ListNodes() {
		// don't send to ourself
		if uuid.Equal(n.Id, bz.TreeNode().Id) {
			continue
		}
		err = bz.SendTo(n, vc)
		if err != nil {
			dbg.Error(bz.Name(), "Error sending view change", err)
		}
	}
}

type viewChange struct {
	LastBlock [sha256.Size]byte
}

func newViewChange() *viewChange {
	res := &viewChange{}
	for i := 0; i < sha256.Size; i++ {
		res.LastBlock[i] = 0
	}
	return res
}

func (bz *BizCoin) handleViewChange(tn *sda.TreeNode, vc *viewChange) error {
	bz.vcCounter++
	dbg.Print(bz.Name(), "Received ViewChange (", bz.vcCounter, "/", 2*bz.threshold, ") from", tn.Name())
	// only do it once
	if bz.vcCounter == 2*bz.threshold {
		dbg.Lvl3(bz.Name(), "Viewchange threshold reached (2/3) of all nodes")
		if bz.vcMeasure != nil {
			bz.vcMeasure.Measure()
		}
		if bz.IsRoot() {
			bz.endProto.Start()
		}
		return nil
	}
	return nil
}

// OnAnnouncementPrepare registers a function which will be called when
// ResponsePrepare round is started
func (bz *BizCoin) OnAnnouncementPrepare(fn func()) {
	bz.onAnnouncementPrepare = fn
}

// OnAnnouncementPrepareDone registers a function which will be called when
// ResponsePrepare round is finished
func (bz *BizCoin) OnAnnouncementPrepareDone(fn func()) {
	bz.onResponsePrepareDone = fn
}

// OnChallengeCommit registers a function which will be called when
// ChallengeCommit round is started
func (bz *BizCoin) OnChallengeCommit(fn func()) {
	bz.onChallengeCommit = fn
}

// OnChallengeCommitDone registers a function which will be called when
// ChallengeCommit round is finished
func (bz *BizCoin) OnChallengeCommitDone(fn func()) {
	bz.onChallengeCommitDone = fn
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bz *BizCoin) nodeDone() bool {
	bz.doneProcessing <- true
	if bz.onDoneCallback != nil {
		bz.onDoneCallback()
	}
	return true
}

func (bz *BizCoin) onEndCallback() {
	bz.doneProcessing <- true
}
