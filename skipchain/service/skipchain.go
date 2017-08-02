package service

import (
	"errors"
	"sync"

	"strconv"

	"time"

	"fmt"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/cothority/skipchain/libsc"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName can be used to refer to the name of this service
const bftNewBlock = "SkipchainBFTNew"
const bftFollowBlock = "SkipchainBFTFollow"

// How many msec to wait before a timeout is generated in the propagation.
const propagateTimeout = 10000

// How often we save the skipchains - in seconds.
const timeBetweenSave = 0

func init() {
	skipchainSID, _ = onet.RegisterNewService(skipchain.ServiceName, newSkipchainService)
	network.RegisterMessage(&libsc.SkipBlockBunch{})
}

// Only used in tests
var skipchainSID onet.ServiceID

// Name used to store skipblocks
const skipblocksID = "skipblocks"

// Service handles adding new SkipBlocks
type Service struct {
	*onet.ServiceProcessor
	Storage          *skipchain.SBBStorage
	propagate        messaging.PropagationFunc
	verifiers        map[libsc.VerifierID]libsc.SkipBlockVerifier
	blockRequests    map[string]chan *libsc.SkipBlock
	lastSave         time.Time
	propagating      int64
	propagatingMutex sync.Mutex
}

// StoreSkipBlock stores a new skipblock in the system. This can be either a
// genesis-skipblock, that will create a new skipchain, or a new skipblock,
// that will be added to an existing chain.
//
// The conode servicing the request needs to be part of the actual valid latest
// skipblock, else it will fail.
//
// Depending on the data given in the new SkipBlock, different actions will
// be done:
//   - GenesisID: if it is nil, a genesis-block will be created. You need
//     to set MaximumHeight, BaseHeight and the VerifierIDs
//   - Index: if 0, then it will be added to the latest skipblock, if > 0, then
//     it will be added at that position, if this is the latest position.
func (s *Service) StoreSkipBlock(psbd *skipchain.StoreSkipBlock) (*skipchain.StoreSkipBlockReply, onet.ClientError) {
	prop := psbd.NewBlock
	var prev *libsc.SkipBlock
	var changed []*libsc.SkipBlock

	if prop.GenesisID.IsNil() {
		// A new chain is created, suppose all arguments in SkipBlock
		// are correctly set up
		prop.Index = 0
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*libsc.BlockLink, 0)
		// genesis block has a random back-link:
		bl := random.Bytes(32, random.Stream)
		prop.BackLinkIDs = []libsc.SkipBlockID{libsc.SkipBlockID(bl)}
		prop.GenesisID = nil
		prop.UpdateHash()
		err := s.verifyBlock(prop)
		if err != nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorParameterWrong,
				err.Error())
		}
		changed = append(changed, prop)
	} else {
		// We're appending a block to an existing chain
		bunch := s.Storage.GetBunch(prop.GenesisID)
		if bunch == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Didn't find skipchain for given genesisid")
		}
		prev = bunch.Latest
		if prev == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Didn't find latest block")
		}
		if i, _ := prev.Roster.Search(s.ServerIdentity().ID); i < 0 {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockContent,
				"We're not responsible for latest block")
		}
		if prop.Index > 0 && prev.Index+1 != prop.Index {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockContent,
				"Chosen index is not next index - did you miss a block?")
		}
		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.VerifierIDs = prev.VerifierIDs
		prop.Index = prev.Index + 1
		index := prop.Index
		for prop.Height = 1; index%prop.BaseHeight == 0; prop.Height++ {
			index /= prop.BaseHeight
			if prop.Height >= prop.MaximumHeight {
				break
			}
		}
		log.Lvl4("Found height", prop.Height, "for index", prop.Index,
			"and maxHeight", prop.MaximumHeight, "and base", prop.BaseHeight)
		prop.BackLinkIDs = make([]libsc.SkipBlockID, prop.Height)
		pointer := prev
		for h := range prop.BackLinkIDs {
			for pointer.Height < h+1 {
				pointer = bunch.GetByID(pointer.BackLinkIDs[0])
				if pointer == nil {
					return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
						"Didn't find convenient SkipBlock for height "+
							strconv.Itoa(h))
				}
			}
			prop.BackLinkIDs[h] = pointer.Hash
		}
		prop.UpdateHash()
		if err := s.addForwardLink(prev, prop, 1); err != nil {
			log.Lvlf3("Previous is %+v", prev)
			for _, sb := range bunch.SkipBlocks {
				log.Lvlf3("%#v", sb)
			}
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockContent,
				"Couldn't get forward signature on block: "+err.Error())
		}
		changed = append(changed, prev, prop)
		for i, bl := range prop.BackLinkIDs[1:] {
			back := bunch.GetByID(bl)
			if back == nil {
				return nil, onet.NewClientErrorCode(skipchain.ErrorBlockContent,
					"Didn't get skipblock in back-link")
			}
			if prop == nil {
				log.Lvlf3(log.Stack())
			}
			if err := s.SendRaw(back.Roster.RandomServerIdentity(),
				&ForwardSignature{
					TargetHeight: i + 1,
					Previous:     prev.Hash,
					Newest:       prop,
					ForwardLink:  prev.GetForward(0),
				}); err != nil {
				// This is not a critical failure - we have at least
				// one forward-link
				log.Error("Couldn't get old block to sign")
			} else {
				changed = append(changed, back)
			}
		}
	}
	if err := s.startPropagation(changed); err != nil {
		return nil, onet.NewClientErrorCode(skipchain.ErrorVerification,
			"Couldn't propagate new blocks: "+err.Error())
	}

	reply := &skipchain.StoreSkipBlockReply{
		Previous: prev,
		Latest:   prop,
	}
	return reply, nil
}

