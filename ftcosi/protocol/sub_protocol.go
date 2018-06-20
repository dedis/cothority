package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
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
		c.subCommitment = make(chan StructCommitment, 2) //can send 2 commitments
		c.subResponse = make(chan StructResponse, 1)
	}

	for _, channel := range []interface{}{
		&c.ChannelAnnouncement,
		&c.ChannelCommitment,
		&c.ChannelChallenge,
		&c.ChannelResponse,
	} {
		err := c.RegisterChannel(channel)
		if err != nil {
			return nil, errors.New("couldn't register channel: " + err.Error())
		}
	}
	err := c.RegisterHandler(c.HandleStop)
	if err != nil {
		return nil, errors.New("couldn't register stop handler: " + err.Error())
	}
	return c, nil
}

// Shutdown stops the protocol
// If the node is the root node, it broadcasts a Stop message to all the nodes in the tree.
func (p *SubFtCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		if p.IsRoot() {
			err := p.Broadcast(&Stop{})
			if err != nil {
				log.Error("error while broadcasting stopping message:", err)
			}
		}
		close(p.ChannelAnnouncement)
		//close(p.ChannelCommitment) // Channel left open to allow verification function to safely return
		close(p.ChannelChallenge)
		close(p.ChannelResponse)
	})
	return nil
}

// Dispatch is the main method of the subprotocol, running on each node and handling the messages in order
func (p *SubFtCosi) Dispatch() error {
	defer p.Done()
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
			log.Lvl2(p.ServerIdentity(), "received announcement from node", announcement.ServerIdentity,
				"that is not its parent nor itself, ignored")
		} else {
			log.Lvl3(p.ServerIdentity(), "received announcement")
			break
		}

	}

	//get announcement parameters
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

	//verify that threshold is valid
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
			secret, personalStructCommitment, err = p.getCommitment(verificationOk)
			if err != nil {
				log.Errorf("error while generating own commitment:", err)
				p.Shutdown()
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
				log.Lvl3(p.ServerIdentity(), "failed to send announcement to all children")
			}
		}()
	}

	// ----- Commitment & Challenge -----

	var commitments = make([]StructCommitment, 0)                      // list of received commitments
	var challenge StructChallenge                                      // the challenge that will be received
	var childrenCommitting = make([]*onet.TreeNode, len(p.Children())) // the list of children that can commit. Children that commits will be removed from it.
	copy(childrenCommitting, p.Children())

	var NRefusal = 0                  // number of refusal received. Will be used only for the subleader
	var firstCommitmentSent = false   // to avoid sending the quick commitment multiple times
	var hasCommitted = false          // to send the aggregate commitment only once this node has committed
	var timedOut = false              // to refuse new commitments once it times out
	var t = time.After(p.Timeout / 2) // the timeout
