package bftcosi

/*
BFTCoSi is a byzantine-fault-tolerant protocol to sign a message given a
verification-function. It uses two rounds of signing - the first round
indicates the willingness of the rounds to sign the message, and the second
round is only started if at least a 'threshold' number of nodes signed off in
the first round.
*/

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/cosi/crypto"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// VerificationFunction can be passes to each protocol node. It will be called
// (in a go routine) during the (start/handle) challenge prepare phase of the
// protocol. The passed message is the same as sent in the challenge phase.
// The `Data`-part is only to help the VerificationFunction do it's job. In
// the case of the services, this part should be replaced by the correct
// passing of the service-configuration-data, which is not done yet.
type VerificationFunction func(Msg []byte, Data []byte) bool

// ProtocolBFTCoSi is the main struct for running the protocol
type ProtocolBFTCoSi struct {
	// the node we are represented-in
	*onet.TreeNodeInstance
	// all data we need during the signature-rounds
	collectStructs

	// The message that will be signed by the BFTCosi
	Msg []byte
	// Data going along the msg to the verification
	Data []byte
	// Timeout is how long to wait while gathering commits. It should be
	// set by the caller after creating the protocol.
	Timeout time.Duration
	// AllowedExceptions for how much exception is allowed. If more than
	// AllowedExceptions number of conodes refuse to sign, no signature
	// will be created. It is set to (n-1)/3 by default.
	AllowedExceptions int
	// last block computed
	lastBlock string
	// our index in the Roster list
	index int

	// onet-channels used to communicate the protocol
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

	// Internal communication channels
	// channel used to wait for the verification of the block
	verifyChan chan bool

	// handler-functions
	// onDone is the callback that will be called at the end of the
	// protocol when all nodes have finished. Either at the end of the response
	// phase of the commit round or at the end of a view change.
	onDone func()
	// onSignatureDone is the callback that will be called when a signature has
	// been generated ( at the end of the response phase of the commit round)
	onSignatureDone func(*BFTSignature)
	// VerificationFunction will be called
	// during the (start/handle) challenge prepare phase of the protocol
	VerificationFunction VerificationFunction
	// closing is true if the node is being shut down
	closing bool
	// mutex for closing down properly
	closingMutex sync.Mutex
	// successful is a flag to indicate whether the protocol finished successfully
	successful    bool
	successfulMut sync.Mutex
}

// collectStructs holds the variables that are used during the protocol to hold
// messages
type collectStructs struct {
	// prepare-round cosi
	prepare *crypto.CoSi
	// commit-round cosi
	commit *crypto.CoSi

	// prepareSignature is the signature generated during the prepare phase
	// This signature is adapted according to the exceptions that occured during
	// the prepare phase.
	prepareSignature []byte

	// mutex for all temporary structures
	tmpMutex sync.Mutex
	// exceptions given during the rounds that is used in the signature
	tempExceptions []Exception
	// respondedNodesPrepare stores a list of nodes that sent a commit
	// message in the prepare phase
	respondedNodesPrepare map[string]*onet.TreeNode
	// respondedNodesCommit stores a list of nodes that sent a commit
	// message in the commit phase
	respondedNodesCommit map[string]*onet.TreeNode
	// temporary buffer of "prepare" commitments
	tempPrepareCommit []kyber.Point
	// temporary buffer of "commit" commitments
	tempCommitCommit []kyber.Point
	// temporary buffer of "prepare" responses
	tempPrepareResponse []kyber.Scalar
	// temporary buffer of "commit" responses
	tempCommitResponse []kyber.Scalar

	committedInPreparePhase bool
	committedInCommitPhase  bool
}

