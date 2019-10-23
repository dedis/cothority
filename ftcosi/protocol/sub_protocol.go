package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/cosi"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

func init() {
	GlobalRegisterDefaultProtocols()
}

// SubFtCosi holds the different channels used to receive the different protocol messages.
type SubFtCosi struct {
	*onet.TreeNodeInstance
	Publics        []kyber.Point
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	stoppedOnce    sync.Once
	verificationFn VerificationFn
	suite          cosi.Suite

	// protocol/subprotocol channels
	// these are used to communicate between the subprotocol and the main protocol
	subleaderNotResponding chan bool
	subCommitment          chan StructCommitment
	subResponse            chan StructResponse

	// internodes channels
	ChannelAnnouncement chan StructAnnouncement
	ChannelCommitment   chan StructCommitment
	ChannelChallenge    chan StructChallenge
	ChannelResponse     chan StructResponse

	stopOnce sync.Once
}

// NewDefaultSubProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewDefaultSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubFtCosi(n, vf, cothority.Suite)
}

// NewSubFtCosi is used to define the subprotocol and to register
// the channels where the messages will be received.
func NewSubFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite cosi.Suite) (onet.ProtocolInstance, error) {

	// tests if it's a three level tree
	moreThreeLevel := false
	n.Tree().Root.Visit(0, func(depth int, n *onet.TreeNode) {
		if depth > 2 {
			moreThreeLevel = true
		}
	})
	if moreThreeLevel {
		return nil, fmt.Errorf("subFtCosi launched with a more than three level tree")
	}

	c := &SubFtCosi{
		TreeNodeInstance: n,
		verificationFn:   vf,
		suite:            suite,
	}

	if n.IsRoot() {
		c.subleaderNotResponding = make(chan bool, 1)
		c.subCommitment = make(chan StructCommitment, 2) // can send 2 commitments
		c.subResponse = make(chan StructResponse, 1)
	}

	err := c.RegisterChannels(&c.ChannelAnnouncement,
		&c.ChannelCommitment,
		&c.ChannelChallenge,
		&c.ChannelResponse)
	if err != nil {
		return nil, errors.New("couldn't register channels: " + err.Error())
	}
	err = c.RegisterHandler(c.HandleStop)
	if err != nil {
		return nil, errors.New("couldn't register stop handler: " + err.Error())
	}
	return c, nil
}

