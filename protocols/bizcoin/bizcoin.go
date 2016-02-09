package bizcoin

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
	"math"
	"time"
)

type BizCoin struct {
	// the node we are represented-in
	*sda.Node
	// the suite we use
	suite abstract.Suite
	// aggregated public key of the peers
	aggregatedPublic abstract.Point
	// prepare-round cosi
	prepare cosi.Cosi
	// commit-round cosi
	commit cosi.Cosi
	// channel for announcement
	announceChan chan announceChan
	// channel for commitment
	commitChan chan commitChan
	// Two channels for the challenge through the 2 rounds: difference is that
	// during the commit round, we need the previous signature of the "prepare"
	// round.
	// channel for challenge during the prepare phase
	challengePrepapreChan chan challengePrepareChan
	// channel for challenge during the commit phase
	challengeCommitChan chan challengeCommitChan
	// channel for response
	responseChan chan responseChan

	// size of the block == number of transactions per blocks
	blockSize int
	// temporary block to pass up between the two rounds (prepare + commits)
	tempBlock *blockchain.TrBlock
	// transaction_pool is a slice of transactions that contains transctions
	// coming from clients
	transaction_pool []blockchain.Tx
	// last block computed
	lastBlock string
	// last key block computed
	lastKeyBlock string
	// channel used to wait for the verification of the block
	verifyBlockChan chan bool
	// temporary buffer of "prepare" commitments
	tempPrepareCommit []*cosi.Commitment
	// temporary buffer of "commit" commitments
	tempCommitCommit []*cosi.Commitment
	// temporary buffer of "prepare" responses
	tempPrepareResponse []*cosi.Response
	// temporary buffer of "commit" responses
	tempCommitResponse []*cosi.Response

	// refusal to sign for the commit phase or not. This flag is set during the
	// Challenge of the commit phase and will be used during the response of the
	// commit phase to put an exception or to sign.
	signRefusal bool
}

func NewBizCoinProtocol(n *sda.Node) (*BizCoin, error) {
	// create the bizcoin
	bz := new(BizCoin)
	bz.Node = n
	bz.suite = n.Suite()
	bz.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bz.commit = cosi.NewCosi(n.Suite(), n.Private())
	bz.verifyBlockChan = make(chan bool)

	// compute the aggregate public key
	agg := bz.suite.Point().Null()
	for _, e := range n.EntityList().List {
		agg = agg.Add(agg, e.Public)
	}
	bz.aggregatedPublic = agg

	// register channels
	n.RegisterChannel(&bz.announceChan)
	n.RegisterChannel(&bz.commitChan)
	n.RegisterChannel(&bz.challengePrepareChan)
	n.RegisterChannel(&bz.challengeCommitChan)
	n.RegisterChannel(&bz.responseChan)

	return bz
}

func (bz *BizCoin) Start() error {

}

// Dispatch listen on the different channels
func (bz *BizCoin) Dispatch() error {
	for bz.closed {
		var err error
		select {
		case msg := <-bz.announceChan:
			err = bz.handleAnnouncement(msg.BizCoinAnnounce)
		case msg := <-bz.commitChan:
			switch msg.BizCoinCommitment.TYPE {
			case ROUND_PREPARE:

			case ROUND_COMMIT:

			}
		case msg := <-bz.challengePrepareChan:
			err = bz.handlePrepareChallenge(msg)
		case msg := <-bz.challengeCommitChan:
			err = bz.handleCommitChallenge(msg)
		case msg := <-bz.responseChan:
			switch msg.BizCoinResponse.TYPE {
			case ROUND_PREPARE:

			case ROUND_COMMIT:

			}
		}
		if err != nil {
			dbg.Error("Error treating the messages :", err)
		}
	}
}

func (bz *BizCoin) listen() {

}

func (bz *BizCoin) handleNewTransaction(tr blockchain.Tx) error {

}

