package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func init() {
	GlobalRegisterDefaultProtocols()
}

// sub_protocol is run by each sub-leader and each node once, and n times by
// the root leader, where n is the number of sub-leader.

// SubBlsFtCosi holds the different channels used to receive the different protocol messages.
type SubBlsFtCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	stoppedOnce    sync.Once
	verificationFn VerificationFn
	suite          pairing.Suite

	// protocol/subprotocol channels
	// these are used to communicate between the subprotocol and the main protocol
	subleaderNotResponding chan bool
	subResponse            chan StructResponse

	// internodes channels
	ChannelAnnouncement chan StructAnnouncement
	ChannelResponse     chan StructResponse
}

// NewDefaultSubProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewDefaultSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubBlsFtCosi(n, vf, bn256.NewSuiteG2())
}

// NewSubBlsFtCosi is used to define the subprotocol and to register
// the channels where the messages will be received.
func NewSubBlsFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite pairing.Suite) (onet.ProtocolInstance, error) {
	// tests if it's a three level tree
	moreThreeLevel := false
	n.Tree().Root.Visit(0, func(depth int, n *onet.TreeNode) {
		if depth > 2 {
			moreThreeLevel = true
		}
	})
	if moreThreeLevel {
		return nil, fmt.Errorf("subBlsFtCosi launched with a more than three level tree")
	}

	c := &SubBlsFtCosi{
		TreeNodeInstance: n,
		verificationFn:   vf,
		suite:            suite,
	}

	if n.IsRoot() {
		c.subleaderNotResponding = make(chan bool, 1)
		c.subResponse = make(chan StructResponse, 2) // can send 2 responses
	}

	err := c.RegisterChannels(&c.ChannelAnnouncement, &c.ChannelResponse)
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
func (p *SubBlsFtCosi) Dispatch() error {
	defer func() {
		if p.IsRoot() {
			err := p.Broadcast(&Stop{})
			if err != nil {
				log.Error("error while broadcasting stopping message:", err)
			}
		}
		p.Done()
	}()
	log.Lvl3("SubBlsFtCosi: Public keys are", p.Roster().Publics())
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
	p.Timeout = announcement.Timeout
	if !p.IsRoot() {
		// We'll be only waiting on the root and the subleaders. The subleaders
		// only have half of the time budget of the root.
		// TODO: Check if we need to change the timeout for BLS protocol
		p.Timeout /= 2
	}
	p.Msg = announcement.Msg
	p.Data = announcement.Data
	p.Threshold = announcement.Threshold

	// verify that threshold is valid
	maxThreshold := p.Tree().Size() - 1
	if p.Threshold > maxThreshold {
		return fmt.Errorf("threshold %d bigger than the maximum of responses this subtree can gather (%d)", p.Threshold, maxThreshold)
	}

	// start the verification in background if I'm not the root because
	// root does the verification in the main protocol
	if !p.IsRoot() {
		go func() {
			log.Lvl3(p.ServerIdentity(), "starting verification in the background")
			verificationOk := p.verificationFn(p.Msg, p.Data)
			personalResponse, err := p.getResponse(verificationOk, p.Roster().Publics(), p.Private())
			if err != nil {
				log.Error("error while generating own commitment:", err)
				return
			}
			p.ChannelResponse <- personalResponse
			log.Lvl3(p.ServerIdentity(), "verification done:", verificationOk)
		}()
	}

	if !p.IsLeaf() {
		// Only send messages if the node has children
		go func() {
			if errs := p.SendToChildrenInParallel(&announcement.Announcement); len(errs) > 0 {
				log.Error(p.ServerIdentity(), "failed to send announcement to all children, trying to continue")
			}
		}()
	}

	// ----- Response -----
	// Collect all responses from children, store them and wait till all have responded or timed out.
	var responses = make([]StructResponse, 0)                       // list of received responses
	var nodesCanRespond = make([]*onet.TreeNode, len(p.Children())) // the list of nodes that can respond. Nodes will be removed from the list once they respond.

	var Refusals = make(map[int][]byte) // refusals received. Will be used only for the subleader

	var firstResponseSent = false // to avoid sending the quick response multiple times
	var t = time.After(p.Timeout) // the timeout

	copy(nodesCanRespond, p.Children())
	if p.IsRoot() {
		nodesCanRespond = append(nodesCanRespond, p.Children()...) // every node can send quick and final answer
	} else {
		nodesCanRespond = append(nodesCanRespond, p.TreeNode()) // can have its own response
	}

loop:
	for len(nodesCanRespond) > 0 {
		select {
		case response, channelOpen := <-p.ChannelResponse:
			if !channelOpen {
				return nil
			}

			if !isValidSender(response.TreeNode, nodesCanRespond...) {
				log.Warn(p.ServerIdentity(), "received a Response from node", response.ServerIdentity,
					"that is not in the list of nodes that can still respond, ignored")
				break // discards it

			}
			nodesCanRespond = remove(nodesCanRespond, response.TreeNode)

			// verify mask of the received response
			verificationMask, err := NewMask(p.suite, p.Roster().Publics(), -1)
			if err != nil {
				return err
			}
			err = verificationMask.SetMask(response.Mask)
			if err != nil {
				return err
			}

			if p.IsRoot() {

				// verify the refusals is signed correctly
				for index, refusal := range response.Refusals {
					refusalMsg := []byte(fmt.Sprintf("%s:%d", p.Msg, index))
					refusalPublic := p.Roster().Publics()[index]
					err = bls.Verify(p.suite, refusalPublic, refusalMsg, refusal)
					// Do not send invalid refusals
					if err != nil {
						delete(response.Refusals, index)
					}
				}

				// send response to super-protocol
				p.subResponse <- response

				// Check if the response is the final response
				// Should we verify if the refusals are correctly signed?
				if verificationMask.CountEnabled()+len(response.Refusals) == len(p.List())-1 {
					return nil
				}
			} else {

				if verificationMask.CountEnabled() > 1 {
					log.Warn(p.ServerIdentity(), "received response with ill-formed mask in non-root node: has",
						verificationMask.CountEnabled(), "nodes enabled instead of 0 or 1, ignored")
					break
				}

				// check if response is a refusal or acceptance
				sign, _ := signedByteSliceToPoint(p.suite, response.CoSiReponse)
				if sign.Equal(p.suite.G1().Point()) { // refusal
					// verify the refusal is signed correctly
					for index, refusal := range response.Refusals {
						refusalMsg := []byte(fmt.Sprintf("%s:%d", p.Msg, index))
						refusalPublic := p.Roster().Publics()[index]
						err = bls.Verify(p.suite, refusalPublic, refusalMsg, refusal)
						// ignore the refusal if not properly signed.
						if err == nil {
							Refusals[index] = refusal
						}
					}

					if p.IsLeaf() {
						log.Warn(p.ServerIdentity(), "leaf refused Response, marking as not signed")
						return p.sendAggregatedResponses(p.Roster().Publics(), []StructResponse{}, Refusals)
					}
					log.Warn(p.ServerIdentity(), "non-leaf got refusal")
				} else {
					//accepted
					responses = append(responses, response)
				}

				thresholdRefusal := (1 + len(p.Children()) - p.Threshold) + 1

				// checks if threshold is reached or unreachable
				quickAnswer := !firstResponseSent &&
					(len(responses) >= p.Threshold || // quick valid answer
						len(Refusals) >= thresholdRefusal) // quick refusal answer

				// checks if every child and himself responded
				finalAnswer := len(responses)+len(Refusals) == len(p.Children())+1

				if quickAnswer || finalAnswer {

					err = p.sendAggregatedResponses(p.Roster().Publics(), responses, Refusals)
					if err != nil {
						return err
					}

					// return if final response
					if finalAnswer {
						break loop
					}

					firstResponseSent = true
				}

				// security check
				if len(responses)+len(Refusals) > maxThreshold {
					log.Error(p.ServerIdentity(), "more responses (", len(responses),
						") and refusals (", len(Refusals), ") than possible in subleader (", maxThreshold, ")")
				}
			}
		case <-t:
			if p.IsRoot() {
				log.Warn(p.ServerIdentity(), "timed out while waiting for subleader response")
				p.subleaderNotResponding <- true
				return nil
			}
			log.Warn(p.ServerIdentity(), "timed out while waiting for commits, got", len(responses), "commitments and", len(Refusals), "refusals")

			// sending responses received
			// TODO - Only send if there are newer responses
			err = p.sendAggregatedResponses(p.Roster().Publics(), responses, Refusals)
			if err != nil {
				return err
			}
			break loop
		}
	}
	return nil
}

func (p *SubBlsFtCosi) sendAggregatedResponses(publics []kyber.Point, responses []StructResponse, Refusals map[int][]byte) error {

	// aggregate responses
	responsePoint, mask, err := aggregateResponses(p.suite, publics, responses)
	if err != nil {
		return err
	}

	response, err := responsePoint.MarshalBinary()
	if err != nil {
		return err
	}

	// send to parent
	err = p.SendToParent(&Response{response, mask.Mask(), Refusals})
	if err != nil {
		return err
	}

	log.Lvl3(p.ServerIdentity(), "response sent with", mask.CountEnabled(), "accepted and", len(Refusals), "refusals")

	return nil
}

// HandleStop is called when a Stop message is send to this node.
// It broadcasts the message to all the nodes in tree and each node will stop
// the protocol by calling p.Done.
func (p *SubBlsFtCosi) HandleStop(stop StructStop) error {
	if !isValidSender(stop.TreeNode, p.Root()) {
		log.Warn(p.ServerIdentity(), "received a Stop from node", stop.ServerIdentity,
			"that is not the root, ignored")
	}
	log.Lvl3("Received stop", p.ServerIdentity())
	close(p.ChannelAnnouncement)
	// close(p.ChannelResponse) // Channel left open to allow verification function to safely return
	return nil
}

// Start is done only by root and starts the subprotocol
func (p *SubBlsFtCosi) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting subCoSi")
	if p.Msg == nil {
		return errors.New("subprotocol does not have a proposal msg")
	}
	if p.Data == nil {
		return errors.New("subprotocol does not have data, it can be empty but cannot be nil")
	}
	if p.Roster().Publics() == nil || len(p.Roster().Publics()) < 1 {
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
		Announcement{p.Msg, p.Data, p.Timeout, p.Threshold},
	}
	p.ChannelAnnouncement <- announcement
	return nil
}

