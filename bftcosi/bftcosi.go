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

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

const defaultTimeout = 1 * time.Second

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
	// Timeout is how long to wait while gathering commits.
	Timeout time.Duration
	// last block computed
	lastBlock string
	// refusal to sign for the commit phase or not. This flag is set during the
	// Challenge of the commit phase and will be used during the response of the
	// commit phase to put an exception or to sign.
	signRefusal bool
	// allowedExceptions for how much exception is allowed. If more than allowedExceptions number
	// of conodes refuse to sign, no signature will be created.
	allowedExceptions int
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
}

// collectStructs holds the variables that are used during the protocol to hold
// messages
type collectStructs struct {
	// prepare-round cosi
	prepare *cosi.CoSi
	// commit-round cosi
	commit *cosi.CoSi

	// prepareSignature is the signature generated during the prepare phase
	// This signature is adapted according to the exceptions that occured during
	// the prepare phase.
	prepareSignature []byte

	// mutex for all temporary structures
	tmpMutex sync.Mutex
	// exceptions given during the rounds that is used in the signature
	tempExceptions []Exception
	// temporary buffer of "prepare" commitments
	tempPrepareCommit []kyber.Point
	// temporary buffer of "commit" commitments
	tempCommitCommit []kyber.Point
	// temporary buffer of "prepare" responses
	tempPrepareResponse []kyber.Scalar
	// temporary buffer of the public keys for nodes that responded
	tempPrepareResponsePublics []kyber.Point
	// temporary buffer of "commit" responses
	tempCommitResponse []kyber.Scalar
}

// NewBFTCoSiProtocol returns a new bftcosi struct
func NewBFTCoSiProtocol(n *onet.TreeNodeInstance, verify VerificationFunction) (*ProtocolBFTCoSi, error) {
	// initialize the bftcosi node/protocol-instance
	nodes := len(n.Tree().List())
	bft := &ProtocolBFTCoSi{
		TreeNodeInstance: n,
		collectStructs: collectStructs{
			prepare: cosi.NewCosi(n.Suite(), n.Private(), n.Roster().Publics()),
			commit:  cosi.NewCosi(n.Suite(), n.Private(), n.Roster().Publics()),
		},
		verifyChan:           make(chan bool),
		VerificationFunction: verify,
		allowedExceptions:    nodes - (nodes+1)*2/3,
		Msg:                  make([]byte, 0),
		Data:                 make([]byte, 0),
		Timeout:              defaultTimeout,
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

	// Finish the prepare round
	if err := bft.handleChallengePrepare(<-bft.challengePrepareChan); err != nil {
		return err
	}
	if !bft.IsLeaf() {
		if err := bft.handleResponsePrepare(bft.responseChan); err != nil {
			return err
		}
	}

	// Finish the commit round
	if err := bft.handleChallengeCommit(<-bft.challengeCommitChan); err != nil {
		return err
	}
	if !bft.IsLeaf() {
		if err := bft.handleResponseCommit(bft.responseChan); err != nil {
			return err
		}
	}
	return nil
}

// Signature will generate the final signature, the output of the BFTCoSi
// protocol.
// The signature contains the commit round signature, with the message.
// If the prepare phase failed, the signature will be nil and the Exceptions
// will contain the exception from the prepare phase. It can be useful to see
// which cosigners refused to sign (each exceptions contains the index of a
// refusing-to-sign signer).
// Expect this function to have an undefined behavior when called from a
// non-root Node.
func (bft *ProtocolBFTCoSi) Signature() *BFTSignature {
	bftSig := &BFTSignature{
		Sig:        bft.commit.Signature(),
		Msg:        bft.Msg,
		Exceptions: nil,
	}
	if bft.signRefusal {
		bftSig.Sig = nil
		bftSig.Exceptions = bft.tempExceptions
	}

	// This is a hack to include exceptions which are the result of offline
	// nodes rather than nodes that refused to sign.
	if bftSig.Exceptions == nil {
		for _, ex := range bft.tempExceptions {
			if ex.Commitment.Equal(bft.Suite().Point().Null()) {
				bftSig.Exceptions = append(bftSig.Exceptions, ex)
			}
		}
	}

	return bftSig
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
	return bft.sendToChildren(&ann)
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

	// TODO this will not always work for non-star graphs
	if len(bft.tempPrepareCommit) < len(bft.Children())-bft.allowedExceptions {
		bft.signRefusal = true
		log.Error("not enough prepare commitment messages")
	}

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
	bft.readCommitChan(c, RoundCommit)

	// TODO this will not always work for non-star graphs
	if len(bft.tempCommitCommit) < len(bft.Children())-bft.allowedExceptions {
		bft.signRefusal = true
		log.Error("not enough commit commitment messages")
	}

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
		bft.verifyChan <- bft.VerificationFunction(bft.Msg, bft.Data)
	}()
	if bft.IsLeaf() {
		return bft.startResponse(RoundPrepare)
	}
	return bft.sendToChildren(&ch)
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
		log.Error(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
	}

	// check if we have no more than threshold failed nodes
	if len(ch.Signature.Exceptions) > int(bft.allowedExceptions) {
		log.Errorf("%s: More than threshold (%d/%d) refused to sign - aborting.",
			bft.Roster(), len(ch.Signature.Exceptions), len(bft.Roster().List))
		bft.signRefusal = true
	}

	// store the exceptions for later usage
	bft.tempExceptions = ch.Signature.Exceptions

	if bft.IsLeaf() {
		// bft.responseChan should be closed
		return bft.handleResponseCommit(bft.responseChan)
	}
	return bft.sendToChildren(&ch)
}