// GetBlocks returns a slice of SkipBlocks which describe the part of the
// skipchain desired in the requested format:
//  - Start: if nil, End must be given
//  - End: if nil, get chain from Start to latest; if Start is nil, get only that
//    block
//  - MaxHeight: how fast to jump. If MaxHeight == 0, go to the maximum height
//    possible. If MaxHeight == 1, give all intermediate blocks
func (s *Service) GetBlocks(request *skipchain.GetBlocks) (*skipchain.GetBlocksReply, onet.ClientError) {
	var start, end *libsc.SkipBlock
	blocks := []*libsc.SkipBlock{}
	var bunch *libsc.SkipBlockBunch
	if !request.Start.IsNil() {
		start = s.Storage.GetByID(request.Start)
		if start == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Couldn't find starting block")
		}
		blocks = append(blocks, start.Copy())
		bunch = s.Storage.GetBunch(start.SkipChainID())
		if bunch == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Didn't find corresponding bunch for start-block")
		}
	}
	if !request.End.IsNil() {
		end = s.Storage.GetByID(request.End)
		if end == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Couldn't find ending block")
		}
		blocks = append(blocks, end.Copy())
		endBunch := s.Storage.GetBunch(end.SkipChainID())
		if endBunch == nil {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
				"Didn't find corresponding bunch for end-block")
		}
		if bunch != nil && bunch != endBunch {
			return nil, onet.NewClientErrorCode(skipchain.ErrorBlockContent,
				"Cannot get blocks between two different skipchains")
		}
		bunch = endBunch
	}
	if start != nil && end != nil && start.Index >= end.Index {
		return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
			"Order of start and end inversed")
	}
	log.Lvlf3("Starting to search chain from %v to %v", start, end)
	for start != nil && start.GetForwardLen() > 0 {
		height := start.GetForwardLen()
		if request.MaxHeight > 0 && height > request.MaxHeight {
			height = request.MaxHeight
		}
		link := start.ForwardLink[height-1]
		next := bunch.GetByID(link.Hash)
		if next == nil {
			log.Lvl3("Didn't find next block, updating block")
			var err error
			next, err = s.getUpdateBlock(start, link.Hash)
			if err != nil {
				return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
					err.Error())
			}
		} else {
			if i, _ := next.Roster.Search(s.ServerIdentity().ID); i < 0 {
				log.Lvl3("We're not responsible for", next, "- asking for update")
				var err error
				next, err = s.getUpdateBlock(next, link.Hash)
				if err != nil {
					return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
						err.Error())
				}
			}
		}
		start = next
		blocks = append(blocks, next.Copy())
		if end != nil {
			if start.Index == end.Index {
				break
			}
			if start.Index > end.Index {
				return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
					"Didn't find end-block in chain - perhaps try with maxHeight = 1")
			}
		}
	}
	log.Lvl3("Found", len(blocks), "blocks")
	reply := &skipchain.GetBlocksReply{Reply: blocks}

	return reply, nil
}

// GetAllSkipchains returns a list of all known skipchains
func (s *Service) GetAllSkipchains(id *skipchain.GetAllSkipchains) (*skipchain.GetAllSkipchainsReply, onet.ClientError) {
	// Write all known skipblocks to a map, thus removing double blocks.
	s.Storage.Lock()
	chains := make([]*libsc.SkipBlock, 0, len(s.Storage.Bunches))
	for _, sbc := range s.Storage.Bunches {
		chains = append(chains, sbc.Latest)
	}
	s.Storage.Unlock()

	return &skipchain.GetAllSkipchainsReply{
		SkipChains: chains,
	}, nil
}