// Dispatch is the main method of the subprotocol, running on each node and handling the messages in order
func (p *SubFtCosi) Dispatch() error {
	defer func() {
		if p.IsRoot() {
			err := p.Broadcast(&Stop{})
			if err != nil {
				log.Error("error while broadcasting stopping message:", err)
			}
		}
		p.Done()
	}()
	var err error
	var channelOpen bool

	// ----- Announcement -----
	var announcement StructAnnouncement
	for {
		announcement, channelOpen = <-p.ChannelAnnouncement
		if !channelOpen {
			return nil
		}
		if !isValidSender(announcement.TreeNode, p.Parent(), p.TreeNode()) {
			log.Warn(p.ServerIdentity(), "received announcement from node", announcement.ServerIdentity,
				"that is not its parent nor itself, ignored")
		} else {
			log.Lvl3(p.ServerIdentity(), "received announcement")
			break
		}

	}

	// get announcement parameters
	p.Publics = announcement.Publics
	p.Timeout = announcement.Timeout
	if !p.IsRoot() {
		// We'll be only waiting on the root and the subleaders. The subleaders
		// only have half of the time budget of the root.
		p.Timeout /= 2
	}
	p.Msg = announcement.Msg
	p.Data = announcement.Data
	p.Threshold = announcement.Threshold

	// verify that threshold is valid
	maxThreshold := p.Tree().Size() - 1
	if p.Threshold > maxThreshold {
		return fmt.Errorf("threshold %d bigger than the maximum of commitments this subtree can gather (%d)", p.Threshold, maxThreshold)
	}

	// start the verification in background if I'm not the root because
	// root does the verification in the main protocol
	var secret kyber.Scalar
	if !p.IsRoot() {
		go func() {
			log.Lvl3(p.ServerIdentity(), "starting verification in the background")
			verificationOk := p.verificationFn(p.Msg, p.Data)

			var personalStructCommitment StructCommitment
			var err error
			secret, personalStructCommitment, err = p.getCommitment(verificationOk)
			if err != nil {
				log.Error("error while generating own commitment:", err)
				return
			}
			p.ChannelCommitment <- personalStructCommitment
			log.Lvl3(p.ServerIdentity(), "verification done:", verificationOk)
		}()
	}

	if !p.IsLeaf() {
		// Only send commits if the node has children
		go func() {
			if errs := p.SendToChildrenInParallel(&announcement.Announcement); len(errs) > 0 {
				log.Error(p.ServerIdentity(), "failed to send announcement to all children, trying to continue")
			}
		}()
	}

	// ----- Commitment & Challenge -----

	var commitments = make([]StructCommitment, 0)                  // list of received commitments
	var challenge StructChallenge                                  // the challenge that will be received
	var nodesCanCommit = make([]*onet.TreeNode, len(p.Children())) // the list of nodes that can commit. Nodes will be removed from the list once they commit.
	var challengeMask *cosi.Mask                                   // the mask received in the challenge, set only if not root.
	var childrenCanResponse = make([]*onet.TreeNode, 0)            // the list of children that can send a response. That is the list of children present in the challenge mask.

	var refusalCount = 0            // number of refusal received. Will be used only for the subleader
	var firstCommitmentSent = false // to avoid sending the quick commitment multiple times
	var verificationDone = false    // to send the aggregate commitment only once this node has done its verification
	var timedOut = false            // to refuse new commitments once it times out
	commitTimeout := p.Timeout / 2
	responseTimeout := p.Timeout / 2
	var t = time.After(commitTimeout) // the timeout for the commitment phase

	copy(nodesCanCommit, p.Children())
	if p.IsRoot() {
		nodesCanCommit = append(nodesCanCommit, p.Children()...) // every node can send quick and final answer
	} else {
		nodesCanCommit = append(nodesCanCommit, p.TreeNode()) // can have its own commitment
	}

loop:
	for {
		select {
		case commitment, channelOpen := <-p.ChannelCommitment:
			if !channelOpen {
				return nil
			}
			if timedOut {
				// ignore new commits once time-out has been reached
				break
			}

			if !isValidSender(commitment.TreeNode, nodesCanCommit...) {
				log.Warn(p.ServerIdentity(), "received a Commitment from node", commitment.ServerIdentity,
					"that is not in the list of nodes that can still commit, ignored")
				break // discards it
			}
			nodesCanCommit = remove(nodesCanCommit, commitment.TreeNode)

			// if is own commitment
			if commitment.TreeNode.ID.Equal(p.TreeNode().ID) {
				verificationDone = true
			}

			if p.IsRoot() {
				// send commitment to super-protocol
				p.subCommitment <- commitment

				// deactivate timeout
				t = make(chan time.Time)
			} else {

				// verify mask of received commitment
				verificationMask, err := cosi.NewMask(p.suite, p.Publics, nil)
				if err != nil {
					return err
				}
				err = verificationMask.SetMask(commitment.Mask)
				if err != nil {
					return err
				}
				if verificationMask.CountEnabled() > 1 {
					log.Warn(p.ServerIdentity(), "received commitment with ill-formed mask in non-root node: has",
						verificationMask.CountEnabled(), "nodes enabled instead of 0 or 1, ignored")
					break
				}

				// checks if commitment is a refusal or acceptance
				if commitment.CoSiCommitment.Equal(p.suite.Point().Null()) { // refusal
					refusalCount++
					if p.IsLeaf() {
						log.Warn(p.ServerIdentity(), "leaf refused Commitment, marking as not signed")
						return p.sendAggregatedCommitments([]StructCommitment{}, 1)
					}
					log.Warn(p.ServerIdentity(), "non-leaf got refusal")
				} else {
					// accepted
					commitments = append(commitments, commitment)
				}

				thresholdRefusal := (1 + len(p.Children()) - p.Threshold) + 1

				// checks if threshold is reached or unreachable
				quickAnswer := !firstCommitmentSent &&
					(len(commitments) >= p.Threshold || // quick valid answer
						refusalCount >= thresholdRefusal) // quick refusal answer

				// checks if every child and himself committed
				finalAnswer := len(commitments)+refusalCount == len(p.Children())+1

				if (quickAnswer || finalAnswer) && verificationDone {

					err = p.sendAggregatedCommitments(commitments, refusalCount)
					if err != nil {
						return err
					}

					// deactivate timeout if final commitment
					if finalAnswer {
						t = make(chan time.Time)
					}

					firstCommitmentSent = true
				}

				// security check
				if len(commitments)+refusalCount > maxThreshold {
					log.Error(p.ServerIdentity(), "more commitments (", len(commitments),
						") and refusals (", refusalCount, ") than possible in subleader (", maxThreshold, ")")
				}
			}
		case challenge, channelOpen = <-p.ChannelChallenge:
			if !channelOpen {
				return nil
			}
			if !isValidSender(challenge.TreeNode, p.Parent(), p.TreeNode()) {
				log.Warn(p.ServerIdentity(), "received a Challenge from node", challenge.ServerIdentity,
					"that is not its parent nor itself, ignored")
				break // discards it
			}
			log.Lvl3(p.ServerIdentity(), "received challenge")

			if challenge.AggregateCommit == nil {
				log.Warn(p.ServerIdentity(), "got an old, insecure challenge from", p.ServerIdentity())
			}

			if p.IsRoot() {
				childrenCanResponse = p.Children()
			} else {

				// get children present in challenge mask
				challengeMask, err = cosi.NewMask(p.suite, p.Publics, nil)
				if err != nil {
					return fmt.Errorf("error in creating new mask: %s", err)
				}
				err = challengeMask.SetMask(challenge.Mask)
				if err != nil {
					return fmt.Errorf("error in setting challenge mask: %s", err)
				}
				for _, child := range p.Children() {
					isEnabled, err := challengeMask.KeyEnabled(child.ServerIdentity.Public)
					if err != nil {
						return fmt.Errorf("error in checking a child presence in challenge mask: %s", err)
					}
					if isEnabled {
						childrenCanResponse = append(childrenCanResponse, child)
					}
				}
			}
			// send challenge to children
			childrenToSendChallenge := make([]*onet.TreeNode, len(childrenCanResponse))
			copy(childrenToSendChallenge, childrenCanResponse) // copy to avoid data race
			go func() {
				if errs := p.multicastParallel(&challenge.Challenge, childrenToSendChallenge...); len(errs) > 0 {
					log.Error(p.ServerIdentity(), errs)
				}
			}()

			break loop
		case <-t:
			if p.IsRoot() {
				log.Warn(p.ServerIdentity(), "timed out while waiting for subleader commitment")
				p.subleaderNotResponding <- true
				return nil
			}
			log.Warnf("%s timed out after %s while waiting for commits, got %d commitments and %d refusals",
				p.ServerIdentity(), commitTimeout, len(commitments), refusalCount)

			// sending commits received
			err = p.sendAggregatedCommitments(commitments, refusalCount)
			if err != nil {
				return err
			}
			timedOut = true
		}
	}

	// ----- Response -----
	responses := make([]StructResponse, 0)

	// Second half of our time budget for the responses.
	timeout := time.After(responseTimeout)
	for len(childrenCanResponse) > 0 {
		select {
		case response, channelOpen := <-p.ChannelResponse:
			if !channelOpen {
				return nil
			}

			if !isValidSender(response.TreeNode, childrenCanResponse...) {
				log.Warn(p.ServerIdentity(), "received a Response from node", response.ServerIdentity,
					"that is not in the list of nodes that cna still send a response, ignored")
				break
			}
			childrenCanResponse = remove(childrenCanResponse, response.TreeNode)

			responses = append(responses, response)
		case <-timeout:
			return fmt.Errorf("timeout while waiting for responses")
		}
	}
	log.Lvl3(p.ServerIdentity(), "received all", len(responses), "response(s)")

	// if root, send response to super-protocol and finish
	if p.IsRoot() {
		if len(responses) != 1 {
			return fmt.Errorf(
				"root node in subprotocol should have received 1 response, but received %d",
				len(responses))
		}
		p.subResponse <- responses[0]
		return nil
	}

	// check challenge
	if challenge.AggregateCommit == nil || challenge.Mask == nil {
		log.Warn("Only have pre-calculated challenge - this is dangerous!")
	} else {
		mask, err := cosi.NewMask(p.suite, p.Publics, nil)
		if err != nil {
			return fmt.Errorf("error, while creating empty mask: %s", err)
		}
		err = mask.SetMask(challenge.Mask)
		if err != nil {
			return fmt.Errorf("error while setting challenge mask: %s", err)
		}
		cosiChallenge, err := cosi.Challenge(p.suite, challenge.AggregateCommit,
			mask.AggregatePublic, p.Msg)
		if err != nil {
			return err
		}
		if !cosiChallenge.Equal(challenge.CoSiChallenge) {
			log.Warn("Pre-calculated challenge is not the same as ours!")
			challenge.CoSiChallenge = cosiChallenge
		}
	}

	// add own response if in mask
	isInMask, err := challengeMask.KeyEnabled(p.Public())
	if err != nil {
		return fmt.Errorf("error in checking a key presence in the challenge mask: %s", err)
	}
	if isInMask {
		personalResponse, err := cosi.Response(p.suite, p.Private(), secret, challenge.CoSiChallenge)
		if err != nil {
			return fmt.Errorf("error while generating own response: %s", err)
		}
		responses = append(responses, StructResponse{p.TreeNode(), Response{personalResponse}})
	}

	// aggregate all responses
	aggResponse, err := aggregateResponses(p.suite, responses)
	if err != nil {
		return err
	}

	// send to parents
	err = p.SendToParent(&Response{aggResponse})
	if err != nil {
		return err
	}
	return nil
}

