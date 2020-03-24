package skipchain

/*
The `NewProtocol` method is used to define the protocol and to register
the handlers that will be called if a certain type of message is received.
The handlers will be treated according to their signature.

The protocol-file defines the actions that the protocol needs to do in each
step. The root-node will call the `Start`-method of the protocol. Each
node will only use the `Handle`-methods, and not call `Start` again.
*/

import (
	"errors"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// ProtocolExtendRoster asks a remote node if he would accept to participate
// in a skipchain with a given id.
const ProtocolExtendRoster = "scExtendRoster"

// ProtocolGetBlocks asks a remote node for some blocks.
const ProtocolGetBlocks = "scGetBlocks"

func init() {
	onet.GlobalProtocolRegister(ProtocolExtendRoster, NewProtocolExtendRoster)
	onet.GlobalProtocolRegister(ProtocolGetBlocks, NewProtocolGetBlocks)
}

// ExtendRoster is used for different communications in the skipchain-service.
type ExtendRoster struct {
	*onet.TreeNodeInstance

	ExtendRoster      *ProtoExtendRoster
	ExtendRosterReply chan []ProtoExtendSignature
	Followers         *[]FollowChainType
	FollowerIDs       []SkipBlockID
	DB                *SkipBlockDB
	SaveCallback      func()
	tempSigs          []ProtoExtendSignature
	tempSigsMutex     sync.Mutex
	// TODO make sure all new nodes are OK
	// new roster in ExtendRoster
	// previous roster in one block back
	allowedFailures int
	doneChan        chan int

	closingMutex sync.Mutex
	closed       bool
	closing      chan bool
}

// NewProtocolExtendRoster prepares for a protocol that checks if a roster can
// be extended.
func NewProtocolExtendRoster(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &ExtendRoster{
		TreeNodeInstance:  n,
		ExtendRosterReply: make(chan []ProtoExtendSignature),
		// it's hardcoded at the moment, maybe the caller can specify
		allowedFailures: (len(n.Roster().List) - 1) / 3,
		closing:         make(chan bool),
		doneChan:        make(chan int, 1),
	}
	return t, t.RegisterHandlers(t.HandleExtendRoster, t.HandleExtendRosterReply)
}

// Start sends the extend roster request to all of the children.
func (p *ExtendRoster) Start() error {
	log.Lvl3("Starting Protocol ExtendRoster")
	go func() {
		errs := p.SendToChildrenInParallel(p.ExtendRoster)
		if len(errs) > p.allowedFailures {
			log.Errorf("Send to children failed: %v", errs)
		}
	}()
	return nil
}

// HandleExtendRoster uses the stored followers to decide if we want to accept
// to be part of the new roster.
func (p *ExtendRoster) HandleExtendRoster(msg ProtoStructExtendRoster) error {
	defer p.Done()

	log.Lvlf3("%s: Check block with roster %s", p.ServerIdentity(), msg.Block.Roster.List)
	if p.isBlockAccepted(msg.ServerIdentity, &msg.Block) {
		sig, err := schnorr.Sign(cothority.Suite, p.Host().ServerIdentity.GetPrivate(), msg.Block.SkipChainID())
		if err != nil {
			log.Error("couldn't sign genesis-block")
			return p.SendToParent(&ProtoExtendRosterReply{})
		}
		return p.SendToParent(&ProtoExtendRosterReply{Signature: &sig})
	}

	return p.SendToParent(&ProtoExtendRosterReply{})
}

func (p *ExtendRoster) isBlockAccepted(sender *network.ServerIdentity, block *SkipBlock) bool {
	// Search for matching skipchain-ids
	log.Lvlf3("%s: checking block with skipchainid: %x", p.ServerIdentity(), block.SkipChainID())
	for _, id := range p.FollowerIDs {
		if block.SkipChainID().Equal(id) {
			log.Lvlf3("%s: Found skipchain-id", p.ServerIdentity())
			return true
		}
	}

	// If followers are defined, first search the latest block, then verify if
	// we're still OK to handle new blocks for that skipchain.
	if p.Followers != nil && len(*p.Followers) > 0 {
		for _, fct := range *p.Followers {
			log.Lvlf3("%s: Checking skipchain %x", p.ServerIdentity(), fct.Block.SkipChainID())
			// See if its in this skipchain
			if fct.Block.SkipChainID().Equal(block.SkipChainID()) {
				log.Lvlf3("%s: Accepted existing skipchain", p.ServerIdentity())
				return true
			}

			// Get the latest skipblock available
			err := fct.GetLatest(p.ServerIdentity(), p)
			if err != nil {
				log.Error(err)
			}

			// Verify if we still accept the new block, given the definition of this
			// new skipchain.
			if fct.AcceptNew(block, p.ServerIdentity()) {
				log.Lvlf3("%s: accepted new block", p.ServerIdentity())
				return true
			}
		}
		log.Lvlf3("%s: refused new block", p.ServerIdentity())
		return false
	}

	if p.SaveCallback != nil {
		p.SaveCallback()
	}

	// If no followers are present, we accept everything.
	log.Lvlf3("%s: will return %t", p.ServerIdentity(), len(p.FollowerIDs) == 0)
	return len(p.FollowerIDs) == 0
}

// HandleExtendRosterReply checks if enough nodes are OK to hold the new block.
func (p *ExtendRoster) HandleExtendRosterReply(r ProtoStructExtendRosterReply) error {
	// HORRIBLE HACK to give handler a timeout behaviour
	// only the first call to HandleExtendRosterReply will have empty tempSigs
	if len(p.tempSigs) == 0 {
		go func() {
			select {
			case <-p.doneChan:
				return
			case <-time.After(time.Second):
				p.Done()

				p.tempSigsMutex.Lock()
				defer p.tempSigsMutex.Unlock()

				if len(p.tempSigs) >= len(p.Children())-p.allowedFailures {
					p.ExtendRosterReply <- p.tempSigs
				} else {
					p.ExtendRosterReply <- []ProtoExtendSignature{}
				}
			case <-p.closing:
				return
			}
		}()
	}

	p.tempSigsMutex.Lock()
	defer p.tempSigsMutex.Unlock()
	ok := func() bool {
		if r.Signature == nil {
			return false
		}
		if schnorr.Verify(cothority.Suite, r.ServerIdentity.Public, p.ExtendRoster.Block.SkipChainID(), *r.Signature) != nil {
			log.Lvl3("Signature verification failed")
			return false
		}
		p.tempSigs = append(p.tempSigs, ProtoExtendSignature{SI: r.ServerIdentity.ID, Signature: *r.Signature})
		return true
	}()
	// if a single node disagrees, we fail
	if !ok {
		p.Done()
		p.ExtendRosterReply <- []ProtoExtendSignature{}
		p.doneChan <- 1
	} else {
		// ideally we collect all the signatures
		if len(p.tempSigs) == len(p.Children()) {
			p.Done()
			p.ExtendRosterReply <- p.tempSigs
			p.doneChan <- 1
		}
	}
	return nil
}

// Shutdown makes sure the protocol stops if the server goes down. This is
// mostly in testing.
func (p *ExtendRoster) Shutdown() error {
	close(p.closing)
	return nil
}

// GetBlocks is used for conodes to get blocks from each other.
type GetBlocks struct {
	*onet.TreeNodeInstance

	GetBlocks      *ProtoGetBlocks
	GetBlocksReply chan []*SkipBlock
	DB             *SkipBlockDB
	replies        int
}

// NewProtocolGetBlocks prepares for a protocol that fetches blocks.
func NewProtocolGetBlocks(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &GetBlocks{
		TreeNodeInstance: n,
		GetBlocksReply:   make(chan []*SkipBlock, 1),
	}
	return t, t.RegisterHandlers(t.HandleGetBlocks, t.HandleGetBlocksReply)
}

// Start sends the block request to all of the children.
func (p *GetBlocks) Start() error {
	log.Lvl3("Starting Protocol GetBlocks")
	go func() {
		errs := p.SendToChildrenInParallel(p.GetBlocks)
		if len(errs) > 0 {
			log.Lvlf1("Error while sending to children: %+v", errs)
		}
	}()
	return nil
}

// HandleGetBlocks returns a given number of blocks from the skipchain,
// starting from a given block. If skipping is true, it will skip forward
// as far as possible, otherwise it will advance one block at a time.
func (p *GetBlocks) HandleGetBlocks(msg ProtoStructGetBlocks) error {
	defer p.Done()

	if p.DB == nil {
		return errors.New("no DB available")
	}

	n := msg.Count
	result := make([]*SkipBlock, 0, n)
	next := msg.SBID
	lastIdx := -1
	for n > 0 {
		// TODO: see if this could be optimised by using multiple bucket.Get in a
		// single transaction.
		s := p.DB.GetByID(next)
		if s == nil {
			break
		}
		last := len(result) - 1
		if last >= 0 && s.Index <= result[last].Index {
			return ErrorInconsistentForwardLink
		}

		result = append(result, s)
		lastIdx = s.Index
		n--

		// Find the next one (or exit if we are at the latest)
		if len(s.ForwardLink) == 0 {
			break
		}

		linkNum := 0
		if msg.Skipping {
			linkNum = len(s.ForwardLink) - 1
		}
		next = s.ForwardLink[linkNum].To
	}
	log.Lvlf2("%v: GetBlocks reply: %v blocks, last index %v", p.ServerIdentity(), len(result), lastIdx)
	return p.SendToParent(&ProtoGetBlocksReply{SkipBlocks: result})
}

// HandleGetBlocksReply contacts the service that a new block has arrived
func (p *GetBlocks) HandleGetBlocksReply(msg ProtoStructGetBlocksReply) error {

	// Take the first non-nil answer, or send a nil reply if all nodes
	// replied that they don't know the blocks.
	p.replies++
	blocksReply := msg.SkipBlocks
	if p.replies == len(p.Children()) || len(blocksReply) > 0 {
		p.GetBlocksReply <- blocksReply
		p.Done()
	}
	return nil
}