// NewBFTCoSiProtocol returns a new bftcosi struct
func NewBFTCoSiProtocol(n *onet.TreeNodeInstance, verify VerificationFunction) (*ProtocolBFTCoSi, error) {
	t := (len(n.Tree().List()) - 1) / 3
	// initialize the bftcosi node/protocol-instance
	bft := &ProtocolBFTCoSi{
		TreeNodeInstance: n,
		collectStructs: collectStructs{
			prepare:               crypto.NewCosi(n.Suite(), n.Private(), n.Roster().Publics()),
			commit:                crypto.NewCosi(n.Suite(), n.Private(), n.Roster().Publics()),
			respondedNodesPrepare: make(map[string]*onet.TreeNode),
			respondedNodesCommit:  make(map[string]*onet.TreeNode),
		},
		verifyChan:           make(chan bool),
		VerificationFunction: verify,
		AllowedExceptions:    t,
		Msg:                  make([]byte, 0),
		Data:                 make([]byte, 0),
		Timeout:              5 * time.Second,
	}

	idx, _ := n.Roster().Search(bft.ServerIdentity().ID)
	bft.index = idx

	// Registering channels.
	err := bft.RegisterChannels(&bft.announceChan,
		&bft.challengePrepareChan, &bft.challengeCommitChan,
		&bft.commitChan, &bft.responseChan)
	if err != nil {
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
	go func() {
		bft.startAnnouncement(RoundCommit)
	}()
	return nil
}

// Dispatch makes sure that the order of the messages is correct by waiting
// on each channel in the correct order.
// By closing the channels for the leafs we can avoid having
// `if !bft.IsLeaf` in the code.
func (bft *ProtocolBFTCoSi) Dispatch() error {
	defer bft.Done()

	bft.closingMutex.Lock()
	if bft.closing {
		return nil
	}
	// Close unused channels for the leaf nodes, so they won't listen
	// and block on those messages which they will only send but never
	// receive.
	// Unfortunately this is not possible for the announce- and
	// challenge-channels, so the root-node has to send the message
	// to the channel instead of using a simple `SendToChildren`.
	if bft.IsLeaf() {
		close(bft.commitChan)
		close(bft.responseChan)
	}
	bft.closingMutex.Unlock()

	// Start prepare round
	if err := bft.handleAnnouncement(<-bft.announceChan); err != nil {
		return err
	}
	if !bft.IsLeaf() {
		if err := bft.handleCommitmentPrepare(bft.commitChan); err != nil {
			return err
		}
	}

	// Start commit round
	if err := bft.handleAnnouncement(<-bft.announceChan); err != nil {
		return err
	}
	if !bft.IsLeaf() {
		if err := bft.handleCommitmentCommit(bft.commitChan); err != nil {
			return err
		}
	}

	var gotChallengePrepare bool
	for {
		select {
		case c, ok := <-bft.challengePrepareChan:
			if !ok {
				return errors.New("Protocol cleanup while waiting for challengePrepare")
			}
			if gotChallengePrepare {
				log.Warn(bft.Name(), "got prepare-challenge after prepare-round is done - potential ddos")
				continue
			}
			// Finish the prepare round
			if err := bft.handleChallengePrepare(c); err != nil {
				return err
			}
			if !bft.IsLeaf() {
				if err := bft.handleResponsePrepare(bft.responseChan); err != nil {
					return err
				}
			}
			gotChallengePrepare = true
		case c, ok := <-bft.challengeCommitChan:
			if !ok {
				return errors.New("Protocol cleanup while waiting for challengeCommit")
			}

			// Finish the commit round
			if err := bft.handleChallengeCommit(c); err != nil {
				return err
			}
			if !bft.IsLeaf() {
				if err := bft.handleResponseCommit(bft.responseChan); err != nil {
					return err
				}
			}
			bft.successfulMut.Lock()
			bft.successful = true
			bft.successfulMut.Unlock()
			// we return here even when prepare is not done yet,
			// because we may not have received the challenge for
			// the prepare phase
			return nil
		case <-time.After(bft.Timeout):
			return errors.New("timeout waiting for challenge - " +
				"might be OK because the protocol already finished")
		}
	}
}

// Signature will generate the final signature, the output of the BFTCoSi
// protocol.  The signature contains the commit round signature, with the
// message.  Expect this function to have an undefined behavior when called
// from a non-root Node.  If the root does not finish correctly, then the
// signature will be nil.
func (bft *ProtocolBFTCoSi) Signature() *BFTSignature {

	bft.successfulMut.Lock()
	ok := bft.successful
	bft.successfulMut.Unlock()
	if !ok {
		return &BFTSignature{
			Msg: bft.Msg,
		}
	}

	// create exceptions for the nodes that were late to respond in the
	// commit phase
	var i int
	exceptions := make([]Exception, bft.AllowedExceptions)
	for _, tn := range bft.Children() {
		if _, ok := bft.respondedNodesCommit[tn.ServerIdentity.Public.String()]; !ok {
			exceptions[i] = Exception{
				Index:      tn.RosterIndex,
				Commitment: bft.Suite().Point().Null(),
			}
			i++
		}
	}

	return &BFTSignature{
		Sig:        bft.commit.Signature(),
		Msg:        bft.Msg,
		Exceptions: exceptions,
	}
}

// RegisterOnDone registers a callback to call when the bftcosi protocols has
// really finished
func (bft *ProtocolBFTCoSi) RegisterOnDone(fn func()) {
	bft.onDone = fn
}

// RegisterOnSignatureDone register a callback to call when the bftcosi
// protocol reached a signature on the block
func (bft *ProtocolBFTCoSi) RegisterOnSignatureDone(fn func(*BFTSignature)) {
	bft.onSignatureDone = fn
}

// Shutdown closes all channels in case we're done
func (bft *ProtocolBFTCoSi) Shutdown() error {
	defer func() {
		// In case the channels were already closed
		recover()
	}()
	bft.setClosing()
	close(bft.announceChan)
	close(bft.challengePrepareChan)
	close(bft.challengeCommitChan)
	if !bft.IsLeaf() {
		close(bft.commitChan)
		close(bft.responseChan)
	}
	return nil
}

// handleAnnouncement passes the announcement to the right CoSi struct.
func (bft *ProtocolBFTCoSi) handleAnnouncement(msg announceChan) error {
	ann := msg.Announce
	if bft.isClosing() {
		log.Lvl3("Closing")
		return nil
	}
	if bft.IsLeaf() {
		bft.Timeout = ann.Timeout
		return bft.startCommitment(ann.TYPE)
	}
	errs := bft.SendToChildrenInParallel(&ann)
	if len(errs) > bft.AllowedExceptions {
		return errs[0]
	}
	return nil
}

// handleCommitmentPrepare handles incoming commit messages in the prepare phase
// and then computes the aggregate commit when enough messages arrive.
// The aggregate is sent to the parent if the node is not a root otherwise it
// starts the challenge.
func (bft *ProtocolBFTCoSi) handleCommitmentPrepare(c chan commitChan) error {
	bft.tmpMutex.Lock()
	defer bft.tmpMutex.Unlock() // NOTE potentially locked for the whole timeout

	// wait until we have enough RoundPrepare commitments or timeout
	// should do nothing if `c` is closed
	if err := bft.readCommitChan(c, RoundPrepare); err != nil {
		return err
	}

	// at this point we should have n-t commit messages
	commitment := bft.prepare.Commit(bft.Suite().RandomStream(), bft.tempPrepareCommit)
	if bft.IsRoot() {
		return bft.startChallenge(RoundPrepare)
	}
	return bft.SendToParent(&Commitment{
		TYPE:       RoundPrepare,
		Commitment: commitment,
	})
}

// handleCommitmentCommit is similar to handleCommitmentPrepare except it is for
// the commit phase.
func (bft *ProtocolBFTCoSi) handleCommitmentCommit(c chan commitChan) error {
	bft.tmpMutex.Lock()
	defer bft.tmpMutex.Unlock() // NOTE potentially locked for the whole timeout

	// wait until we have enough RoundCommit commitments or timeout
	// should do nothing if `c` is closed
	if err := bft.readCommitChan(c, RoundCommit); err != nil {
		return err
	}

	// at this point we should have a threshold number of commitments
	commitment := bft.commit.Commit(bft.Suite().RandomStream(), bft.tempCommitCommit)
	if bft.IsRoot() {
		// do nothing:
		// stop the processing of the round, wait the end of
		// the "prepare" round: calls startChallengeCommit
		return nil
	}
	return bft.SendToParent(&Commitment{
		TYPE:       RoundCommit,
		Commitment: commitment,
	})
}

// handleChallengePrepare collects the challenge-messages
func (bft *ProtocolBFTCoSi) handleChallengePrepare(msg challengePrepareChan) error {
	if bft.isClosing() {
		return nil
	}
	ch := msg.ChallengePrepare
	if !bft.IsRoot() {
		bft.Msg = ch.Msg
		bft.Data = ch.Data
		// start the verification of the message
		// acknowledge the challenge and send it down
		bft.prepare.Challenge(ch.Challenge)
	}
	go func() {
		select {
		case bft.verifyChan <- bft.VerificationFunction(bft.Msg, bft.Data):
		case <-time.After(bft.Timeout):
			log.Error(bft.Name(), "verification didn't complete after", bft.Timeout)
			// we might not have a reader on bft.verifyChan if nodes exit early
			select {
			case bft.verifyChan <- false:
			default:
			}
		}
	}()
	if bft.IsLeaf() {
		return bft.handleResponsePrepare(bft.responseChan)
	}
	// only send to the committed children
	return bft.multicast(&ch, bft.respondedNodesPrepare)
}

// handleChallengeCommit verifies the signature and checks if not more than
// the threshold of participants refused to sign
func (bft *ProtocolBFTCoSi) handleChallengeCommit(msg challengeCommitChan) error {
	if bft.isClosing() {
		return nil
	}
	ch := msg.ChallengeCommit
	if !bft.IsRoot() {
		bft.commit.Challenge(ch.Challenge)
	}

	// verify if the signature is correct
	data := sha512.Sum512(ch.Signature.Msg)
	bftPrepareSig := &BFTSignature{
		Sig:        ch.Signature.Sig,
		Msg:        data[:],
		Exceptions: ch.Signature.Exceptions,
	}
	if err := bftPrepareSig.Verify(bft.Suite(), bft.Roster().Publics()); err != nil {
		return fmt.Errorf("%s: Verification of the commit-challenge signature failed: %v", bft.Name(), err)
	}

	// check if we have no more than threshold failed nodes
	if len(ch.Signature.Exceptions) > int(bft.AllowedExceptions) {
		return fmt.Errorf("%s: More than threshold (%d/%d) refused to sign - aborting",
			bft.Roster(), len(ch.Signature.Exceptions), len(bft.Roster().List))
	}

	// store the exceptions for later usage
	bft.tempExceptions = ch.Signature.Exceptions

	if bft.IsLeaf() {
		// bft.responseChan should be closed
		return bft.handleResponseCommit(bft.responseChan)
	}
	// only send to the committed children
	return bft.multicast(&ch, bft.respondedNodesCommit)
}

// handleResponsePrepare handles response messages in the prepare phase.  If
// the node is not the root, it'll aggregate the response and forward to the
// parent. Otherwise it verifies the response.
func (bft *ProtocolBFTCoSi) handleResponsePrepare(c chan responseChan) error {
	bft.tmpMutex.Lock()
	defer bft.tmpMutex.Unlock() // NOTE potentially locked for the whole timeout

	// wait until we have enough RoundPrepare responses or timeout
	// does nothing if channel is closed
	if err := bft.readResponseChan(c, RoundPrepare); err != nil {
		return err
	}

	// wait for verification
	bzrReturn, ok := bft.waitResponseVerification()
	if !ok {
		return fmt.Errorf("%v verification of the prepare-response failed", bft.Name())
	}

	// return if we're not root
	if !bft.IsRoot() {
		return bft.SendToParent(bzrReturn)
	}

	// Since cosi does not support exceptions yet, we have to remove the
	// responses that are not supposed to be there, i.e. exceptions.
	cosiSig := bft.prepare.Signature()
	correctResponseBuff, err := bzrReturn.Response.MarshalBinary()
	if err != nil {
		return err
	}

	// signature is aggregate commit || aggregate response || mask
	// replace the old aggregate response with the corrected one
	pointLen := bft.Suite().PointLen()
	sigLen := pointLen + bft.Suite().ScalarLen()
	copy(cosiSig[pointLen:sigLen], correctResponseBuff)
	bft.prepareSignature = cosiSig

	// Verify the signature is correct
	data := sha512.Sum512(bft.Msg)
	sig := &BFTSignature{
		Msg:        data[:],
		Sig:        cosiSig,
		Exceptions: bft.tempExceptions,
	}

	if err := sig.Verify(bft.Suite(), bft.Roster().Publics()); err != nil {
		return fmt.Errorf("%s: Verification of the prepare-response signature failed: %v", bft.Name(), err)
	}
	log.Lvl3(bft.Name(), "Verification of signature successful")

	// Start the challenge of the 'commit'-round
	if err := bft.startChallenge(RoundCommit); err != nil {
		log.Error(bft.Name(), err)
		return err
	}
	return nil
}

// handleResponseCommit is similar to `handleResponsePrepare` except it is for
// the commit phase. A key distinction is that the protocol ends at the end of
// this function and final signature is generated if it is called by the root.
func (bft *ProtocolBFTCoSi) handleResponseCommit(c chan responseChan) error {
	bft.tmpMutex.Lock()
	defer bft.tmpMutex.Unlock()

	// wait until we have enough RoundCommit responses or timeout
	// does nothing if channel is closed
	if err := bft.readResponseChan(c, RoundCommit); err != nil {
		return err
	}

	r := &Response{
		TYPE:     RoundCommit,
		Response: bft.Suite().Scalar().Zero(),
	}

	var err error
	if bft.IsLeaf() {
		r.Response, err = bft.commit.CreateResponse()
	} else {
		r.Response, err = bft.commit.Response(bft.tempCommitResponse)
	}
	if err != nil {
		return err
	}

	// if root we have finished
	if bft.IsRoot() {
		sig := bft.Signature()
		if bft.onSignatureDone != nil {
			bft.onSignatureDone(sig)
		}
		return nil
	}

	err = bft.SendToParent(r)
	return err
}

// readCommitChan reads a threshold (n-t) of the commit messages or until the
// timeout for message type `t`.
func (bft *ProtocolBFTCoSi) readCommitChan(c chan commitChan, t RoundType) error {
	timeout := time.After(bft.Timeout)
	for {
		if bft.isClosing() {
			return errors.New("closing")
		}

		select {
		case msg, ok := <-c:
			if !ok {
				log.Lvl3("Channel closed")
				return nil
			}

			from := msg.ServerIdentity.Public
			comm := msg.Commitment
			// store the message and return when we have enough
			switch comm.TYPE {
			case RoundPrepare:
				if bft.committedInPreparePhase {
					continue
				}
				if _, ok := bft.respondedNodesPrepare[from.String()]; ok {
					log.Warnf("%s: node %v already responded - potential malicious behaviour")
					continue
				}
				bft.respondedNodesPrepare[from.String()] = msg.TreeNode
				bft.tempPrepareCommit = append(bft.tempPrepareCommit, comm.Commitment)
				if t == RoundPrepare && len(bft.tempPrepareCommit) == len(bft.Children())-bft.AllowedExceptions {
					bft.committedInPreparePhase = true
					return nil
				}
			case RoundCommit:
				if bft.committedInCommitPhase {
					continue
				}
				if _, ok := bft.respondedNodesCommit[from.String()]; ok {
					log.Warnf("%s: node %v already responded - potential malicious behaviour")
					continue
				}
				bft.respondedNodesCommit[from.String()] = msg.TreeNode
				bft.tempCommitCommit = append(bft.tempCommitCommit, comm.Commitment)
				if t == RoundCommit && len(bft.tempCommitCommit) == len(bft.Children())-bft.AllowedExceptions {
					bft.committedInCommitPhase = true
					return nil
				}
			}
		case <-timeout:
			// if there is a timeout, then we cannot continue
			return fmt.Errorf("timeout while trying to read commit message for phase %v", t)
		}
	}
}

// readResponseChan reads a threshold (n-t) of response messages, the threshold
// must be from the committed set of nodes.  It returns an error on timeout.
func (bft *ProtocolBFTCoSi) readResponseChan(c chan responseChan, t RoundType) error {
	timeout := time.After(bft.Timeout)
	for {
		if bft.isClosing() {
			return errors.New("Closing")
		}

		select {
		case msg, ok := <-c:
			if !ok {
				log.Lvl3("Channel closed")
				return nil
			}
			r := msg.Response

			switch msg.Response.TYPE {
			case RoundPrepare:
				bft.tempPrepareResponse = append(bft.tempPrepareResponse, r.Response)
				bft.tempExceptions = append(bft.tempExceptions, r.Exceptions...)
				// NOTE here we assume all nodes that committed will respond
				if t == RoundPrepare && len(bft.tempPrepareResponse) == len(bft.tempPrepareCommit) {
					return nil
				}
			case RoundCommit:
				bft.tempCommitResponse = append(bft.tempCommitResponse, r.Response)
				// NOTE here we assume all nodes that committed will respond
				if t == RoundCommit && len(bft.tempCommitResponse) == len(bft.tempCommitCommit) {
					return nil
				}
			}
		case <-timeout:
			// if there is a timeout, then we cannot continue
			return fmt.Errorf("%s: timeout while trying to read response message for phase %v", bft.Name(), t)
		}
	}
}

// startAnnouncement creates its announcement for the prepare round and
// sends it down the tree.
func (bft *ProtocolBFTCoSi) startAnnouncement(t RoundType) error {
	bft.announceChan <- announceChan{Announce: Announce{TYPE: t, Timeout: bft.Timeout}}
	return nil
}

// startCommitment sends the first commitment to the parent node
func (bft *ProtocolBFTCoSi) startCommitment(t RoundType) error {
	cm := bft.getCosi(t).CreateCommitment(bft.Suite().RandomStream())
	return bft.SendToParent(&Commitment{TYPE: t, Commitment: cm})
}

// startChallenge creates the challenge and sends it to children that committed
func (bft *ProtocolBFTCoSi) startChallenge(t RoundType) error {
	switch t {
	case RoundPrepare:
		// need to hash the message before so challenge in both phases are not
		// the same
		data := sha512.Sum512(bft.Msg)
		ch, err := bft.prepare.CreateChallenge(data[:])
		if err != nil {
			return err
		}
		bftChal := ChallengePrepare{
			Challenge: ch,
			Msg:       bft.Msg,
			Data:      bft.Data,
		}
		bft.challengePrepareChan <- challengePrepareChan{ChallengePrepare: bftChal}
	case RoundCommit:
		// commit phase
		ch, err := bft.commit.CreateChallenge(bft.Msg)
		if err != nil {
			return err
		}

		// send challenge + signature
		cc := ChallengeCommit{
			Challenge: ch,
			Signature: &BFTSignature{
				Msg:        bft.Msg,
				Sig:        bft.prepareSignature,
				Exceptions: bft.tempExceptions,
			},
		}
		bft.challengeCommitChan <- challengeCommitChan{ChallengeCommit: cc}
	}
	return nil
}

// waitResponseVerification waits till the end of the verification and returns
// the BFTCoSiResponse along with the flag:
// true => no exception, the verification is correct
// false => exception, the verification failed
func (bft *ProtocolBFTCoSi) waitResponseVerification() (*Response, bool) {
	log.Lvl3(bft.Name(), "Waiting for response verification:")
	// wait the verification
	if !<-bft.verifyChan {
		return nil, false
	}

	// sanity check
	if bft.IsLeaf() && len(bft.tempPrepareResponse) != 0 {
		panic("bft.tempPrepareResponse is not 0 on leaf node")
	}

	resp, err := bft.prepare.Response(bft.tempPrepareResponse)
	if err != nil {
		return nil, false
	}

	// Create the exceptions for nodes that were not included in the first
	// 2/3 of the responses. We do this in two steps. (1) Find children
	// that are not in respondedNodesPrepare and (2) for the missing
	// ones, find the global index and then add it to the exception.
	for _, tn := range bft.Children() {
		if _, ok := bft.respondedNodesPrepare[tn.ServerIdentity.Public.String()]; !ok {
			// We assume the server was also not available for the commitment
			// so no need to subtract the commitment.
			// Conversely, we cannot handle nodes which fail right
			// after making a commitment at the moment.
			bft.tempExceptions = append(bft.tempExceptions, Exception{
				Index:      tn.RosterIndex,
				Commitment: bft.Suite().Point().Null(),
			})
		}
	}

	r := &Response{
		TYPE:       RoundPrepare,
		Exceptions: bft.tempExceptions,
		Response:   resp,
	}

	log.Lvl3(bft.Name(), "Response verified")
	return r, true
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bft *ProtocolBFTCoSi) nodeDone() bool {
	if bft.onDone != nil {
		// only true for the root
		bft.onDone()
	}
	return true
}

func (bft *ProtocolBFTCoSi) getCosi(t RoundType) *crypto.CoSi {
	if t == RoundPrepare {
		return bft.prepare
	}
	return bft.commit
}

func (bft *ProtocolBFTCoSi) isClosing() bool {
	bft.closingMutex.Lock()
	defer bft.closingMutex.Unlock()
	return bft.closing
}

func (bft *ProtocolBFTCoSi) setClosing() {
	bft.closingMutex.Lock()
	bft.closing = true
	bft.closingMutex.Unlock()
}

func (bft *ProtocolBFTCoSi) multicast(msg interface{}, nodes map[string]*onet.TreeNode) error {
	for _, tn := range nodes {
		if err := bft.SendTo(tn, msg); err != nil {
			log.Error(err)
		}
	}
	return nil
}
