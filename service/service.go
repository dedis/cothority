package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"bytes"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onchain-secrets"
	"github.com/dedis/onchain-secrets/darc"
	"github.com/dedis/onchain-secrets/protocol"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// Used for tests
var templateID onet.ServiceID

const propagationTimeout = 5000
const timestampRange = 60

func init() {
	network.RegisterMessage(Storage{})
	network.RegisterMessage(Darcs{})
	var err error
	templateID, err = onet.RegisterNewService(ocs.ServiceName, newService)
	log.ErrFatal(err)
}

// Service holds all data for the ocs-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	propagateOCS messaging.PropagationFunc

	Storage   *Storage
	saveMutex sync.Mutex
}

// pubPoly is a serializaable version of share.PubPoly
type pubPoly struct {
	B       kyber.Point
	Commits []kyber.Point
}

// Storage holds the skipblock-bunches for the OCS-skipchain.
type Storage struct {
	OCSs     *ocs.SBBStorage
	Accounts map[string]*Darcs
	Shared   map[string]*protocol.SharedSecret
	Polys    map[string]*pubPoly
	Admins   map[string]*darc.Darc
}

// Darcs holds a series of darcs in increasing, succeeding version numbers.
type Darcs struct {
	Darcs []*darc.Darc
}

// CreateSkipchains sets up a new OCS-skipchain.
func (s *Service) CreateSkipchains(req *ocs.CreateSkipchainsRequest) (reply *ocs.CreateSkipchainsReply,
	cerr onet.ClientError) {

	// Create OCS-skipchian
	c := skipchain.NewClient()
	reply = &ocs.CreateSkipchainsReply{}

	log.Lvlf2("Creating OCS-skipchain with darc %x", req.Writers.GetID())
	genesis := &ocs.Transaction{
		Darc:      &req.Writers,
		Timestamp: time.Now().Unix(),
	}
	genesisBuf, err := protobuf.Encode(genesis)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, err.Error())
	}
	reply.OCS, cerr = c.CreateGenesis(&req.Roster, 1, 1, ocs.VerificationOCS, genesisBuf, nil)
	// reply.OCS, cerr = c.CreateGenesis(&req.Roster, 1, 1, ocs.VerificationOCS, &req.Writers, nil)
	if cerr != nil {
		return nil, cerr
	}
	replies, err := s.propagateOCS(&req.Roster, reply.OCS, propagationTimeout)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	if replies != len(req.Roster.List) {
		log.Warn("Got only", replies, "replies for ocs-propagation")
	}

	// Do DKG on the nodes
	tree := req.Roster.GenerateNaryTreeWithRoot(len(req.Roster.List), s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameDKG, tree)
	setupDKG := pi.(*protocol.SetupDKG)
	setupDKG.Wait = true
	setupDKG.SetConfig(&onet.GenericConfig{Data: reply.OCS.Hash})
	//log.Lvl2(s.ServerIdentity(), reply.OCS.Hash)
	if err := pi.Start(); err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	log.Lvl3("Started DKG-protocol - waiting for done")
	select {
	case <-setupDKG.Done:
		shared, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
		}
		s.saveMutex.Lock()
		s.Storage.Shared[string(reply.OCS.Hash)] = shared
		dks, err := setupDKG.DKG.DistKeyShare()
		if err != nil {
			s.saveMutex.Unlock()
			return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
		}
		s.Storage.Polys[string(reply.OCS.Hash)] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.saveMutex.Unlock()
		reply.X = shared.X
	case <-time.After(propagationTimeout * time.Millisecond):
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol,
			"dkg didn't finish in time")
	}

	s.Storage.Admins[string(reply.OCS.Hash)] = &req.Writers
	s.save()
	return
}