// handleResponsePrepare handles response messages in the prepare phase.
// If the node is not the root, it'll aggregate the response and forward to
// the parent. Otherwise it verifies the response.
func (bft *ProtocolBFTCoSi) handleResponsePrepare(c chan responseChan) error {
	bft.tmpMutex.Lock()
	defer bft.tmpMutex.Unlock() // NOTE potentially locked for the whole timeout

	// wait until we have enough RoundPrepare responses or timeout
	// does nothing if channel is closed
	if err := bft.readResponseChan(c, RoundPrepare); err != nil {
		return err
	}

	// TODO this will only work for star-graphs
	// check if we have enough messages
	if len(bft.tempPrepareResponse) < len(bft.Children())-bft.allowedExceptions {
		log.Error("not enough prepare response messages")
		bft.signRefusal = true
	}

	// wait for verification
	bzrReturn, ok := bft.waitResponseVerification()
	// append response
	if !ok {
		log.Lvl2(bft.Roster(), "Refused to sign")
	}

	// Return if we're not root
	if !bft.IsRoot() {
		return bft.SendTo(bft.Parent(), bzrReturn)
	}

	// Since cosi does not support exceptions yet, we have to remove
	// the responses that are not supposed to be there,i.e. exceptions.
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
		log.Error(bft.Name(), "Verification of the signature failed:", err)
		bft.signRefusal = true
		return err
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
	bft.readResponseChan(c, RoundCommit)

	// TODO this will only work for star-graphs
	// check if we have enough messages
	if len(bft.tempCommitResponse) < len(bft.Children())-bft.allowedExceptions {
		log.Error("not enough commit response messages")
		bft.signRefusal = true
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

	if bft.signRefusal {
		r.Exceptions = append(r.Exceptions, Exception{
			Index:      bft.index,
			Commitment: bft.commit.GetCommitment(),
		})
		// don't include our own!
		r.Response.Sub(r.Response, bft.commit.GetResponse())
	}

	// notify we have finished to participate in this signature
	log.Lvl3(bft.Name(), "refusal=", bft.signRefusal)
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
	err = bft.SendTo(bft.Parent(), r)
	bft.Done()
	return err
}

// readCommitChan reads until all commit messages are received or a timeout for message type `t`
func (bft *ProtocolBFTCoSi) readCommitChan(c chan commitChan, t RoundType) error {
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

			comm := msg.Commitment
			// store the message and return when we have enough
			switch comm.TYPE {
			case RoundPrepare:
				bft.tempPrepareCommit = append(bft.tempPrepareCommit, comm.Commitment)
				if t == RoundPrepare && len(bft.tempPrepareCommit) == len(bft.Children()) {
					return nil
				}
			case RoundCommit:
				bft.tempCommitCommit = append(bft.tempCommitCommit, comm.Commitment)
				if t == RoundCommit && len(bft.tempCommitCommit) == len(bft.Children()) {
					return nil
				}
			}
		case <-time.After(bft.Timeout):
			// in some cases this might be ok because we accept a certain number of faults
			// the caller is responsible for checking if enough messages are received
			log.Lvl1("timeout while trying to read commit messages")
			return nil
		}
	}
}

