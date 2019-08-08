package byzcoin

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/cothority/v3/byzcoin/viewchange"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

type viewChangeManager struct {
	sync.Mutex
	controllers map[string]*viewchange.Controller
}

func newViewChangeManager() viewChangeManager {
	return viewChangeManager{
		controllers: make(map[string]*viewchange.Controller),
	}
}

// adds a new controller to the map. This method should always be followed
// by `start`, else `started` will not work.
func (m *viewChangeManager) add(SendInitReq viewchange.SendInitReqFunc,
	sendNewView viewchange.SendNewViewReqFunc, isLeader viewchange.IsLeaderFunc, k string) {
	m.Lock()
	defer m.Unlock()
	c := viewchange.NewController(SendInitReq, sendNewView, isLeader)
	m.controllers[k] = &c
}

// actually starts the viewchange monitor. This should always be called after
// `add`, else `started` will not work
func (m *viewChangeManager) start(myID network.ServerIdentityID, scID skipchain.SkipBlockID, initialDuration time.Duration, f int) {
	k := string(scID)
	m.Lock()
	defer m.Unlock()
	c, ok := m.controllers[k]
	if !ok {
		panic("never start without add first: " + log.Stack())
	}
	go c.Start(myID, scID, initialDuration, f)
}

// started returns true if the monitor is started. This supposes that `start`
// has been called after `add`.
func (m *viewChangeManager) started(scID skipchain.SkipBlockID) bool {
	m.Lock()
	defer m.Unlock()
	_, s := m.controllers[string(scID)]
	return s
}

func (m *viewChangeManager) stop(scID skipchain.SkipBlockID) {
	k := string(scID)
	m.Lock()
	defer m.Unlock()
	c, ok := m.controllers[k]
	if !ok {
		return
	}
	c.Stop()
	delete(m.controllers, k)
}

func (m *viewChangeManager) addReq(req viewchange.InitReq) {
	m.Lock()
	defer m.Unlock()
	c := m.controllers[string(req.View.Gen)]
	// In theory c will never be nil, but if addReq happens after closeAll...
	if c != nil {
		c.AddReq(req)
	}
}

func (m *viewChangeManager) done(view viewchange.View) {
	m.Lock()
	defer m.Unlock()
	c := m.controllers[string(view.Gen)]
	if c != nil {
		c.Done(view)
	}
	log.Lvl3("view-change done for " + view.String())
}

func (m *viewChangeManager) waiting(k string) bool {
	m.Lock()
	defer m.Unlock()
	c, ok := m.controllers[k]
	if !ok {
		return false
	}
	return c.Waiting()
}

func (m *viewChangeManager) closeAll() {
	m.Lock()
	defer m.Unlock()
	for _, c := range m.controllers {
		c.Stop()
	}
	m.controllers = make(map[string]*viewchange.Controller)
}

// sendViewChangeReq is called when the node detects that a view change is
// needed. It uses SendRaw to send the message to all other nodes. This
// function should only be used as a callback in viewchange.Controller.
func (s *Service) sendViewChangeReq(view viewchange.View) error {
	if view.LeaderIndex < 0 {
		return errors.New("leader index must be positive")
	}

	log.Lvl2(s.ServerIdentity(), "sending view-change request for view:", view)
	latest, err := s.db().GetLatestByID(view.ID)
	if err != nil {
		return err
	}
	log.Lvlf2("%s: current leader: %s - asking to elect leader: %s", s.ServerIdentity(), latest.Roster.List[0],
		latest.Roster.List[view.LeaderIndex%len(latest.Roster.List)])
	req := viewchange.InitReq{
		SignerID: s.ServerIdentity().ID,
		View:     view,
	}
	if err := req.Sign(s.getPrivateKey()); err != nil {
		return err
	}
	for _, sid := range latest.Roster.List {
		if sid.Equal(s.ServerIdentity()) {
			continue
		}
		go func(id *network.ServerIdentity) {
			if err := s.SendRaw(id, &req); err != nil {
				// Having an error here is fine because not all the
				// nodes are guaranteed to be online. So we log a
				// warning instead of returning an error.
				log.Warn(s.ServerIdentity(), "Couldn't send view-change request to", id.Address, err)
			}
		}(sid)
	}
	return nil
}