// UpdateDarc adds a new account or modifies an existing one.
func (s *Service) UpdateDarc(req *ocs.UpdateDarc) (reply *ocs.UpdateDarcReply,
	cerr onet.ClientError) {
	if _, exists := s.getDarc(req.Darc.GetID()); exists {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "cannot store darc again")
	}
	latest := s.getLatestDarc(req.Darc.GetBaseID())
	if latest != nil && latest.Version >= req.Darc.Version {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "cannot store darc with lower or equal version")
	}
	if err := req.Darc.Verify(); err != nil {
		log.Lvl2("Error when checking signature:", err)
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	dataOCS := &ocs.Transaction{
		Darc:      &req.Darc,
		Timestamp: time.Now().Unix(),
	}
	s.saveMutex.Lock()
	ocsBunch := s.Storage.OCSs.GetBunch(req.OCS)
	s.saveMutex.Unlock()
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, err.Error())
	}
	sb, cerr := s.BunchAddBlock(ocsBunch, ocsBunch.Latest.Roster, data)
	if cerr != nil {
		return nil, cerr
	}
	log.Lvl3("New darc is:", req.Darc.String())
	log.Lvlf2("Added darc %x to %x:", req.Darc.GetID(), req.Darc.GetBaseID())
	log.Lvlf2("New darc is %d", req.Darc.Version)

	replies, err := s.propagateOCS(ocsBunch.Latest.Roster, sb, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
		return
	}
	if replies != len(ocsBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}

	return &ocs.UpdateDarcReply{SB: sb}, nil
}

// GetDarcPath returns the latest valid Darc given its identity.
func (s *Service) GetDarcPath(req *ocs.GetDarcPath) (reply *ocs.GetDarcPathReply,
	cerr onet.ClientError) {
	d, exists := s.getDarc(req.BaseDarcID)
	if !exists {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "this Darc doesn't exist")
	}
	if req.Identity.Ed25519 != nil {
		log.Lvlf2("Searching %d/%s, starting from %x", req.Role, req.Identity.Ed25519.Point,
			req.BaseDarcID)
	} else {
		log.Lvlf2("Searching %d/%x, starting from %x", req.Role, req.Identity.Darc.ID,
			req.BaseDarcID)
	}
	path := s.searchPath([]darc.Darc{*d}, req.Identity, darc.Role(req.Role))
	log.Lvlf3("%#v", path)
	if len(path) == 0 {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "didn't find a path to the given identity")
	}
	return &ocs.GetDarcPathReply{Path: &path}, nil
}

// GetLatestDarc searches for new darcs and returns the
// whole path for the requester to verify.
func (s *Service) GetLatestDarc(req *ocs.GetLatestDarc) (reply *ocs.GetLatestDarcReply, cerr onet.ClientError) {
	start, exists := s.getDarc(req.DarcID)
	if !exists {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "this Darc doesn't exist")
	}
	path := []*darc.Darc{start}
	darcs := s.Storage.Accounts[string(start.GetBaseID())]
	for v, d := range darcs.Darcs {
		if v > start.Version {
			path = append(path, d)
		}
	}
	return &ocs.GetLatestDarcReply{
		Darcs: &path,
	}, nil
}