// should do nothing if the channel is closed
func (bft *ProtocolBFTCoSi) readResponseChan(c chan responseChan, t RoundType) error {
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
			from := msg.ServerIdentity.Public
			r := msg.Response

			switch msg.Response.TYPE {
			case RoundPrepare:
				bft.tempPrepareResponse = append(bft.tempPrepareResponse, r.Response)
				bft.tempExceptions = append(bft.tempExceptions, r.Exceptions...)
				bft.tempPrepareResponsePublics = append(bft.tempPrepareResponsePublics, from)
				if t == RoundPrepare && len(bft.tempPrepareResponse) == len(bft.Children()) {
					return nil
				}
			case RoundCommit:
				bft.tempCommitResponse = append(bft.tempCommitResponse, r.Response)
				if t == RoundCommit && len(bft.tempCommitResponse) == len(bft.Children()) {
					return nil
				}
			}
		case <-time.After(bft.Timeout):
			log.Lvl1("timeout while trying to read response messages")
			return nil
		}
	}
}

// startAnnouncementPrepare create its announcement for the prepare round and
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

// startChallenge creates the challenge and sends it to its children
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

// startResponse dispatches the response to the correct round-type
func (bft *ProtocolBFTCoSi) startResponse(t RoundType) error {
	if !bft.IsLeaf() {
		panic("Only leaf can call startResponse")
	}

	switch t {
	case RoundPrepare:
		return bft.handleResponsePrepare(bft.responseChan)
	case RoundCommit:
		return bft.handleResponseCommit(bft.responseChan)
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
	verified := <-bft.verifyChan

	// sanity check
	if bft.IsLeaf() && len(bft.tempPrepareResponse) != 0 {
		panic("bft.tempPrepareResponse is not 0 on leaf node")
	}

	resp, err := bft.prepare.Response(bft.tempPrepareResponse)
	if err != nil {
		return nil, false
	}

	if !verified {
		// Add our exception
		bft.tempExceptions = append(bft.tempExceptions, Exception{
			Index:      bft.index,
			Commitment: bft.prepare.GetCommitment(),
		})
		// Don't include our response!
		resp = bft.Suite().Scalar().Set(resp).Sub(resp, bft.prepare.GetResponse())
		log.Lvl3(bft.Name(), "Response verification: failed")
	}

	// if we didn't get all the responses, add them to the exception
	// 1, find children that are not in tempPrepareResponsePublics
	// 2, for the missing ones, find the global index and then add it to the exception
	publicsMap := make(map[kyber.Point]bool)
	for _, p := range bft.tempPrepareResponsePublics {
		publicsMap[p] = true
	}
	for _, tn := range bft.Children() {
		if !publicsMap[tn.ServerIdentity.Public] {
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

	log.Lvl3(bft.Name(), "Response verification:", verified)
	return r, verified
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (bft *ProtocolBFTCoSi) nodeDone() bool {
	bft.Shutdown()
	if bft.onDone != nil {
		// only true for the root
		bft.onDone()
	}
	return true
}

func (bft *ProtocolBFTCoSi) getCosi(t RoundType) *cosi.CoSi {
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

func (bft *ProtocolBFTCoSi) sendToChildren(msg interface{}) error {
	// TODO send to only nodes that did reply
	errs := bft.SendToChildrenInParallel(msg)
	if len(errs) > bft.allowedExceptions {
		return fmt.Errorf("sendToChildren failed with errors: %v", errs)
	}
	return nil
}