func (s *Service) sendNewView(proof []viewchange.InitReq) {

	if len(proof) == 0 {
		log.Error(s.ServerIdentity(), "no proofs")
	}
	log.Lvl2(s.ServerIdentity(), "sending new-view request for view:", proof[0].View)

	// Our own proof might not be signed, so sign it.
	for i := range proof {
		if proof[i].SignerID.Equal(s.ServerIdentity().ID) && len(proof[i].Signature) == 0 {
			proof[i].Sign(s.getPrivateKey())
		}
	}

	sb := s.db().GetByID(proof[0].View.ID)
	req := viewchange.NewViewReq{
		Roster: *rotateRoster(sb.Roster, proof[0].View.LeaderIndex),
		Proof:  proof,
	}

	go func() {
		s.working.Add(1)
		defer s.working.Done()
		// This go-routine eventually exists because both cosi and
		// block creation have a timeout.
		sig, err := s.startViewChangeCosi(req)
		if err != nil {
			log.Error(s.ServerIdentity(), "Error while starting view-change:", err)
			return
		}
		if len(sig) == 0 {
			log.Error(s.ServerIdentity(), "empty viewchange cosi signature")
			return
		}
		if err := s.createViewChangeBlock(req, sig); err != nil {
			log.Error(s.ServerIdentity(), err)
		}
	}()
}

func (s *Service) computeInitialDuration(scID skipchain.SkipBlockID) (time.Duration, error) {
	interval, _, err := s.LoadBlockInfo(scID)
	if err != nil {
		return 0, err
	}
	return s.rotationWindow * interval, nil
}

func (s *Service) getFaultThreshold(sbID skipchain.SkipBlockID) int {
	sb := s.db().GetByID(sbID)
	return (len(sb.Roster.List) - 1) / 3
}

// handleViewChangeReq should be registered as a handler for viewchange.InitReq
// messages.
func (s *Service) handleViewChangeReq(env *network.Envelope) error {
	// Parse message.
	req, ok := env.Msg.(*viewchange.InitReq)
	if !ok {
		return fmt.Errorf("%v failed to cast to viewchange.ViewChangeReq", s.ServerIdentity())
	}
	// Should not be sending to ourself.
	if req.SignerID.Equal(s.ServerIdentity().ID) {
		return fmt.Errorf("%v should not send to ourself", s.ServerIdentity())
	}

	// Check that the genesis exists and the view is valid.
	if gen := s.db().GetByID(req.View.Gen); gen == nil || gen.Index != 0 {
		return fmt.Errorf("%v cannot find the genesis block in request", s.ServerIdentity())
	}
	reqLatest := s.db().GetByID(req.View.ID)
	if reqLatest == nil {
		// NOTE: If we don't know about the this view, it might be that
		// we are not up-do-date, which should not happen because the
		// delay for triggering view-change should be longer than the
		// time it takes to create and propagate a new block. Hence,
		// somebody is sending bogus views.
		return fmt.Errorf("%v we do not know this view", s.ServerIdentity())
	}
	if len(reqLatest.ForwardLink) != 0 {
		// This is because the node is out-of-sync with others. If the current leader happens
		// to be offline, it won't catch up because it doesn't get the requests to collect
		// transactions so we need to trigger a catch up here to the distant peer.

		// The simplest solution is to pro-actively send it the current state of the chain
		// and the distant peer will detect the Trie index difference.
		ro := onet.NewRoster([]*network.ServerIdentity{env.ServerIdentity})
		err := s.skService().PropagateProof(ro, req.View.Gen)
		if err != nil {
			log.Errorf("View change failed to propagate a proof: %s", err.Error())
		}

		return fmt.Errorf("%v view-change should not happen for blocks that are not the latest", s.ServerIdentity())
	}

	// Check signature.
	_, signerSID := reqLatest.Roster.Search(req.SignerID)
	if signerSID == nil {
		return fmt.Errorf("%v signer does not exist", s.ServerIdentity())
	}
	if err := schnorr.Verify(cothority.Suite, signerSID.Public, req.Hash(), req.Signature); err != nil {
		return fmt.Errorf("%v %v", s.ServerIdentity(), err)
	}

	// Store it in our log.
	s.viewChangeMan.addReq(*req)
	return nil
}

func (s *Service) startViewChangeCosi(req viewchange.NewViewReq) ([]byte, error) {
	defer log.Lvl2(s.ServerIdentity(), "finished view-change blscosi")
	sb := s.db().GetByID(req.GetView().ID)
	newRoster := rotateRoster(sb.Roster, req.GetView().LeaderIndex)
	if !newRoster.List[0].Equal(s.ServerIdentity()) {
		return nil, errors.New("startViewChangeCosi should not be called by non-leader")
	}
	proto, err := s.CreateProtocol(viewChangeFtCosi, newRoster.GenerateBinaryTree())
	if err != nil {
		return nil, err
	}
	payload, err := protobuf.Encode(&req)
	if err != nil {
		return nil, err
	}

	interval, _, err := s.LoadBlockInfo(req.GetView().ID)
	if err != nil {
		return nil, err
	}

	cosiProto := proto.(*protocol.BlsCosi)
	cosiProto.Msg = req.Hash()
	cosiProto.Data = payload
	cosiProto.CreateProtocol = s.CreateProtocol
	cosiProto.Timeout = interval * 2

	if err := cosiProto.Start(); err != nil {
		return nil, err
	}
	// The protocol should always send FinalSignature because it has a
	// timeout, so we don't need a select.
	return <-cosiProto.FinalSignature, nil
}