// WriteRequest adds a block the OCS-skipchain with a new file.
func (s *Service) WriteRequest(req *ocs.WriteRequest) (reply *ocs.WriteReply,
	cerr onet.ClientError) {
	log.Lvl2("Write request")
	reply = &ocs.WriteReply{}
	s.saveMutex.Lock()
	ocsBunch := s.Storage.OCSs.GetBunch(req.OCS)
	s.saveMutex.Unlock()
	if ocsBunch == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "didn't find that bunch")
	}
	block := ocsBunch.GetByID(req.OCS)
	if block == nil {
		log.Error("not")
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "Didn't find block-skipchain")
	}
	dataOCS := &ocs.Transaction{
		Write:     &req.Write,
		Darc:      req.Readers,
		Timestamp: time.Now().Unix(),
	}
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, err.Error())
	}
	admin := s.Storage.Admins[string(req.OCS)]
	if admin == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "couldn't find admin for this chain")
	}
	if s.searchPath([]darc.Darc{*admin}, req.Signature.SignaturePath.Signer, darc.User) == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "writer is not in admin-darc")
	}
	if err = req.Signature.Verify(req.Write.Reader.GetID(), admin); err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "invalid signature: "+err.Error())
	}
	i := 1
	for {
		reply.SB, cerr = s.BunchAddBlock(ocsBunch, block.Roster, data)
		if cerr == nil {
			break
		}
		if cerr.ErrorCode() == skipchain.ErrorBlockInProgress {
			log.Lvl2("Waiting for block to be propagated...")
			time.Sleep(time.Duration(rand.Intn(20)*i) * time.Millisecond)
			i++
		} else {
			return nil, cerr
		}
	}

	log.Lvl2("Writing a key to the skipchain")
	if cerr != nil {
		log.Error(cerr)
		return
	}

	replies, err := s.propagateOCS(ocsBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
		return
	}
	if replies != len(ocsBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// ReadRequest asks for a read-offer on the skipchain for a reader on a file.
func (s *Service) ReadRequest(req *ocs.ReadRequest) (reply *ocs.ReadReply,
	cerr onet.ClientError) {
	log.Lvl2("Requesting a file. Reader:", req.Read.Signature.SignaturePath.Signer)
	reply = &ocs.ReadReply{}
	s.saveMutex.Lock()
	ocsBunch := s.Storage.OCSs.GetBunch(req.OCS)
	s.saveMutex.Unlock()
	if ocsBunch == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "didn't find that block")
	}
	block := ocsBunch.GetByID(req.OCS)
	if block == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "Didn't find block-skipchain")
	}
	blockData := ocs.NewOCS(ocsBunch.GetByID(req.Read.DataID).Data)
	if blockData == nil || blockData.Write == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "Not an ocs-write block")
	}
	log.Lvlf2("Document reader is %x", blockData.Write.Reader.GetID())
	if err := req.Read.Signature.SignaturePath.Verify(darc.User); err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "signature by wrong identity: "+err.Error())
	}
	if err := req.Read.Signature.Verify(req.Read.DataID, &blockData.Write.Reader); err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "wrong signature: "+err.Error())
	}
	dataOCS := &ocs.Transaction{
		Read:      &req.Read,
		Timestamp: time.Now().Unix(),
	}
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, err.Error())
	}

	i := 1
	for {
		reply.SB, cerr = s.BunchAddBlock(ocsBunch, block.Roster, data)
		if cerr == nil {
			break
		}
		if cerr.ErrorCode() == skipchain.ErrorBlockInProgress {
			log.Lvl2("Waiting for block to be propagated...")
			time.Sleep(time.Duration(rand.Intn(20)*i) * time.Millisecond)
			i++
		} else {
			return nil, cerr
		}
	}

	replies, err := s.propagateOCS(ocsBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
		return
	}
	if replies != len(ocsBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// GetReadRequests returns up to a maximum number of read-requests.
func (s *Service) GetReadRequests(req *ocs.GetReadRequests) (reply *ocs.GetReadRequestsReply, cerr onet.ClientError) {
	reply = &ocs.GetReadRequestsReply{}
	s.saveMutex.Lock()
	current := s.Storage.OCSs.GetByID(req.Start)
	s.saveMutex.Unlock()
	log.Lvlf2("Asking read-requests on writeID: %x", req.Start)

	if current == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "didn't find starting skipblock")
	}
	var doc skipchain.SkipBlockID
	if req.Count == 0 {
		dataOCS := ocs.NewOCS(current.Data)
		if dataOCS == nil || dataOCS.Write == nil {
			log.Error("Didn't find this writeID")
			return nil, onet.NewClientErrorCode(ocs.ErrorParameter,
				"id is not a writer-block")
		}
		log.Lvl2("Got first block")
		doc = current.Hash
	}
	for req.Count == 0 || len(reply.Documents) < req.Count {
		if current.Index > 0 {
			// Search next read-request
			dataOCS := ocs.NewOCS(current.Data)
			if dataOCS == nil {
				return nil, onet.NewClientErrorCode(ocs.ErrorParameter,
					"unknown block in ocs-skipchain")
			}
			if dataOCS.Read != nil {
				if req.Count > 0 || dataOCS.Read.DataID.Equal(doc) {
					doc := &ocs.ReadDoc{
						Reader: dataOCS.Read.Signature.SignaturePath.Signer,
						ReadID: current.Hash,
						DataID: dataOCS.Read.DataID,
					}
					log.Lvl2("Found read-request from", doc.Reader)
					reply.Documents = append(reply.Documents, doc)
				}
			}
		}
		if len(current.ForwardLink) > 0 {
			s.saveMutex.Lock()
			current = s.Storage.OCSs.GetFromGenesisByID(current.SkipChainID(),
				current.ForwardLink[0].Hash())
			s.saveMutex.Unlock()
		} else {
			log.Lvl3("No forward-links, stopping")
			break
		}
	}
	log.Lvlf2("WriteID %x: found %d out of a maximum of %d documents", req.Start, len(reply.Documents), req.Count)
	return
}