// handleAnnouncement pass the announcement to the right CoSi struct.
func (bz *BizCoin) handleAnnouncement(ann BizCoinAnnounce) error {
	var announcement BizCoinAnnounce
	switch ann.TYPE {
	case ROUND_PREPARE:
		announcement = &BizCoinAnnounce{
			TYPE:         ROUND_PREPARE,
			Announcement: bz.prepare.Announce(&ann.Announcement),
		}
		if bz.IsLeaf() {
			return bz.startPrepareCommitment()
		}
	case ROUND_COMMIT:
		announcement = &BizCoinAnnounce{
			TYPE:         ROUND_COMMIT,
			Announcement: bz.commit.Announce(&ann.Announcement),
		}
		if bz.IsLeaf() {
			return bz.startCommitCommitment()
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
func (bz *BizCoin) startPrepareCommitment() error {
	cm := bz.prepare.CreateCommitment()
	return bz.SendTo(bz.Parent(), &BizCoinCommitment{TYPE: ROUND_PREPARE, Commitment: cm})
}

// startCommitCommitment send the first commitment up the tree for the
// commitment round.
func (bz *BizCoin) startCommitCommitment() error {
	cm := bz.commit.CreateCommitment()
	return bz.SendTo(bz.Parent(), &BizCoinCommitment{TYPE: ROUND_COMMIT, Commitment: cm})
}

// handle the arrival of a commitment
func (bz *BizCoin) handleCommit(ann BizCoinCommitment) error {
	var commitment *BizCoinCommitment
	// store it and check if we have enough commitments
	switch ann.TYPE {
	case ROUND_PREPARE:
		bz.tempPrepareCommit = append(bz.tempPrepareCommit, &ann.Commitment)
		if len(bz.tempPrepareCommit) < len(bz.Children()) {
			return nil
		}
		commit := bz.prepare.Commit(bz.tempPrepareCommit)
		if bz.IsRoot() {
			return bz.startPrepareChallenge()
		}
		commitment = &BizCoinCommitment{
			TYPE:       ROUND_PREPARE,
			Commitment: commit,
		}
	case ROUND_COMMIT:
		bz.tempCommitCommit = append(bz.tempCommitCommit, &ann.Commitment)
		if len(bz.tempCommitCommit) < len(bz.Children()) {
			return nil
		}
		commit := bz.commit.Commit(bz.tempCommitCommit)
		if bz.IsRoot() {
			return bz.startCommitChallenge()
		}
		commitment = &BizCoinCommitment{
			TYPE:       ROUND_COMMIT,
			Commitment: commit,
		}
	}
	return bz.SendTo(bz.Parent(), commitment)
}

// startPrepareChallenge create the challenge and send its down the tree
func (bz *BizCoin) startPrepareChallenge() error {
	// Get the block we want to sign
	trblock, err := bz.getBlock(bz.blockSize)
	if err != nil {
		return err
	}
	bz.tempBlock = trblock
	// make the challenge out of it
	marshalled, err := json.Marshal(trblock)
	if err != nil {
		return err
	}
	ch, err := bz.prepare.CreateChallenge(marshalled)
	if err != nil {
		return err
	}
	bizChal := &BizCoinChallenge{
		TYPE:      ROUND_PREPARE,
		Challenge: ch,
		TrBlock:   trblock,
	}
	// send to children
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bizChal)
	}
	return err
}

// startCommitChallenge waits the end of the "prepare" round.
// Then it creates the challenge and sends it along with the
// "prepare" signature down the tree.
func (bz *BizCoin) startCommitChallenge() error {
	// wait the end of prepare
	// TODO timeout ?
	<-bz.prepareFinishedChan

	// create the challenge out of it
	marshalled, err := json.Marshal(bz.tempBlock)
	if err != nil {
		return err
	}
	chal, err := bz.commit.CreateChallenge(marshalled)
	if err != ni {
		return err
	}

	// send challenge + signature
	bz := &BizCoinChallenge{
		TYPE:      ROUND_COMMIT,
		Challenge: chal,
		Signature: bz.prepare.Signature(),
	}

	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tn, bz)
	}
	return err
}

// handlePrepareChallenge receive the challenge messages for the "prepare"
// round.
func (bz *BizCoin) handlePrepareChallenge(ch BizCoinChallengePrepare) error {
	bz.tempBlock = ch.TrBlock
	// start the verification of the block
	go bz.verifyBlock(bz.tempBlock)
	// acknoledge the challenge and send its down
	chal := bz.prepare.Challenge(ch.Challenge)
	ch.Challenge = chal

	// go to response if leaf
	if bz.IsLeaf() {
		return bz.startPrepareResponse()
	}

	var err error
	for _, tn := range bz.Children() {
		err = bu.SendTo(tn, ch)
	}
	return err
}

// handleCommitChallenge will verify the signature + check if no more than 1/3
// of participants refused to sign. If so, it passes up the responses.
func (bz *BizCoin) handleCommitChallenge(ch BizCoinChallengeCommit) error {
	// marshal the block
	marshalled, err := json.Marshal(bz.tempBlock)
	if err != nil {
		return err
	}

	// verify if the signature is correct
	if err := cosi.VerifyCosiSignatureWithException(bz.suite, bz.aggregatedPublic, marshalled, ch.Signature, ch.Exceptions); err != nil {
		dbg.Error("Verification of the signature failed:", err)
		bz.signRefusal = true
	}

	// Verify if we have no more than 1/3 failed nodes
	threshold := math.Ceil(float64(len(bz.EntityList().List)) / 3.0)
	if len(ch.Exceptions) > int(threshold) {
		dbg.Errorf("More than 1/3 (%d/%d) refused to sign ! ABORT", len(ch.Exceptions), len(bz.EntityList().List))
		bz.signRefusal = true
	}

	if bz.IsLeaf() {
		return bz.startCommitResponse()
	}
	// send it down
	var err error
	for _, tn := range bz.Children() {
		err = bz.SendTo(tj, ch)
	}
	return nil
}

