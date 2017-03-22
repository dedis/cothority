package skipchain

import (
	"errors"

	"strconv"

	"time"

	"fmt"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/messaging"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"
const bftNewBlock = "SkipchainBFTNew"
const bftFollowBlock = "SkipchainBFTFollow"

func init() {
	skipchainSID, _ = onet.RegisterNewService(ServiceName, newSkipchainService)
	network.RegisterMessage(&SkipBlockMap{})
}

// Only used in tests
var skipchainSID onet.ServiceID

// Name used to store skipblocks
const skipblocksID = "skipblocks"

// Service handles adding new SkipBlocks
type Service struct {
	*onet.ServiceProcessor
	// Sbm is the skipblock-map that holds all known skipblocks to this service.
	Sbm           *SkipBlockMap
	propagate     messaging.PropagationFunc
	verifiers     map[VerifierID]SkipBlockVerifier
	blockRequests map[string]chan *SkipBlock
}

// StoreSkipBlock stores a new skipblock in the system. This can be either a
// genesis-skipblock, that will create a new skipchain, or a new skipblock,
// that will be added to an existing chain.
//
// The conode servicing the request needs to be part of the actual valid latest
// skipblock, else it will fail.
//
// It takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be stored, else it will be discarded and an error will be returned.
//
// If the latest block given is nil it will create a new skipchain and store
// the given block as genesis-block.
//
// If the latest block is non-nil and exists, the skipblock will be added to the
// skipchain after verification that it fits and no other block already has been
// added.
func (s *Service) StoreSkipBlock(psbd *StoreSkipBlock) (network.Message, onet.ClientError) {
	prop := psbd.NewBlock
	var prev *SkipBlock
	var changed []*SkipBlock

	if psbd.LatestID.IsNull() {
		// A new chain is created, suppose all arguments in SkipBlock
		// are correctly set up
		prop.Index = 0
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*BlockLink, 0)
		// genesis block has a random back-link:
		bl := random.Bytes(32, random.Stream)
		prop.BackLinkIDs = []SkipBlockID{SkipBlockID(bl)}
		prop.GenesisID = prop.updateHash()
		err := s.verifyBlock(prop)
		if err != nil {
			return nil, onet.NewClientErrorCode(ErrorParameterWrong,
				err.Error())
		}
		if !prop.ParentBlockID.IsNull() {
			parent, ok := s.Sbm.GetByID(prop.ParentBlockID)
			log.Print(parent, ok)
			if !ok {
				return nil, onet.NewClientErrorCode(ErrorParameterWrong,
					"Didn't find parent")
			}
			parent.ChildSL = append(parent.ChildSL, prop.Hash)
			changed = append(changed, parent)
		}
		changed = append(changed, prop)
	} else {
		// We're appending a block to an existing chain
		var ok bool
		prev, ok = s.Sbm.GetByID(psbd.LatestID)
		if !ok {
			return nil, onet.NewClientErrorCode(ErrorBlockNotFound,
				"Didn't find latest block")
		}
		if i, _ := prev.Roster.Search(s.ServerIdentity().ID); i < 0 {
			return nil, onet.NewClientErrorCode(ErrorBlockContent,
				"We're not responsible for latest block")
		}
		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.ParentBlockID = nil
		prop.VerifierIDs = prev.VerifierIDs
		prop.Index = prev.Index + 1
		prop.GenesisID = prev.GenesisID
		index := prop.Index
		for prop.Height = 1; index%prop.BaseHeight == 0; prop.Height++ {
			index /= prop.BaseHeight
			if prop.Height >= prop.MaximumHeight {
				break
			}
		}
		log.Lvl4("Found height", prop.Height, "for index", prop.Index,
			"and maxHeight", prop.MaximumHeight, "and base", prop.BaseHeight)
		prop.BackLinkIDs = make([]SkipBlockID, prop.Height)
		pointer := prev
		for h := range prop.BackLinkIDs {
			for pointer.Height < h+1 {
				var ok bool
				pointer, ok = s.Sbm.GetByID(pointer.BackLinkIDs[0])
				if !ok {
					return nil, onet.NewClientErrorCode(ErrorBlockNotFound,
						"Didn't find convenient SkipBlock for height "+
							strconv.Itoa(h))
				}
			}
			prop.BackLinkIDs[h] = pointer.Hash
		}
		prop.updateHash()
		if err := s.addForwardLink(prev, prop, 1); err != nil {
			return nil, onet.NewClientErrorCode(ErrorBlockContent,
				"Couldn't get forward signature on block.")
		}
		changed = append(changed, prev, prop)
		for i, bl := range prop.BackLinkIDs[1:] {
			back, ok := s.Sbm.GetByID(bl)
			if !ok {
				return nil, onet.NewClientErrorCode(ErrorBlockContent,
					"Didn't get skipblock in back-link")
			}
			if err := s.SendRaw(back.Roster.RandomServerIdentity(),
				&ForwardSignature{i + 1, prev.Hash, prop, prev.ForwardLink[0]}); err != nil {
				// This is not a critical failure - we have at least
				// one forward-link
				log.Error("Couldn't get old block to sign")
			} else {
				changed = append(changed, back)
			}
		}
	}
	el, err := prop.GetResponsible(s.Sbm)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorBlockContent, err.Error())
	}
	prop.RespPublic = el.Roster.Publics()

	if err := s.startPropagation(changed); err != nil {
		return nil, onet.NewClientErrorCode(ErrorVerification,
			"Couldn't propagate new blocks: "+err.Error())
	}
	s.save()

	reply := &StoreSkipBlockReply{
		Previous: prev,
		Latest:   prop,
	}
	return reply, nil
}