// generates a response.
// the boolean indicates whether the response is a proposal acceptance or a proposal refusal.
// Returns the response and an error if there was a problem in the process.
func (p *SubBlsFtCosi) getResponse(accepts bool, publics []kyber.Point, private kyber.Scalar) (StructResponse, error) {

	personalMask, err := NewMask(p.suite, publics, -1)
	if err != nil {
		return StructResponse{}, err
	}

	personalSig, err := p.suite.G1().Point().MarshalBinary()
	if err != nil {
		return StructResponse{}, err
	}

	Refusals := make(map[int][]byte)

	if accepts {
		personalMask, err = NewMask(p.suite, publics, p.Index())
		if err != nil {
			return StructResponse{}, err
		}

		personalSig, err = bls.Sign(p.suite, private, p.Msg)
		if err != nil {
			return StructResponse{}, err
		}

	} else { // refuses
		refusalMsg := []byte(fmt.Sprintf("%s:%d", p.Msg, p.Index()))
		refusalSig, err := bls.Sign(p.suite, private, refusalMsg)
		if err != nil {
			return StructResponse{}, err
		}
		Refusals[p.Index()] = refusalSig
	}

	structResponse := StructResponse{p.TreeNode(),
		Response{personalSig, personalMask.Mask(), Refusals}}
	return structResponse, nil
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