// startPrepareResponse wait the verification of the block and then start the
// challenge process
func (bz *BizCoin) startPrepareResponse() error {
	// create response
	resp, err := bz.prepare.CreateResponse()
	if err != nil {
		return nil, err
	}
	// wait the verification
	bzr, ok := bz.waitResponseVerification()
	if ok {
		// apend response only if OK
		bz.Response = resp
	}
	// send to parent
	return bz.SendTo(bz.Parent(), bzr)
}

// startCommitResponse will create the response for the commit phase and send it
// up. It will not create the response if it decided the signature is wrong from
// the prepare phase.
func (bz *BizCoin) startCommitResponse() error {
	bz := &BizCoinResponse{
		TYPE: ROUND_COMMIT,
	}
	// if i dont want to sign
	if bz.signRefusal {
		bz.Exceptions = append(bz.Exceptions, cosi.Exception{bz.Public(), bz.commit.GetCommitment()})
	} else {
		// otherwise i create the response
		resp, err := bz.commit.CreateResponse()
		if err != nil {
			return err
		}
		bz.Response = resp
	}
	// send to parent
	return bz.SendTo(bz.Parent(), bz)
}

func (bz *BizCoin) handleCommitResponse(bz BizCoinResponse) error {
	// check if we have enough
	bz.tempCommitResponse = append(bz.tempCommitResponse, bz.Response)
	if len(bz.tempCommitResponse) < len(bz.Children()) {
		return nil
	}

	if bz.signRefusal {
		bz.Exceptions = append(bz.Exceptions, cosi.Exception{bz.Public(), bz.commit.GetCommitment()})
	} else {
		resp, err := bz.commit.Response(bz.tempCommitResponse)
		if err != nil {
			return err
		}
		bz.Response = resp
	}

	// if root we have finished
	if bz.IsRoot() {
		return nil
	}
	// otherwise , send the response up
	return bz.SendTo(bz.Parent(), bz)
}

// handlePrepapreResponse
func (bz *BizCoin) handlePrepareResponse(bz BizCoinResponse) error {
	// check if we have enough
	bz.tempPrepareResponse = append(bz.tempPrepareResponse, bz.Response)
	if len(bz.tempPrepareResponse) < len(bz.Children()) {
		return nil
	}

	// wait for verification
	bz, ok := bz.WaitResponseVerification()
	if ok {
		// append response
		resp, err := bz.prepare.Response(bz.tempPrepareResponse)
		if err != nil {
			return err
		}
		bz.Response = resp
	}
	// if I'm root, we are finished, let's notify the "commit" round
	if bz.IsRoot() {
		bz.prepareFinishedChan <- true
		return nil
	}
	// send up
	return bz.SendTo(pc.Parent(), bz)
}

// computePrepareResponse wait the end of the verification and returns the
// BizCoinResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification is NOT correct
func (bz *BizCoin) waitResponseVerification() (*BizCoinResponse, bool) {
	bz := &BizCoinResponse{
		TYPE:     ROUND_PREPARE,
		Response: resp,
	}
	// wait the verification
	verified := <-bz.verifyBlockChan
	if !verified {
		// append our exception
		bz.Exceptions = append(bz.Exceptions, cosi.Exception{bz.Public(), bz.prepare.GetCommitment()})
		return bz, false
	}

	return bz, true
}

// verifyBlock is a simulation of a real verification block algorithm
func (bz *BizCoin) verifyBlock(block *TrBlock) {
	//We measure the average block verification delays is 174ms for an average
	//block of 500kB.
	//To simulate the verification cost of bigger blocks we multipley 174ms
	//times the size/500*1024
	var n time.Duration
	n = time.Duration(s / (500 * 1024))
	time.Sleep(150 * time.Millisecond * n) //verification of 174ms per 500KB simulated
	// verification of the header
	verified := block.Header.Parent == round.lastBlock && block.Header.ParentKey == round.lastKeyBlock
	verified = verified && block.Header.MerkleRoot == HashRootTransactions(block.TransactionList)
	verified = verified && block.HeaderHash == HashHeader(block.Header)
	// notify it
	bz.verifyBlockChan <- verified
}

// getblock returns the next block available from the transaction pool.
func (bz *BizCoin) getblock(n int) (*blockchain.TrBlock, error) {
	bz.transactionLock.Lock()
	trb = new(blockchain.TrBlock)
	if len(bz.transaction_pool) < 1 {
		bz.transactionLock.Unlock()
		return nil, errors.New("no transaction available")
	}

	trlist := blockchain.NewTransactionList(bz.transaction_pool, n)
	header := blockchain.NewHeader(trlist, bz.lastBlock, bz.lastKeyBlock)
	trblock := blockchain.NewTrBlock(trlist, header)
	bz.transaction_pool = bz.transaction_pool[trblock.TransactionList.TxCnt:]
	bz.transactionLock.Unlock()
	return trblock, nil
}