// GetUpdateChain returns a slice of SkipBlocks which describe the part of the
// skipchain from the latest block the caller knows of to the actual latest
// SkipBlock.
// Somehow comparable to search in SkipLists.
func (s *Service) GetUpdateChain(latestKnown *GetUpdateChain) (network.Message, onet.ClientError) {
	block, ok := s.Sbm.GetByID(latestKnown.LatestID)
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "Couldn't find latest skipblock")
	}
	// at least the latest know and the next block:
	blocks := []*SkipBlock{block}
	log.Lvlf3("Starting to search chain at %x", s.Context.ServerIdentity().ID[0:8])
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[len(block.ForwardLink)-1]
		next, ok := s.Sbm.GetByID(link.Hash)
		if !ok {
			log.Lvl3("Didn't find next block, updating block")
			var err error
			next, err = s.getUpdateBlock(block, link.Hash)
			if err != nil {
				return nil, onet.NewClientErrorCode(ErrorBlockNotFound,
					err.Error())
			}
		} else {
			if i, _ := next.Roster.Search(s.ServerIdentity().ID); i < 0 {
				log.Lvl3("We're not responsible for", next, "- asking for update")
				var err error
				next, err = s.getUpdateBlock(next, link.Hash)
				if err != nil {
					return nil, onet.NewClientErrorCode(ErrorBlockNotFound,
						err.Error())
				}
			}
		}
		block = next
		blocks = append(blocks, next)
	}
	log.Lvl3("Found", len(blocks), "blocks")
	reply := &GetUpdateChainReply{blocks}

	return reply, nil
}

func (s *Service) getUpdateBlock(known *SkipBlock, unknown SkipBlockID) (*SkipBlock, error) {
	s.blockRequests[string(unknown)] = make(chan *SkipBlock)
	node := known.Roster.RandomServerIdentity()
	if err := s.SendRaw(node,
		&GetBlock{unknown}); err != nil {
		return nil, errors.New("Couldn't get updated block: " + known.String())
	}
	var block *SkipBlock
	select {
	case block = <-s.blockRequests[string(unknown)]:
		log.Lvl3("Got block", block)
		delete(s.blockRequests, string(unknown))
	case <-time.After(time.Millisecond * time.Duration(propagateTimeout)):
		delete(s.blockRequests, string(unknown))
		return nil, errors.New("Couldn't get updated block in time: " + unknown.String())
	}
	return block, nil
}

