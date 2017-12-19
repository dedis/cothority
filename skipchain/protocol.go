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
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// ProtocolExtendRoster asks a remote node if he would accept to participate
// in a skipchain with a given id.
const ProtocolExtendRoster = "scExtendRoster"

// ProtocolGetUpdate asks a remote node to return the latest block of a
// skipchain.
const ProtocolGetUpdate = "scGetUpdate"

func init() {
	onet.GlobalProtocolRegister(ProtocolExtendRoster, NewProtocolExtendRoster)
	onet.GlobalProtocolRegister(ProtocolGetUpdate, NewProtocolGetUpdate)
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
}

// GetUpdate needs to be configured by the service to hold the database
// of all skipblocks.
type GetUpdate struct {
	*onet.TreeNodeInstance

	GetUpdate      *ProtoGetUpdate
	GetUpdateReply chan *SkipBlock
	DB             *SkipBlockDB
}

// NewProtocolExtendRoster prepares for a protocol that checks if a roster can
// be extended.
func NewProtocolExtendRoster(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &ExtendRoster{
		TreeNodeInstance:  n,
		ExtendRosterReply: make(chan []ProtoExtendSignature),
	}
	return t, t.RegisterHandlers(t.HandleExtendRoster, t.HandleExtendRosterReply)
}

// NewProtocolGetUpdate prepares for a protocol that fetches an update
func NewProtocolGetUpdate(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &GetUpdate{
		TreeNodeInstance: n,
		GetUpdateReply:   make(chan *SkipBlock),
	}
	return t, t.RegisterHandlers(t.HandleGetUpdate, t.HandleBlockReply)
}

// Start sends the Announce-message to all children
func (p *ExtendRoster) Start() error {
	log.Lvl3("Starting Protocol ExtendRoster")
	return p.SendToChildren(p.ExtendRoster)
}

// Start sends the Announce-message to all children
func (p *GetUpdate) Start() error {
	log.Lvl3("Starting Protocol GetUpdate")
	return p.SendToChildren(p.GetUpdate)
}

// HandleExtendRoster uses the stored followers to decide if we want to accept
// to be part of the new roster.
func (p *ExtendRoster) HandleExtendRoster(msg ProtoStructExtendRoster) error {
	defer p.Done()

	log.Lvlf3("%s: Check block with roster %s", p.ServerIdentity(), msg.Block.Roster.List)
	if p.isBlockAccepted(msg.ServerIdentity, &msg.Block) {
		sig, err := schnorr.Sign(Suite, p.Private(), msg.Block.SkipChainID())
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

// HandleExtendRosterReply checks if all nodes are OK to hold this new block.
func (p *ExtendRoster) HandleExtendRosterReply(reply []ProtoStructExtendRosterReply) error {
	defer p.Done()

	var sigs []ProtoExtendSignature
	for _, r := range reply {
		if r.Signature == nil {
			sigs = []ProtoExtendSignature{}
			break
		}
		if schnorr.Verify(Suite, r.ServerIdentity.Public, p.ExtendRoster.Block.SkipChainID(), *r.Signature) != nil {
			sigs = []ProtoExtendSignature{}
			break
		}
		sigs = append(sigs, ProtoExtendSignature{SI: r.ServerIdentity.ID, Signature: *r.Signature})
	}
	p.ExtendRosterReply <- sigs
	return nil
}

// HandleGetUpdate searches for a skipblock and returns it if it is found.
func (p *GetUpdate) HandleGetUpdate(msg ProtoStructGetUpdate) error {
	defer p.Done()

	if p.DB == nil {
		log.Lvl3(p.ServerIdentity(), "no block stored in Db")
		return p.SendToParent(&ProtoBlockReply{})
	}

	sb, err := p.DB.GetLatest(p.DB.GetByID(msg.SBID))
	if err != nil {
		log.Error("couldn't get latest: " + err.Error())
		return err
	}
	return p.SendToParent(&ProtoBlockReply{SkipBlock: sb})
}

// HandleBlockReply contacts the service that a new block has arrived
func (p *GetUpdate) HandleBlockReply(msg ProtoStructBlockReply) error {
	defer p.Done()
	p.GetUpdateReply <- msg.SkipBlock
	return nil
}