// SharedPublic returns the shared public key of a skipchain.
func (s *Service) SharedPublic(req *ocs.SharedPublicRequest) (reply *ocs.SharedPublicReply, error onet.ClientError) {
	log.Lvl2("Getting shared public key")
	s.saveMutex.Lock()
	shared, ok := s.Storage.Shared[string(req.Genesis)]
	s.saveMutex.Unlock()
	if !ok {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "didn't find this skipchain")
	}
	return &ocs.SharedPublicReply{X: shared.X}, nil
}

// DecryptKeyRequest re-encrypts the stored symmetric key under the public
// key of the read-request. Once the read-request is on the skipchain, it is
// not necessary to check its validity again.
func (s *Service) DecryptKeyRequest(req *ocs.DecryptKeyRequest) (reply *ocs.DecryptKeyReply,
	cerr onet.ClientError) {
	reply = &ocs.DecryptKeyReply{}
	log.Lvl2("Re-encrypt the key to the public key of the reader")

	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	readSB := s.Storage.OCSs.GetByID(req.Read)
	read := ocs.NewOCS(readSB.Data)
	if read == nil || read.Read == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "This is not a read-block")
	}
	fileSB := s.Storage.OCSs.GetByID(read.Read.DataID)
	file := ocs.NewOCS(fileSB.Data)
	if file == nil || file.Write == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorParameter, "Data-block is broken")
	}

	// Start OCS-protocol to re-encrypt the file's symmetric key under the
	// reader's public key.
	nodes := len(fileSB.Roster.List)
	tree := fileSB.Roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameOCS, tree)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	ocsProto := pi.(*protocol.OCS)
	ocsProto.U = file.Write.U
	ocsProto.Xc = read.Read.Signature.SignaturePath.Signer.Ed25519.Point
	log.Lvlf2("Public key is: %s", ocsProto.Xc)
	ocsProto.Shared = s.Storage.Shared[string(fileSB.GenesisID)]
	pp := s.Storage.Polys[string(fileSB.GenesisID)]
	ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B, pp.Commits)
	ocsProto.SetConfig(&onet.GenericConfig{Data: fileSB.GenesisID})
	err = ocsProto.Start()
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	log.Lvl3("Waiting for end of ocs-protocol")
	<-ocsProto.Done
	reply.XhatEnc, err = share.RecoverCommit(cothority.Suite, ocsProto.Uis,
		nodes-1, nodes)
	if err != nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol, err.Error())
	}
	reply.Cs = file.Write.Cs
	reply.X = s.Storage.Shared[string(fileSB.GenesisID)].X
	return
}