// forwardSignature receives a signature request of a newly accepted block.
// It only needs the 2nd-newest block and the forward-link.
func (s *Service) forwardSignature(env *network.Envelope) {
	err := func() error {
		fs, ok := env.Msg.(*ForwardSignature)
		if !ok {
			return errors.New("Didn't receive a ForwardSignature")
		}
		if fs.TargetHeight >= len(fs.Newest.BackLinkIDs) {
			return errors.New("This backlink-height doesn't exist")
		}
		target, ok := s.Sbm.GetByID(fs.Newest.BackLinkIDs[fs.TargetHeight])
		if !ok {
			return errors.New("Didn't find target-block")
		}
		data, err := network.Marshal(fs)
		if err != nil {
			return err
		}
		// TODO: is this really signed by target.roster?
		sig, err := s.startBFT(bftFollowBlock, target.Roster, fs.ForwardLink.Hash, data)
		if err != nil {
			return errors.New("Couldn't get signature")
		}
		target.ForwardLink = append(target.ForwardLink, &BlockLink{fs.ForwardLink.Hash, sig.Sig})
		s.startPropagation([]*SkipBlock{target})
		return nil
	}()
	if err != nil {
		log.Error(err)
	}
}

func (s *Service) getBlock(env *network.Envelope) {
	gb, ok := env.Msg.(*GetBlock)
	if !ok {
		log.Error("Didn't receive GetBlock")
		return
	}
	sb, ok := s.Sbm.GetByID(gb.ID)
	if !ok {
		log.Error("Did not find block")
		return
	}
	if i, _ := sb.Roster.Search(s.ServerIdentity().ID); i < 0 {
		log.Lvl3("Not responsible for that block, recursing")
		var err error
		sb, err = s.getUpdateBlock(sb, sb.Hash)
		if err != nil {
			log.Error(err)
			sb = nil
		}
	}
	if err := s.SendRaw(env.ServerIdentity, &GetBlockReply{sb}); err != nil {
		log.Error(err)
	}
}

func (s *Service) getBlockReply(env *network.Envelope) {
	gbr, ok := env.Msg.(*GetBlockReply)
	if !ok {
		log.Error("Didn't receive GetBlock")
		return
	}
	if err := gbr.SkipBlock.VerifyLinks(s.Sbm); err != nil {
		log.Error("Received invalid skipblock: " + err.Error())
	}
	id := s.Sbm.Store(gbr.SkipBlock)
	log.Lvl3("Sending block to channel")
	s.blockRequests[string(id)] <- gbr.SkipBlock
}

// verifyFollowBlock makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftVerifyFollowBlock(msg []byte, data []byte) bool {
	err := func() error {
		_, fsInt, err := network.Unmarshal(data)
		if err != nil {
			return err
		}
		fs, ok := fsInt.(*ForwardSignature)
		if !ok {
			return errors.New("Didn't receive a ForwardSignature")
		}
		previous, ok := s.Sbm.GetByID(fs.Previous)
		if !ok {
			return errors.New("Didn't find newest block")
		}
		newest := fs.Newest
		if len(newest.BackLinkIDs) <= fs.TargetHeight {
			return errors.New("Asked for signing too high a backlink")
		}
		if err := fs.ForwardLink.VerifySignature(previous.Roster.Publics()); err != nil {
			return errors.New("Wrong forward-link signature: " + err.Error())
		}
		if !fs.ForwardLink.Hash.Equal(newest.Hash) {
			return errors.New("No forward-link from previous to newest")
		}
		target, ok := s.Sbm.GetByID(newest.BackLinkIDs[fs.TargetHeight])
		if !ok {
			return errors.New("Don't have target-block")
		}
		if len(target.ForwardLink) >= fs.TargetHeight+1 {
			return errors.New("Already have forward-link at height " +
				strconv.Itoa(fs.TargetHeight+1))
		}
		if !target.GenesisID.Equal(newest.GenesisID) {
			return errors.New("Target and newest not from same skipchain")
		}
		return nil
	}()
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