func (p *SubFtCosi) sendAggregatedCommitments(commitments []StructCommitment, NRefusal int) error {

	// aggregate commitments
	commitment, mask, err := aggregateCommitments(p.suite, p.Publics, commitments)
	if err != nil {
		return err
	}

	// send to parent
	err = p.SendToParent(&Commitment{commitment, mask.Mask(), NRefusal})
	if err != nil {
		return err
	}

	log.Lvl3(p.ServerIdentity(), "commitment sent with", mask.CountEnabled(), "accepted and", NRefusal, "refusals")

	return nil
}

// HandleStop is called when a Stop message is send to this node. It stops the node.
func (p *SubFtCosi) HandleStop(stop StructStop) error {
	if !isValidSender(stop.TreeNode, p.Root()) {
		log.Warn(p.ServerIdentity(), "received a Stop from node", stop.ServerIdentity,
			"that is not the root, ignored")
	}

	p.stopOnce.Do(func() {
		close(p.ChannelAnnouncement)
		// close(p.ChannelCommitment) // Channel left open to allow verification function to safely return
		close(p.ChannelChallenge)
		close(p.ChannelResponse)
	})
	return nil
}

// Start is done only by root and starts the subprotocol
func (p *SubFtCosi) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting subCoSi")
	if p.Msg == nil {
		return errors.New("subprotocol does not have a proposal msg")
	}
	if p.Data == nil {
		return errors.New("subprotocol does not have data, it can be empty but cannot be nil")
	}
	if p.Publics == nil || len(p.Publics) < 1 {
		return errors.New("subprotocol has invalid public keys")
	}
	if p.verificationFn == nil {
		return errors.New("subprotocol has an empty verification fn")
	}
	if p.Timeout < 10*time.Nanosecond {
		return errors.New("unrealistic timeout")
	}
	if p.Threshold > p.Tree().Size() {
		return errors.New("threshold bigger than number of nodes in subtree")
	}
	if p.Threshold < 1 {
		return fmt.Errorf("threshold of %d smaller than one node", p.Threshold)
	}

	announcement := StructAnnouncement{
		p.TreeNode(),
		Announcement{p.Msg, p.Data, p.Publics, p.Timeout, p.Threshold},
	}
	p.ChannelAnnouncement <- announcement
	return nil
}