// GetBunches returns all defined bunches in this conode.
func (s *Service) GetBunches(req *ocs.GetBunchRequest) (reply *ocs.GetBunchReply, cerr onet.ClientError) {
	log.Lvl2("Getting all bunches")
	reply = &ocs.GetBunchReply{}
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	for _, b := range s.Storage.OCSs.Bunches {
		reply.Bunches = append(reply.Bunches, b.GetByID(b.GenesisID))
	}
	return reply, nil
}

// BunchAddBlock adds a block to the latest block from the bunch. If the block
// doesn't have a roster set, it will be copied from the last block.
func (s *Service) BunchAddBlock(bunch *ocs.SkipBlockBunch, r *onet.Roster, data interface{}) (*skipchain.SkipBlock, onet.ClientError) {
	log.Lvl2("Adding block to bunch")
	s.saveMutex.Lock()
	latest := bunch.Latest.Copy()
	s.saveMutex.Unlock()
	reply, err := skipchain.NewClient().StoreSkipBlock(latest, r, data)
	if err != nil {
		return nil, err
	}
	sbNew := reply.Latest
	s.saveMutex.Lock()
	id := bunch.Store(sbNew)
	s.saveMutex.Unlock()
	if id == nil {
		return nil, onet.NewClientErrorCode(ocs.ErrorProtocol,
			"Couldn't add block to bunch")
	}
	return sbNew, nil
}

// NewProtocol intercepts the DKG and OCS protocols to retrieve the values
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	//log.Lvl2(s.ServerIdentity(), tn.ProtocolName(), conf)
	switch tn.ProtocolName() {
	case protocol.NameDKG:
		pi, err := protocol.NewSetupDKG(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*protocol.SetupDKG)
		go func(conf *onet.GenericConfig) {
			<-setupDKG.Done
			shared, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			log.Lvl3(s.ServerIdentity(), "Got shared", shared)
			//log.Lvl2(conf)
			s.saveMutex.Lock()
			s.Storage.Shared[string(conf.Data)] = shared
			s.saveMutex.Unlock()
		}(conf)
		return pi, nil
	case protocol.NameOCS:
		s.saveMutex.Lock()
		shared, ok := s.Storage.Shared[string(conf.Data)]
		s.saveMutex.Unlock()
		if !ok {
			return nil, errors.New("didn't find skipchain")
		}
		pi, err := protocol.NewOCS(tn)
		if err != nil {
			return nil, err
		}
		ocs := pi.(*protocol.OCS)
		ocs.Shared = shared
		return ocs, nil
	}
	return nil, nil
}

func (s *Service) addDarc(d *darc.Darc) {
	key := string(d.GetBaseID())
	darcs := s.Storage.Accounts[key]
	if darcs == nil {
		darcs = &Darcs{}
	}
	darcs.Darcs = append(darcs.Darcs, d)
	s.Storage.Accounts[key] = darcs
}

func (s *Service) getDarc(id darc.ID) (*darc.Darc, bool) {
	for _, darcs := range s.Storage.Accounts {
		for _, d := range darcs.Darcs {
			if d.GetID().Equal(id) {
				return d, true
			}
		}
	}
	return nil, false
}

func (s *Service) getLatestDarc(genesisID darc.ID) *darc.Darc {
	darcs := s.Storage.Accounts[string(genesisID)]
	if darcs == nil || len(darcs.Darcs) == 0 {
		return nil
	}
	return darcs.Darcs[len(darcs.Darcs)-1]
}

// printPath is a debugging function to print the
// path of darcs.
func (s *Service) printPath(path []darc.Darc) {
	for i, d := range path {
		log.Lvlf1("path[%d] => %s", i, d.String())
	}
}