// GetBlockByIndex searches for the given block and returns it. If no such block is
// found, a nil is returned.
func (s *Service) GetBlockByIndex(id *skipchain.GetBlockByIndex) (*libsc.SkipBlock, onet.ClientError) {
	sb := s.Storage.GetByID(id.Genesis)
	if sb == nil {
		return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
			"No such genesis-block")
	}
	if sb.Index == id.Index {
		return sb, nil
	}
	for len(sb.ForwardLink) > 0 {
		sb = s.Storage.GetByID(sb.ForwardLink[0].Hash)
		if sb.Index == id.Index {
			return sb, nil
		}
	}
	return nil, onet.NewClientErrorCode(skipchain.ErrorBlockNotFound,
		"No block with this index found")
}

// IsPropagating returns true if there is at least one propagation running.
func (s *Service) IsPropagating() bool {
	s.propagatingMutex.Lock()
	defer s.propagatingMutex.Unlock()
	return s.propagating > 0
}

func (s *Service) getUpdateBlock(known *libsc.SkipBlock, unknown libsc.SkipBlockID) (*libsc.SkipBlock, error) {
	s.blockRequests[string(unknown)] = make(chan *libsc.SkipBlock)
	node := known.Roster.RandomServerIdentity()
	if err := s.SendRaw(node,
		&GetBlock{unknown}); err != nil {
		return nil, errors.New("Couldn't get updated block: " + known.Short())
	}
	var block *libsc.SkipBlock
	select {
	case block = <-s.blockRequests[string(unknown)]:
		log.Lvl3("Got block", block)
		delete(s.blockRequests, string(unknown))
	case <-time.After(time.Millisecond * time.Duration(propagateTimeout)):
		delete(s.blockRequests, string(unknown))
		return nil, errors.New("Couldn't get updated block in time: " + unknown.Short())
	}
	return block, nil
}

// forwardSignature receives a signature request of a newly accepted block.
// It only needs the 2nd-newest block and the forward-link.
func (s *Service) forwardSignature(env *network.Envelope) {
	target, err := func() (target *libsc.SkipBlock, err error) {
		fs, ok := env.Msg.(*ForwardSignature)
		if !ok {
			return nil, errors.New("Didn't receive a ForwardSignature")
		}
		if fs.TargetHeight >= len(fs.Newest.BackLinkIDs) {
			return nil, errors.New("This backlink-height doesn't exist")
		}
		target = s.Storage.GetByID(fs.Newest.BackLinkIDs[fs.TargetHeight])
		if target == nil {
			return nil, errors.New("Didn't find target-block")
		}
		data, err := network.Marshal(fs)
		if err != nil {
			return
		}
		sig, err := s.startBFT(bftFollowBlock, target.Roster, fs.ForwardLink.Hash, data)
		if err != nil {
			return nil, errors.New("Couldn't get signature")
		}
		s.Storage.Mutex.Lock()
		target.AddForward(&libsc.BlockLink{
			Hash:      fs.ForwardLink.Hash,
			Signature: sig.Sig,
		})
		s.Storage.Mutex.Unlock()
		return
	}()
	if err != nil {
		log.Fatal(err)
	}
	if err = s.startPropagation([]*libsc.SkipBlock{target}); err != nil {
		log.Error(err)
	}
}

