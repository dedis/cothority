package protocol

import (
	"fmt"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// init is done at startup. It defines every messages that is handled by the network
// and registers the protocols.
func init() {
	network.RegisterMessages(Announcement{}, Commitment{}, Challenge{}, Response{}, Stop{})

	onet.GlobalProtocolRegister(ProtocolName, NewProtocol)
	onet.GlobalProtocolRegister(subProtocolName, NewSubProtocol)
}

// CoSiRootNode holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
type CoSiRootNode struct {
	*onet.TreeNodeInstance

	NSubtrees        int
	Proposal         []byte
	CreateProtocol   CreateProtocolFunction
	ProtocolTimeout  time.Duration
	SubleaderTimeout time.Duration
	LeavesTimeout    time.Duration
	FinalSignature   chan []byte

	publics    []kyber.Point
	hasStopped bool //used since Shutdown can be called multiple time
	startChan  chan bool
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in CoSiRootNode protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewProtocol method is used to define the protocol.
func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {

	var list []kyber.Point
	for _, t := range n.Tree().List() {
		list = append(list, t.ServerIdentity.Public)
	}

	c := &CoSiRootNode{
		TreeNodeInstance: n,
		FinalSignature:   make(chan []byte),
		publics:          list,
		hasStopped:       false,
		startChan:        make(chan bool),
	}

	return c, nil
}

// Shutdown stops the protocol
func (p *CoSiRootNode) Shutdown() error {
	if !p.hasStopped {
		p.hasStopped = true
	}
	return nil
}

// Dispatch is the main method of the protocol, defining the root node behaviour
// and sequential handling of subprotocols.
func (p *CoSiRootNode) Dispatch() error {
	if !p.IsRoot() {
		return nil
	}

	select {
	case _, ok := <-p.startChan:
		if !ok {
			log.Lvl1("protocol finished prematurely")
			return nil
		}
		close(p.startChan)
	case <-time.After(DefaultProtocolTimeout):
		return fmt.Errorf("timeout, did you forget to call Start?")
	}

	log.Lvl1("leader protocol started")

	// generate trees
	nNodes := p.Tree().Size()
	trees, err := GenTrees(p.Tree().Roster, nNodes, p.NSubtrees)
	if err != nil {
		return fmt.Errorf("error in tree generation: %s", err)
	}

	// if one node, sign without subprotocols
	if nNodes == 1 {
		trees = make([]*onet.Tree, 0)
	}

	// start all subprotocols
	cosiSubProtocols := make([]*CoSiSubProtocolNode, len(trees))
	for i, tree := range trees {
		cosiSubProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			return err
		}
	}
	log.Lvl3("all protocols started")

	// get all commitments, restart subprotocols where subleaders do not respond
	commitments := make([]StructCommitment, 0)
	runningSubProtocols := make([]*CoSiSubProtocolNode, 0)
subtrees:
	for i, subProtocol := range cosiSubProtocols {
		for {
			select {
			case _ = <-subProtocol.subleaderNotResponding:
				subleaderID := trees[i].Root.Children[0].RosterIndex
				log.Lvlf2("subleader from tree %d (id %d) failed, restarting it", i, subleaderID)

				// send stop signal
				subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})

				// generate new tree
				newSubleaderID := subleaderID + 1
				if newSubleaderID >= len(trees[i].Roster.List) {
					log.Lvl2("subprotocol", i, "failed with every subleader, ignoring this subtree")
					continue subtrees
				}
				trees[i], err = GenSubtree(trees[i].Roster, newSubleaderID)
				if err != nil {
					return err
				}

				// restart subprotocol
				subProtocol, err = p.startSubProtocol(trees[i])
				if err != nil {
					return fmt.Errorf("error in restarting of subprotocol: %s", err)
				}
				cosiSubProtocols[i] = subProtocol
			case commitment := <-subProtocol.subCommitment:
				runningSubProtocols = append(runningSubProtocols, subProtocol)
				commitments = append(commitments, commitment)
				continue subtrees
			case <-time.After(p.ProtocolTimeout):
				return fmt.Errorf("didn't get commitment in time")
			}
		}
	}

	suite := suites.MustFind(p.Suite().String()) // convert network.Suite to full suite

	// generate challenge
	log.Lvl3("root-node generating global challenge")
	secret, commitment, finalMask, err := generateCommitmentAndAggregate(suite, p.TreeNodeInstance, p.publics, commitments)
	if err != nil {
		return err
	}

	cosiChallenge, err := cosi.Challenge(suite, commitment, finalMask.AggregatePublic, p.Proposal)
	if err != nil {
		return err
	}
	structChallenge := StructChallenge{p.TreeNode(), Challenge{cosiChallenge}}

	// send challenge to every subprotocol
	for _, coSiProtocol := range runningSubProtocols {
		subProtocol := coSiProtocol
		subProtocol.ChannelChallenge <- structChallenge
	}

	// get response from all subprotocols
	responses := make([]StructResponse, 0)
	for _, cosiSubProtocol := range runningSubProtocols {
		subProtocol := cosiSubProtocol
		select {
		case response := <-subProtocol.subResponse:
			responses = append(responses, response)
			continue
		case <-time.After(p.ProtocolTimeout):
			return fmt.Errorf("didn't finish in time")
		}
	}

	// signs the proposal
	response, err := generateResponse(suite, p.TreeNodeInstance, responses, secret, cosiChallenge)
	if err != nil {
		return err
	}
	log.Lvl3(p.ServerIdentity().Address, "starts final signature")
	var signature []byte
	signature, err = cosi.Sign(suite, commitment, response, finalMask)
	if err != nil {
		return err
	}
	p.FinalSignature <- signature

	log.Lvl3("Root-node is done without errors")
	return nil
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *CoSiRootNode) Start() error {
	if p.Proposal == nil {
		close(p.startChan)
		return fmt.Errorf("no proposal specified")
	}
	if p.CreateProtocol == nil {
		close(p.startChan)
		return fmt.Errorf("no create protocol function specified")
	}

	if p.NSubtrees < 1 {
		p.NSubtrees = 1
	}
	if p.ProtocolTimeout < 10 {
		p.ProtocolTimeout = DefaultProtocolTimeout
	}
	if p.SubleaderTimeout < 10 {
		p.SubleaderTimeout = DefaultSubleaderTimeout
	}
	if p.LeavesTimeout < 10 {
		p.LeavesTimeout = DefaultLeavesTimeout
	}

	log.Lvl3("Starting CoSi")
	p.startChan <- true
	return nil
}

// startSubProtocol creates, parametrize and starts a subprotocol on a given tree
// and returns the started protocol.
func (p *CoSiRootNode) startSubProtocol(tree *onet.Tree) (*CoSiSubProtocolNode, error) {

	pi, err := p.CreateProtocol(subProtocolName, tree)
	if err != nil {
		return nil, err
	}

	cosiSubProtocol := pi.(*CoSiSubProtocolNode)
	cosiSubProtocol.Publics = p.publics
	cosiSubProtocol.Proposal = p.Proposal
	cosiSubProtocol.SubleaderTimeout = p.SubleaderTimeout
	cosiSubProtocol.LeavesTimeout = p.LeavesTimeout

	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}