// searchPath does a breadth-first search of a path going from the last element
// of path to the identity. It starts by first getting the latest darc-version,
// then searching all sub-darcs.
// If it doesn't find a matching path, it returns nil.
func (s *Service) searchPath(path []darc.Darc, identity darc.Identity, role darc.Role) []darc.Darc {
	newpath := make([]darc.Darc, len(path))
	copy(newpath, path)

	// Any role deeper in the tree must be a user role.
	if role == darc.Owner && len(path) > 1 {
		role = darc.User
	}
	d := &path[len(path)-1]

	// First get latest version
	for _, di := range s.Storage.Accounts[string(d.GetBaseID())].Darcs {
		if di.Version > d.Version {
			log.Lvl3("Adding new version", di.Version)
			newpath = append(newpath, *di)
			d = di
		}
	}
	log.Lvl3("role is:", role)
	for i, p := range newpath {
		log.Lvlf3("newpath[%d] = %x", i, p.GetID())
	}
	log.Lvl3("This darc is:", newpath[len(newpath)-1].String())

	// Then search for identity
	ids := d.Users
	if role == darc.Owner {
		ids = d.Owners
	}
	if ids != nil {
		// First search the identity
		for _, id := range *ids {
			if identity.Ed25519 != nil {
				if id.Ed25519 != nil {
					if id.Ed25519.Point.Equal(identity.Ed25519.Point) {
						return newpath
					}
				}
			} else if identity.Darc != nil {
				if id.Darc != nil {
					if id.Darc.ID.Equal(identity.Darc.ID) {
						return newpath
					}
				}
			}
		}
		// Then search sub-darcs
		for _, id := range *ids {
			if id.Darc != nil {
				d, found := s.getDarc(id.Darc.ID)
				if !found {
					log.Lvlf1("Got unknown darc-id in path - ignoring: %x", id.Darc.ID)
					continue
				}
				if np := s.searchPath(append(newpath, *d), identity, role); np != nil {
					return np
				}
			}
		}
	}
	return nil
}

// darcRecursive searches for all darcs given an id. It makes sure to avoid
// recursive endless loops by verifying that all new calls are done with
// not-yet-existing IDs.
func (s *Service) darcRecursive(storage map[string]*darc.Darc, search darc.ID) {
	// darc := s.Storage.Accounts[string(search)]
	// storage[string(search)] = darc
	// log.Lvlf2("%+v", darc)
	// for _, d := range darc.Accounts {
	// 	if _, exists := storage[string(d.ID)]; !exists {
	// 		s.darcRecursive(storage, d.ID)
	// 	}
	// }
}

func (s *Service) verifyOCS(newID []byte, sb *skipchain.SkipBlock) bool {
	log.Lvl3(s.ServerIdentity(), "Verifying ocs")
	dataOCS := ocs.NewOCS(sb.Data)
	if dataOCS == nil {
		log.Lvl3("Didn't find ocs")
		return false
	}
	s.saveMutex.Lock()
	genesis := s.Storage.OCSs.GetFromGenesisByID(sb.SkipChainID(), sb.SkipChainID())
	s.saveMutex.Unlock()
	if genesis == nil {
		log.Lvl3("No genesis-block")
		return false
	}
	ocsData := ocs.NewOCS(genesis.Data)
	if ocsData == nil {
		log.Lvl3("No ocs-data in genesis-block")
		return false
	}

	unixNow := time.Now().Unix()
	unixDifference := unixNow - dataOCS.Timestamp
	if unixDifference < 0 {
		unixDifference = -unixDifference
	}
	if unixDifference > timestampRange {
		log.Lvl3("Difference in time is too high - now: %v, timestamp: %v",
			unixNow, dataOCS.Timestamp)
		return false
	}

	if write := dataOCS.Write; write != nil {
		// Write has to check if the signature comes from a valid writer.
		log.Lvl2("Checking the proof of the writer knowing r.")
		return true
	} else if read := dataOCS.Read; read != nil {
		// Read has to check that it's a valid reader
		log.Lvl2("It's a read")
		// Search file
		var writeBlock *ocs.Write
		var readersBlock *darc.Darc
		for _, sb := range s.Storage.OCSs.GetBunch(genesis.Hash).SkipBlocks {
			wd := ocs.NewOCS(sb.Data)
			if wd != nil && wd.Write != nil {
				if bytes.Compare(sb.Hash, read.DataID) == 0 {
					writeBlock = wd.Write
					readersBlock = wd.Darc
					break
				}
			}
		}
		if writeBlock == nil {
			log.Lvl2("Didn't find file")
			return false
		}
		if readersBlock == nil {
			log.Error("Found empty readers-block")
			return false
		}
		if err := readersBlock.Verify(); err != nil {
			log.Error("wrong reader verification:" + err.Error())
			return false
		}
		return true
		//return false
	} else if darc := dataOCS.Darc; darc != nil {
		log.Lvl2("Accepting all darc side")
		return true
	}
	return false
}