// generates a commitment.
// the boolean indicates whether the commitment is a proposal acceptance or a proposal refusal.
// Returns the generated secret, the commitment and an error if there was a problem in the process.
func (p *SubFtCosi) getCommitment(accepts bool) (kyber.Scalar, StructCommitment, error) {

	emptyMask, err := cosi.NewMask(p.suite, p.Publics, nil)
	if err != nil {
		return nil, StructCommitment{}, err
	}

	structCommitment := StructCommitment{p.TreeNode(),
		Commitment{p.suite.Point().Null(), emptyMask.Mask(), 0}}

	var secret kyber.Scalar // nil
	if accepts {
		secret, structCommitment.CoSiCommitment = cosi.Commit(p.suite)
		var personalMask *cosi.Mask
		personalMask, err = cosi.NewMask(p.suite, p.Publics, p.Public())
		if err != nil {
			return secret, StructCommitment{}, err
		}
		structCommitment.Mask = personalMask.Mask()
	} else { // refuses
		structCommitment.NRefusal++
	}

	return secret, structCommitment, nil
}

// multicastParallel can be moved to onet.TreeNodeInstance once it shows
// promise.
func (p *SubFtCosi) multicastParallel(msg interface{}, nodes ...*onet.TreeNode) []error {
	var errs []error
	eMut := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, node := range nodes {
		name := node.Name()
		wg.Add(1)
		go func(n2 *onet.TreeNode) {
			if err := p.SendTo(n2, msg); err != nil {
				eMut.Lock()
				errs = append(errs, errors.New(name+": "+err.Error()))
				eMut.Unlock()
			}
			wg.Done()
		}(node)
	}
	wg.Wait()
	return errs
}

// checks if a node is in a list of nodes
func isValidSender(node *onet.TreeNode, valids ...*onet.TreeNode) bool {
	// check if comes from a committed children
	isValid := false
	for _, valid := range valids {
		if valid != nil {
			if valid.Equal(node) {
				isValid = true
			}
		}
	}
	return isValid
}

// removes the first instance of a node from a slice
func remove(nodesList []*onet.TreeNode, node *onet.TreeNode) []*onet.TreeNode {
	for i, iNode := range nodesList {
		if iNode.Equal(node) {
			return append(nodesList[:i], nodesList[i+1:]...)
		}
	}
	return nodesList
}