func (s *Service) getBlock(env *network.Envelope) {
	gb, ok := env.Msg.(*GetBlock)
	if !ok {
		log.Error("Didn't receive GetBlock")
		return
	}
	sb := s.Storage.GetByID(gb.ID)
	if sb == nil {
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
	bunch := s.Storage.GetBunch(gbr.SkipBlock.SkipChainID())
	if bunch == nil {
		log.Error("Don't know about this bunch")
		return
	}
	if err := bunch.VerifyLinks(gbr.SkipBlock); err != nil {
		log.Error("Received invalid skipblock: " + err.Error())
		return
	}
	id := bunch.Store(gbr.SkipBlock)
	s.save()
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
		previous := s.Storage.GetByID(fs.Previous)
		if previous == nil {
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
		target := s.Storage.GetByID(newest.BackLinkIDs[fs.TargetHeight]).Copy()
		if target == nil {
			return errors.New("Don't have target-block")
		}
		if target.GetForwardLen() >= fs.TargetHeight+1 {
			return errors.New("Already have forward-link at height " +
				strconv.Itoa(fs.TargetHeight+1))
		}
		if !target.SkipChainID().Equal(newest.SkipChainID()) {
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
	srcSB := s.Storage.GetByID(src)
	if srcSB == nil {
		log.Error("Didn't find src-skipblock")
		return false
	}
	_, dstN, err := network.Unmarshal(data[32:])
	if err != nil {
		log.Error("Couldn't unmarshal SkipBlock", data)
		return false
	}
	sb := dstN.(*libsc.SkipBlock)
	if !sb.Hash.Equal(libsc.SkipBlockID(msg)) {
		log.Lvlf2("Dest skipBlock different from msg %x %x", msg, []byte(sb.Hash))
		return false
	}

	isFree := func() bool {
		for i, b := range sb.BackLinkIDs {
			if b.Equal(src) {
				return srcSB.GetForwardLen() == i
			}
		}
		return false
	}()
	if !isFree {
		log.Lvl2("Didn't find a free corresponding forward-link")
		return false
	}

	for _, ver := range sb.VerifierIDs {
		f, ok := s.verifiers[ver]
		if !ok {
			log.Lvlf2("Found no verification-function for %x", ver)
			return false
		}
		if !f(sb) {
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
		bunch := s.Storage.GetBunch(sb.SkipChainID())
		if bunch == nil {
			s.Storage.AddBunch(sb)
		} else {
			bunch.Store(sb)
		}
	}
	s.save()
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) registerVerification(v libsc.VerifierID, f libsc.SkipBlockVerifier) error {
	s.verifiers[v] = f
	return nil
}

// checkBlock makes sure the basic parameters of a block are correct and returns
// an error if something fails.
func (s *Service) verifyBlock(sb *libsc.SkipBlock) error {
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
func (s *Service) addForwardLink(src, dst *libsc.SkipBlock, height int) error {
	if height <= src.GetForwardLen() {
		return fmt.Errorf("already have %d forward-links: height %d",
			src.GetForwardLen(), height)
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

	fwd := &libsc.BlockLink{
		Hash:      dst.Hash,
		Signature: sig.Sig,
	}
	src.AddForward(fwd)
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
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*libsc.SkipBlock) error {
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

	s.propagatingMutex.Lock()
	s.propagating++
	s.propagatingMutex.Unlock()
	replies, err := s.propagate(roster, &PropagateSkipBlocks{blocks}, propagateTimeout)
	s.propagatingMutex.Lock()
	s.propagating--
	s.propagatingMutex.Unlock()
	if err != nil {
		return err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil
}

// VerifyBase checks basic parameters between two skipblocks.
func (s *Service) verifyFuncBase(newSB *libsc.SkipBlock) bool {
	if err := s.verifyBlock(newSB); err != nil {
		log.LLvl2("verifyBlock failed:", err)
		return false
	}
	log.Lvl4("No verification - accepted")
	return true
}

// saves all skipblocks.
func (s *Service) save() {
	s.Storage.Lock()
	defer s.Storage.Unlock()
	if time.Now().Sub(s.lastSave) < time.Second*timeBetweenSave {
		return
	}
	s.lastSave = time.Now()
	log.Lvl3("Saving service")
	err := s.Save(skipblocksID, s.Storage)
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
	s.Storage, ok = msg.(*skipchain.SBBStorage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterVerification`.
type GetService interface {
	Service(name string) onet.Service
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func RegisterVerification(s GetService, v libsc.VerifierID, f libsc.SkipBlockVerifier) error {
	scs := s.Service(skipchain.ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + skipchain.ServiceName)
	}
	return scs.(*Service).registerVerification(v, f)
}

func newSkipchainService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Storage:          skipchain.NewSBBStorage(),
		verifiers:        map[libsc.VerifierID]libsc.SkipBlockVerifier{},
		blockRequests:    make(map[string]chan *libsc.SkipBlock),
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	s.lastSave = time.Now()
	log.ErrFatal(s.RegisterHandlers(s.StoreSkipBlock, s.GetBlocks,
		s.GetAllSkipchains, s.GetBlockByIndex))
	s.RegisterProcessorFunc(network.MessageType(ForwardSignature{}),
		s.forwardSignature)
	s.RegisterProcessorFunc(network.MessageType(GetBlock{}),
		s.getBlock)
	s.RegisterProcessorFunc(network.MessageType(GetBlockReply{}),
		s.getBlockReply)

	log.ErrFatal(s.registerVerification(libsc.VerifyBase, s.verifyFuncBase))

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