loop:
	for {
		select {
		case commitment, channelOpen := <-p.ChannelCommitment:
			if !channelOpen {
				return nil
			}
			if timedOut { //ignore new commits once time-out has been reached
				break
			}

			isOwnCommitment := commitment.TreeNode.ID.Equal(p.TreeNode().ID)

			if !isOwnCommitment {
				if !isValidSender(commitment.TreeNode, childrenCommitting...) {
					log.Lvl2(p.ServerIdentity(), "received a Commitment from node", commitment.ServerIdentity,
						"that is neither a children nor itself, ignored")
					break //discards it
				}
				remove(childrenCommitting, commitment.TreeNode)
			}

			if p.IsRoot() {
				// send commitment to super-protocol
				p.subCommitment <- commitment

				//deactivate timeout
				t = make(chan time.Time)

				childrenCommitting = []*onet.TreeNode{commitment.TreeNode}
			} else {

				//verify mask of received commitment
				verificationMask, err := cosi.NewMask(p.suite, p.Publics, nil)
				if err != nil {
					return err
				}
				err = verificationMask.SetMask(commitment.Mask)
				if err != nil {
					return err
				}
				if verificationMask.CountEnabled() > 1 {
					log.Lvl2(p.ServerIdentity(), "received commitment with ill-formed mask in non-root node: has",
						verificationMask.CountEnabled(), "nodes enabled instead of 0 or 1")
					break
				}

				if commitment.CoSiCommitment.Equal(p.suite.Point().Null()) { //refusal
					NRefusal++
					if isOwnCommitment {
						log.Warn(p.ServerIdentity(), "refused Commitment, stopping protocol")
						defer p.Shutdown()

						err = p.sendAggregatedCommitments(nil, NRefusal)
						if err != nil {
							return err
						}
						return nil
					}
				} else { //accepted
					if isOwnCommitment {
						hasCommitted = true
					}
					commitments = append(commitments, commitment)
				}

				thresholdRefusal := (1 + len(p.Children()) - p.Threshold) + 1
				quickAnswer := !firstCommitmentSent &&
					(len(commitments) >= p.Threshold || // quick valid answer
						NRefusal >= thresholdRefusal) // quick refusal answer
				finalAnswer := len(commitments)+NRefusal == len(p.Children())+1

				if (quickAnswer || finalAnswer || p.IsLeaf()) && hasCommitted {

					err = p.sendAggregatedCommitments(commitments, NRefusal)
					if err != nil {
						return err
					}

					//deactivate timeout if final commitment
					if firstCommitmentSent || p.IsLeaf() {
						t = make(chan time.Time)
					}

					firstCommitmentSent = true
				}

				//security check
				if len(commitments)+NRefusal > maxThreshold {
					log.Error(p.ServerIdentity(), "more commitments (", len(commitments),
						") and refusals (", NRefusal, ") than possible in subleader (", maxThreshold, ")")
				}
			}
		case challenge, channelOpen = <-p.ChannelChallenge:
			if !channelOpen {
				return nil
			}
			if !isValidSender(challenge.TreeNode, p.Parent(), p.TreeNode()) {
				log.Lvl2(p.ServerIdentity(), "received a Challenge from node", challenge.ServerIdentity,
					"that is not its parent nor itself, ignored")
				break //discards it
			}
			log.Lvl3(p.ServerIdentity(), "received challenge")

			childrenCommitting, err = p.getChildrenInMask(challenge.Mask)
			if err != nil {
				return fmt.Errorf("error in handling challenge mask: %s", err)
			}

			//send challenge to children
			go func() {
				if errs := p.multicastParallel(&challenge.Challenge, childrenCommitting...); len(errs) > 0 {
					log.Lvl3(p.ServerIdentity(), errs)
				}
			}()

			break loop
		case <-t:
			if p.IsRoot() {
				log.Warn(p.ServerIdentity(), "timed out while waiting for subleader commitment")
				p.subleaderNotResponding <- true
				return nil
			}
			log.Warn(p.ServerIdentity(), "timed out while waiting for commits, got", len(commitments), "commitments and", NRefusal, "refusals")

			//sending commits received
			err = p.sendAggregatedCommitments(commitments, NRefusal)
			if err != nil {
				return err
			}
			timedOut = true
		}
	}

	// ----- Response -----
	responses := make([]StructResponse, 0)

	// Second half of our time budget for the responses.
	timeout := time.After(p.Timeout / 2)
	for len(childrenCommitting) > 0 {
		select {
		case response, channelOpen := <-p.ChannelResponse:
			if !channelOpen {
				return nil
			}

			if !isValidSender(response.TreeNode, childrenCommitting...) {
				log.Lvl2(p.ServerIdentity(), "received a Response from node", response.ServerIdentity,
					"that is not a committed children, ignored")
				break
			}
			childrenCommitting = remove(childrenCommitting, response.TreeNode)

			responses = append(responses, response)
		case <-timeout:
			return fmt.Errorf("timeout while waiting for responses")
		}
	}
	log.Lvl3(p.ServerIdentity(), "received all", len(responses), "response(s)")

	if p.IsRoot() {
		// send response to super-protocol
		if len(responses) != 1 {
			return fmt.Errorf(
				"root node in subprotocol should have received 1 response, but received %d",
				len(responses))
		}
		p.subResponse <- responses[0]
	} else {
		if secret != nil {
			// add own response
			personalResponse, err := cosi.Response(p.suite, p.Private(), secret, challenge.CoSiChallenge)
			if err != nil {
				return fmt.Errorf("error while generating own response: %s", err)
			}
			responses = append(responses, StructResponse{p.TreeNode(), Response{personalResponse}})
		}

		aggResponse, err := aggregateResponses(p.suite, responses)
		if err != nil {
			return err
		}
		err = p.SendToParent(&Response{aggResponse})
		if err != nil {
			return err
		}
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

	log.Lvl2(p.ServerIdentity(), "commitment sent with", mask.CountEnabled(), "accepted and", NRefusal, "refusals")

	return nil
}

// HandleStop is called when a Stop message is send to this node. It stops the node.
func (p *SubFtCosi) HandleStop(stop StructStop) error {
	if !isValidSender(stop.TreeNode, p.Root()) {
		log.Lvl2(p.ServerIdentity(), "received a Stop from node", stop.ServerIdentity,
			"that is not the root, ignored")
	}
	p.Shutdown()
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

	if p.Threshold == 0 {
		log.Lvl3("no threshold specified, using \"as much as possible\" policy")
	}

	announcement := StructAnnouncement{
		p.TreeNode(),
		Announcement{p.Msg, p.Data, p.Publics, p.Timeout, p.Threshold},
	}
	p.ChannelAnnouncement <- announcement
	return nil
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

	var secret kyber.Scalar //nil
	if accepts {
		secret, structCommitment.CoSiCommitment = cosi.Commit(p.suite)
		var personalMask *cosi.Mask
		personalMask, err = cosi.NewMask(p.suite, p.Publics, p.Public())
		if err != nil {
			return secret, StructCommitment{}, err
		}
		structCommitment.Mask = personalMask.Mask()
	} else { //refuses
		structCommitment.NRefusal++
	}

	return secret, structCommitment, nil
}

// checks if a node is in a list of nodes
func isValidSender(node *onet.TreeNode, valids ...*onet.TreeNode) bool {
	//check if comes from a committed children
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

// removes a node from a slice
func remove(nodesList []*onet.TreeNode, node *onet.TreeNode) []*onet.TreeNode {
	for i, iNode := range nodesList {
		if iNode.Equal(node) {
			return append(nodesList[:i], nodesList[i+1:]...)
		}
	}
	return nodesList
}

// returns the list of children present in a given mask
func (p *SubFtCosi) getChildrenInMask(byteMask []byte) ([]*onet.TreeNode, error) {
	mask, err := cosi.NewMask(p.suite, p.Publics, nil)
	if err != nil {
		return nil, err
	}
	err = mask.SetMask(byteMask)
	if err != nil {
		return nil, err
	}

	childrenInMask := make([]*onet.TreeNode, 0)
	for _, child := range p.Children() {
		isEnabled, err := mask.KeyEnabled(child.ServerIdentity.Public)
		if err != nil {
			return nil, err
		}
		if isEnabled {
			childrenInMask = append(childrenInMask, child)
		}
	}
	return childrenInMask, nil
}