func (s *Service) propagateOCSFunc(sbI network.Message) {
	sb, ok := sbI.(*skipchain.SkipBlock)
	if !ok {
		log.Error("got something else than a skipblock")
		return
	}
	dataOCS := ocs.NewOCS(sb.Data)
	if dataOCS == nil {
		log.Error("Got a skipblock without dataOCS - not storing")
		return
	}
	s.saveMutex.Lock()
	s.Storage.OCSs.Store(sb)
	if r := dataOCS.Darc; r != nil {
		log.Lvlf2("Storing new darc %x - %x", r.GetID(), r.GetBaseID())
		s.addDarc(r)
	}
	s.saveMutex.Unlock()
	s.save()
	if sb.Index == 0 {
		return
	}
	c := skipchain.NewClient()
	for _, sbID := range sb.BackLinkIDs {
		sbNew, cerr := c.GetSingleBlock(sb.Roster, sbID)
		if cerr != nil {
			log.Error(cerr)
		} else {
			s.saveMutex.Lock()
			s.Storage.OCSs.Store(sbNew)
			s.saveMutex.Unlock()
		}
	}
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3(s.String(), "Saving service")
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	s.Storage.OCSs.Lock()
	defer s.Storage.OCSs.Unlock()
	for _, b := range s.Storage.OCSs.Bunches {
		b.Lock()
	}
	err := s.Save("storage", s.Storage)
	for _, b := range s.Storage.OCSs.Bunches {
		b.Unlock()
	}
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	defer func() {
		if len(s.Storage.Shared) == 0 {
			s.Storage.Shared = map[string]*protocol.SharedSecret{}
		}
		if len(s.Storage.Polys) == 0 {
			s.Storage.Polys = map[string]*pubPoly{}
		}
		if len(s.Storage.Accounts) == 0 {
			s.Storage.Accounts = map[string]*Darcs{}
		}
		if len(s.Storage.Admins) == 0 {
			s.Storage.Admins = map[string]*darc.Darc{}
		}
	}()
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.Storage, ok = msg.(*Storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	log.Lvl2("Successfully loaded:", len(s.Storage.Accounts))
	return nil
}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the configuration on our own from time to time.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Storage: &Storage{
			OCSs:   ocs.NewSBBStorage(),
			Admins: make(map[string]*darc.Darc),
		},
	}
	if err := s.RegisterHandlers(s.CreateSkipchains,
		s.WriteRequest, s.ReadRequest, s.GetReadRequests,
		s.DecryptKeyRequest, s.SharedPublic,
		s.GetBunches, s.UpdateDarc, s.GetDarcPath,
		s.GetLatestDarc); err != nil {
		log.Error("Couldn't register messages", err)
		return nil, err
	}
	skipchain.RegisterVerification(c, ocs.VerifyOCS, s.verifyOCS)
	var err error
	s.propagateOCS, err = messaging.NewPropagationFunc(c, "PropagateOCS", s.propagateOCSFunc, -1)
	log.ErrFatal(err)
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