// verifyViewChange is registered in the view-change ftcosi.
func (s *Service) verifyViewChange(msg []byte, data []byte) bool {
	// Parse message and check hash.
	var req viewchange.NewViewReq
	if err := protobuf.Decode(data, &req); err != nil {
		log.Error(s.ServerIdentity(), err)
		return false
	}
	if !bytes.Equal(msg, req.Hash()) {
		log.Error(s.ServerIdentity(), "digest doesn't verify")
		return false
	}
	// Check that we know about the view and the new roster in the request
	// matches the view-change proofs.
	sb := s.db().GetByID(req.GetView().ID)
	if sb == nil {
		log.Error(s.ServerIdentity(), "view does not exist")
		return false
	}
	newRosterID := rotateRoster(sb.Roster, req.GetView().LeaderIndex).ID
	if !newRosterID.Equal(req.Roster.ID) {
		log.Error(s.ServerIdentity(), "invalid roster in request")
		return false
	}
	// Check the signers are unique, they are in the roster and the
	// signatures are correct.
	uniqueSigners, uniqueViews := func() (int, int) {
		signers := make(map[[16]byte]bool)
		views := make(map[string]bool)
		for _, p := range req.Proof {
			signers[p.SignerID] = true
			views[string(p.View.Hash())] = true
		}
		return len(signers), len(views)
	}()
	f := s.getFaultThreshold(sb.Hash)
	if uniqueSigners <= 2*f {
		log.Error(s.ServerIdentity(), "not enough proofs: %v <= %v", uniqueSigners, 2*f)
		return false
	}
	if uniqueViews != 1 {
		log.Error(s.ServerIdentity(), "conflicting views")
		return false
	}
	// Put the roster in a map so that it's more efficient to search.
	rosterMap := make(map[network.ServerIdentityID]*network.ServerIdentity)
	for _, sid := range sb.Roster.List {
		rosterMap[sid.ID] = sid
	}
	for _, p := range req.Proof {
		sid, ok := rosterMap[p.SignerID]
		if !ok {
			log.Error(s.ServerIdentity(), "the signer is not in the roster")
			return false
		}
		// Check that the signature is correct.
		if err := schnorr.Verify(cothority.Suite, sid.Public, p.Hash(), p.Signature); err != nil {
			log.Error(s.ServerIdentity(), err)
			return false
		}
	}
	log.Lvl2(s.ServerIdentity(), "view-change verification OK")
	return true
}

// createViewChangeBlock creates a new block to record the successful
// view-change operation.
func (s *Service) createViewChangeBlock(req viewchange.NewViewReq, multisig []byte) error {
	defer log.Lvl2(s.ServerIdentity(), "created view-change block")
	sb, err := s.db().GetLatestByID(req.GetGen())
	if err != nil {
		return err
	}
	if len(sb.Roster.List) < 4 {
		return errors.New("roster size is too small, must be >= 4")
	}

	reqBuf, err := protobuf.Encode(&req)
	if err != nil {
		return err
	}

	signer := darc.NewSignerEd25519(s.ServerIdentity().Public, s.getPrivateKey())

	st, err := s.GetReadOnlyStateTrie(sb.SkipChainID())
	if err != nil {
		return err
	}
	ctr, err := getSignerCounter(st, signer.Identity().String())
	if err != nil {
		return err
	}

	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: NewInstanceID(nil),
			Invoke: &Invoke{
				ContractID: ContractConfigID,
				Command:    "view_change",
				Args: []Argument{
					{
						Name:  "newview",
						Value: reqBuf,
					},
					{
						Name:  "multisig",
						Value: multisig,
					},
				},
			},
			SignerIdentities: []darc.Identity{signer.Identity()},
			SignerCounter:    []uint64{ctr + 1},
		}},
	}

	header, err := decodeBlockHeader(sb)
	if err != nil {
		return err
	}

	ctx.Instructions.SetVersion(header.Version)

	if err = ctx.Instructions[0].SignWith(ctx.Instructions.Hash(), signer); err != nil {
		return err
	}

	_, err = s.createNewBlock(req.GetGen(), rotateRoster(sb.Roster, req.GetView().LeaderIndex), []TxResult{TxResult{ctx, false}})
	return err
}

// getPrivateKey returns the default private key of the server
// that is used to sign schnorr signatures for the view change
// protocol
func (s *Service) getPrivateKey() kyber.Scalar {
	return s.ServerIdentity().GetPrivate()
}

func rotateRoster(roster *onet.Roster, i int) *onet.Roster {
	// handle catastrophic situations where a round of the roster
	// is not enough to find a new leader
	i = i % len(roster.List)

	return onet.NewRoster(append(roster.List[i:], roster.List[:i]...))
}