// verifyNewBlock makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftVerifyNewBlock(msg []byte, data []byte) bool {
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	src := data[0:32]
	srcSB, ok := s.Sbm.GetByID(src)
	if !ok {
		log.Error("Didn't find src-skipblock")
		return false
	}
	_, dstN, err := network.Unmarshal(data[32:])
	if err != nil {
		log.Error("Couldn't unmarshal SkipBlock", data)
		return false
	}
	dst := dstN.(*SkipBlock)
	if !dst.Hash.Equal(SkipBlockID(msg)) {
		log.Lvlf2("Dest skipBlock different from msg %x %x", msg, []byte(dst.Hash))
		return false
	}

	isFree := func() bool {
		for i, b := range dst.BackLinkIDs {
			if b.Equal(src) {
				return len(srcSB.ForwardLink) == i
			}
		}
		return false
	}()
	if !isFree {
		log.Lvl2("Didn't find a free corresponding forward-link")
		return false
	}

	for _, ver := range dst.VerifierIDs {
		f, ok := s.verifiers[ver]
		if !ok {
			log.Lvlf2("Found no user verification for %x", ver)
			return false
		}
		if !f(msg, dst) {
			return false
		}
	}
	return true
}

// VerifyBase checks basic parameters between two skipblocks.
func (s *Service) verifyFuncBase(newID []byte, newSB *SkipBlock) bool {
	if !newSB.Hash.Equal(newID) {
		return false
	}
	if s.verifyBlock(newSB) != nil {
		return false
	}
	if newSB.VerifyLinks(s.Sbm) != nil {
		return false
	}
	log.Lvl4("No verification - accepted")
	return true
}

// VerifyShardFunc makes sure that the cothority of the child-skipchain is
// part of the root-cothority.
func (s *Service) verifyFuncShard(newID []byte, newSB *SkipBlock) bool {
	if newSB.ParentBlockID.IsNull() {
		log.Lvl3("No parent skipblock to verify against")
		return false
	}
	sbParent, exists := s.Sbm.GetByID(newSB.ParentBlockID)
	if !exists {
		log.Lvl3("Parent skipblock doesn't exist")
		return false
	}
	for _, e := range newSB.Roster.List {
		if i, _ := sbParent.Roster.Search(e.ID); i < 0 {
			log.Lvl3("ServerIdentity in child doesn't exist in parent")
			return false
		}
	}
	return true
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) propagateSkipBlock(msg network.Message) {
	sbs, ok := msg.(*PropagateSkipBlocks)
	if !ok {
		log.Error("Couldn't convert to slice of SkipBlocks")
		return
	}
	for _, sb := range sbs.SkipBlocks {
		if err := sb.VerifyForwardSignatures(); err != nil {
			log.Error(err)
			return
		}
		s.Sbm.Store(sb)
		log.Lvlf3("Stored skip block %+v in %x", *sb, s.Context.ServerIdentity().ID[0:8])
	}
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) registerVerification(v VerifierID, f SkipBlockVerifier) error {
	s.verifiers[v] = f
	return nil
}

// checkBlock makes sure the basic parameters of a block are correct and returns
// an error if something fails.
func (s *Service) verifyBlock(sb *SkipBlock) error {
	if sb.MaximumHeight <= 0 {
		return errors.New("Set a maximumHeight > 0")
	}
	if sb.BaseHeight <= 0 {
		return errors.New("Set a baseHeight > 0")
	}
	if sb.MaximumHeight > sb.BaseHeight {
		return errors.New("maximumHeight must be smaller or equal baseHeight")
	}
	if sb.Index < 0 {
		return errors.New("Can't have an index < 0")
	}
	if len(sb.BackLinkIDs) <= 0 {
		return errors.New("Need at least one backlinkID")
	}
	if sb.Height < 1 {
		return errors.New("Minimum height is 1")
	}
	if sb.Height > sb.MaximumHeight {
		return errors.New("Height must be <= maximumHeight")
	}
	if sb.Roster == nil {
		return errors.New("Need a roster")
	}
	return nil
}

// addForwardLink verifies if the new block is valid. If it is not valid, it
// returns with an error.
// If it finds a valid block, a forward-link will be added and a BFT-signature
// requested.
func (s *Service) addForwardLink(src, dst *SkipBlock, height int) error {
	if height <= len(src.ForwardLink) {
		return errors.New("already have forward-link at this height")
	}
	// create the message we want to sign for this round
	roster := src.Roster
	log.Lvlf3("%s is adding forward-link to %s", s.ServerIdentity(), roster.List)
	data, err := network.Marshal(dst)
	if err != nil {
		return fmt.Errorf("Couldn't marshal block: %s", err.Error())
	}
	msg := []byte(dst.Hash)
	sig, err := s.startBFT(bftNewBlock, roster, msg, append(src.Hash, data...))
	if err != nil {
		return err
	}

	fwd := &BlockLink{
		Hash:      dst.Hash,
		Signature: sig.Sig,
	}
	src.ForwardLink = append(src.ForwardLink, fwd)
	if err = src.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong BFT-signature: " + err.Error())
	}
	return nil
}

// startBFT starts a BFT-protocol with the given parameters.
func (s *Service) startBFT(proto string, roster *onet.Roster, msg, data []byte) (*bftcosi.BFTSignature, error) {
	switch len(roster.List) {
	case 0:
		return nil, errors.New("Found empty Roster")
	case 1:
		return nil, errors.New("Need more than 1 entry for Roster")
	}

	// Start the protocol
	tree := roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())
	//tree := roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())
	node, err := s.CreateProtocol(proto, tree)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create new node: %s", err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	root.Data = data

	// in testing-mode with more than one host and service per cothority-instance
	// we might have the wrong verification-function, so set it again here.
	//root.VerificationFunction = s.bftVerifyNewBlock
	// function that will be called when protocol is finished by the root
	done := make(chan bool)
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	var sig *bftcosi.BFTSignature
	select {
	case <-done:
		sig = root.Signature()
		if sig.Sig == nil {
			return nil, errors.New("Couldn't sign forward-link")
		}
		return sig, nil
	case <-time.After(time.Second * 60):
		return nil, errors.New("Timed out while waiting for signature")
	}
	return sig, nil
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*SkipBlock) error {
	log.Lvl3("Starting to propagate for service", s.ServerIdentity())
	siMap := map[string]*network.ServerIdentity{}
	// Add all rosters of all blocks - everybody needs to be contacted
	for _, block := range blocks {
		for _, si := range block.Roster.List {
			siMap[uuid.UUID(si.ID).String()] = si
		}
	}
	siList := make([]*network.ServerIdentity, 0, len(siMap))
	for _, si := range siMap {
		siList = append(siList, si)
	}
	roster := onet.NewRoster(siList)

	//log.Print(s.ServerIdentity(), "Starting")
	replies, err := s.propagate(roster, &PropagateSkipBlocks{blocks}, propagateTimeout)
	if err != nil {
		return err
	}
	//log.Print(s.ServerIdentity(), "Stopped")
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil
}

// saves all skipblocks.
func (s *Service) save() {
	log.Lvl3("Saving service")
	err := s.Save(skipblocksID, s.Sbm)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	if !s.DataAvailable(skipblocksID) {
		return nil
	}
	msg, err := s.Load(skipblocksID)
	if err != nil {
		return err
	}
	var ok bool
	s.Sbm, ok = msg.(*SkipBlockMap)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

func newSkipchainService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Sbm:              NewSkipBlockMap(),
		verifiers:        map[VerifierID]SkipBlockVerifier{},
		blockRequests:    make(map[string]chan *SkipBlock),
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	log.ErrFatal(s.RegisterHandlers(s.StoreSkipBlock, s.GetUpdateChain))
	s.RegisterProcessorFunc(network.MessageType(ForwardSignature{}),
		s.forwardSignature)
	s.RegisterProcessorFunc(network.MessageType(GetBlock{}),
		s.getBlock)
	s.RegisterProcessorFunc(network.MessageType(GetBlockReply{}),
		s.getBlockReply)

	log.ErrFatal(s.registerVerification(VerifyBase, s.verifyFuncBase))
	log.ErrFatal(s.registerVerification(VerifyShard, s.verifyFuncShard))

	var err error
	s.propagate, err = messaging.NewPropagationFunc(c, "SkipchainPropagate", s.propagateSkipBlock)
	log.ErrFatal(err)
	s.ProtocolRegister(bftNewBlock, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyNewBlock)
	})
	s.ProtocolRegister(bftFollowBlock, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyFollowBlock)
	})
	return s
}
