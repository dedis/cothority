package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
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

const propagationTimeout = 5 * time.Second
const timestampRange = 60

func init() {
	network.RegisterMessages(Storage{}, Darcs{}, vData{})
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

	skipchain *skipchain.Service
	// saveMutex protects access to the storage field.
	saveMutex sync.Mutex
	Storage   *Storage
}

// pubPoly is a serializaable version of share.PubPoly
type pubPoly struct {
	B       kyber.Point
	Commits []kyber.Point
}

// Storage holds the skipblock-bunches for the OCS-skipchain.
type Storage struct {
	Accounts map[string]*Darcs
	Shared   map[string]*protocol.SharedSecret
	Polys    map[string]*pubPoly
	Admins   map[string]*darc.Darc
}

// Darcs holds a series of darcs in increasing, succeeding version numbers.
type Darcs struct {
	Darcs []*darc.Darc
}

// vData is sent to all nodes when re-encryption takes place. If Ephemeral
// is non-nil, Signature needs to hold a valid signature from the reader
// in the SB-block.
type vData struct {
	SB        skipchain.SkipBlockID
	Ephemeral kyber.Point
	Signature *darc.Signature
}

// CreateSkipchains sets up a new OCS-skipchain.
func (s *Service) CreateSkipchains(req *ocs.CreateSkipchainsRequest) (reply *ocs.CreateSkipchainsReply,
	err error) {

	// Create OCS-skipchian
	reply = &ocs.CreateSkipchainsReply{}

	log.Lvlf2("Creating OCS-skipchain with darc %x", req.Writers.GetID())
	genesis := &ocs.Transaction{
		Darc:      &req.Writers,
		Timestamp: time.Now().Unix(),
	}
	genesisBuf, err := protobuf.Encode(genesis)
	if err != nil {
		return nil, err
	}
	block := skipchain.NewSkipBlock()
	block.Roster = &req.Roster
	block.BaseHeight = 1
	block.MaximumHeight = 1
	block.VerifierIDs = ocs.VerificationOCS
	block.Data = genesisBuf
	// This is for creating the skipchain, so we cannot use s.storeSkipBlock.
	replySSB, err := s.skipchain.StoreSkipBlock(&skipchain.StoreSkipBlock{
		NewBlock: block,
	})
	if err != nil {
		return nil, err
	}
	reply.OCS = replySSB.Latest
	replies, err := s.propagateOCS(&req.Roster, reply.OCS, propagationTimeout)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	log.Lvl3("Started DKG-protocol - waiting for done")
	select {
	case <-setupDKG.Done:
		shared, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, err
		}
		s.saveMutex.Lock()
		s.Storage.Shared[string(reply.OCS.Hash)] = shared
		dks, err := setupDKG.DKG.DistKeyShare()
		if err != nil {
			s.saveMutex.Unlock()
			return nil, err
		}
		s.Storage.Polys[string(reply.OCS.Hash)] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.saveMutex.Unlock()
		reply.X = shared.X
	case <-time.After(propagationTimeout):
		return nil, errors.New("dkg didn't finish in time")
	}

	s.Storage.Admins[string(reply.OCS.Hash)] = &req.Writers
	s.save()
	return
}

// UpdateDarc adds a new account or modifies an existing one.
func (s *Service) UpdateDarc(req *ocs.UpdateDarc) (reply *ocs.UpdateDarcReply,
	err error) {
	if _, exists := s.getDarc(req.Darc.GetID()); exists {
		return nil, errors.New("cannot store darc again")
	}
	latest := s.getLatestDarc(req.Darc.GetBaseID())
	if latest != nil && latest.Version >= req.Darc.Version {
		return nil, errors.New("cannot store darc with lower or equal version")
	}
	if err := req.Darc.Verify(); err != nil {
		log.Lvl2("Error when checking signature:", err)
		return nil, err
	}
	dataOCS := &ocs.Transaction{
		Darc:      &req.Darc,
		Timestamp: time.Now().Unix(),
	}
	latestSB, err := s.db().GetLatest(s.db().GetByID(req.OCS))
	if err != nil {
		return nil, errors.New("couldn't find latest block: " + err.Error())
	}
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, err
	}
	latestSB, err = s.storeSkipBlock(latestSB, data)
	if err != nil {
		return nil, err
	}
	log.Lvl3("New darc is:", req.Darc.String())
	log.Lvlf2("Added darc %x to %x:", req.Darc.GetID(), req.Darc.GetBaseID())
	log.Lvlf2("New darc is %d", req.Darc.Version)

	replies, err := s.propagateOCS(latestSB.Roster, latestSB, propagationTimeout)
	if err != nil {
		return
	}
	if replies != len(latestSB.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}

	return &ocs.UpdateDarcReply{SB: latestSB}, nil
}

// GetDarcPath returns the latest valid Darc given its identity.
func (s *Service) GetDarcPath(req *ocs.GetDarcPath) (reply *ocs.GetDarcPathReply,
	err error) {
	d, exists := s.getDarc(req.BaseDarcID)
	if !exists {
		return nil, errors.New("this Darc doesn't exist")
	}
	log.Lvlf2("Searching %d/%s, starting from %x", req.Role, req.Identity.String(),
		req.BaseDarcID)
	path := s.searchPath([]darc.Darc{*d}, req.Identity, darc.Role(req.Role))
	log.Lvlf3("%#v", path)
	if len(path) == 0 {
		return nil, errors.New("didn't find a path to the given identity")
	}
	return &ocs.GetDarcPathReply{Path: &path}, nil
}

// GetLatestDarc searches for new darcs and returns the
// whole path for the requester to verify.
func (s *Service) GetLatestDarc(req *ocs.GetLatestDarc) (reply *ocs.GetLatestDarcReply, err error) {
	start, exists := s.getDarc(req.DarcID)
	if !exists {
		return nil, errors.New("this Darc doesn't exist")
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
	err error) {
	log.Lvl2("Write request")
	reply = &ocs.WriteReply{}
	latestSB, err := s.db().GetLatest(s.db().GetByID(req.OCS))
	if err != nil {
		return nil, errors.New("didn't find latest block: " + err.Error())
	}
	dataOCS := &ocs.Transaction{
		Write:     &req.Write,
		Darc:      req.Readers,
		Timestamp: time.Now().Unix(),
	}
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, err
	}
	admin := s.Storage.Admins[string(req.OCS)]
	if admin == nil {
		return nil, errors.New("couldn't find admin for this chain")
	}
	if s.searchPath([]darc.Darc{*admin}, req.Signature.SignaturePath.Signer, darc.User) == nil {
		return nil, errors.New("writer is not in admin-darc")
	}
	if err = req.Signature.Verify(req.Write.Reader.GetID(), admin); err != nil {
		return nil, errors.New("invalid signature: " + err.Error())
	}
	i := 1
	for {
		reply.SB, err = s.storeSkipBlock(latestSB, data)
		if err == nil {
			break
		}
		if err == skipchain.ErrorProcessing {
			log.Lvl2("Waiting for block to be propagated...")
			time.Sleep(time.Duration(rand.Intn(20)*i) * time.Millisecond)
			i++
		} else {
			return nil, err
		}
	}

	log.Lvl2("Writing a key to the skipchain")
	if err != nil {
		log.Error(err)
		return
	}

	replies, err := s.propagateOCS(reply.SB.Roster, reply.SB, propagationTimeout)
	if err != nil {
		return
	}
	if replies != len(reply.SB.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// ReadRequest asks for a read-offer on the skipchain for a reader on a file.
func (s *Service) ReadRequest(req *ocs.ReadRequest) (reply *ocs.ReadReply,
	err error) {
	log.Lvl2("Requesting a file. Reader:", req.Read.Signature.SignaturePath.Signer)
	reply = &ocs.ReadReply{}
	writeSB := s.db().GetByID(req.Read.DataID)
	blockData := ocs.NewOCS(writeSB.Data)
	if blockData == nil || blockData.Write == nil {
		return nil, errors.New("Not an ocs-write block")
	}
	log.Lvlf2("Document reader is %x", blockData.Write.Reader.GetID())
	if err := req.Read.Signature.SignaturePath.Verify(darc.User); err != nil {
		return nil, errors.New("signature by wrong identity: " + err.Error())
	}
	if err := req.Read.Signature.Verify(req.Read.DataID, &blockData.Write.Reader); err != nil {
		return nil, errors.New("wrong signature: " + err.Error())
	}
	dataOCS := &ocs.Transaction{
		Read:      &req.Read,
		Timestamp: time.Now().Unix(),
	}
	data, err := protobuf.Encode(dataOCS)
	if err != nil {
		return nil, err
	}

	latestSB, err := s.db().GetLatest(writeSB)
	if err != nil {
		return nil, errors.New("didn't find latest block: " + err.Error())
	}
	i := 1
	for {
		reply.SB, err = s.storeSkipBlock(latestSB, data)
		if err == nil {
			break
		}
		if err == skipchain.ErrorProcessing {
			log.Lvl2("Waiting for block to be propagated...")
			time.Sleep(time.Duration(rand.Intn(20)*i) * time.Millisecond)
			i++
		} else {
			return nil, err
		}
	}

	replies, err := s.propagateOCS(reply.SB.Roster, reply.SB, propagationTimeout)
	if err != nil {
		return
	}
	if replies != len(reply.SB.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// GetReadRequests returns up to a maximum number of read-requests.
func (s *Service) GetReadRequests(req *ocs.GetReadRequests) (reply *ocs.GetReadRequestsReply, err error) {
	reply = &ocs.GetReadRequestsReply{}
	current := s.db().GetByID(req.Start)
	log.Lvlf2("Asking read-requests on writeID: %x", req.Start)

	if current == nil {
		return nil, errors.New("didn't find starting skipblock")
	}
	var doc skipchain.SkipBlockID
	if req.Count == 0 {
		dataOCS := ocs.NewOCS(current.Data)
		if dataOCS == nil || dataOCS.Write == nil {
			log.Error("Didn't find this writeID")
			return nil, errors.New(
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
				return nil, errors.New(
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
			current = s.db().GetByID(current.ForwardLink[0].Hash())
		} else {
			log.Lvl3("No forward-links, stopping")
			break
		}
	}
	log.Lvlf2("WriteID %x: found %d out of a maximum of %d documents", req.Start, len(reply.Documents), req.Count)
	return
}

// SharedPublic returns the shared public key of a skipchain.
func (s *Service) SharedPublic(req *ocs.SharedPublicRequest) (reply *ocs.SharedPublicReply, err error) {
	log.Lvl2("Getting shared public key")
	s.saveMutex.Lock()
	shared, ok := s.Storage.Shared[string(req.Genesis)]
	s.saveMutex.Unlock()
	if !ok {
		return nil, errors.New("didn't find this skipchain")
	}
	return &ocs.SharedPublicReply{X: shared.X}, nil
}

// DecryptKeyRequest re-encrypts the stored symmetric key under the public
// key of the read-request. Once the read-request is on the skipchain, it is
// not necessary to check its validity again.
func (s *Service) DecryptKeyRequest(req *ocs.DecryptKeyRequest) (reply *ocs.DecryptKeyReply,
	err error) {
	reply = &ocs.DecryptKeyReply{}
	log.Lvl2("Re-encrypt the key to the public key of the reader")

	readSB := s.db().GetByID(req.Read)
	read := ocs.NewOCS(readSB.Data)
	if read == nil || read.Read == nil {
		return nil, errors.New("This is not a read-block")
	}
	fileSB := s.db().GetByID(read.Read.DataID)
	file := ocs.NewOCS(fileSB.Data)
	if file == nil || file.Write == nil {
		return nil, errors.New("Data-block is broken")
	}

	// Start OCS-protocol to re-encrypt the file's symmetric key under the
	// reader's public key.
	nodes := len(fileSB.Roster.List)
	threshold := nodes - (nodes-1)/3
	tree := fileSB.Roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameOCS, tree)
	if err != nil {
		return nil, err
	}
	ocsProto := pi.(*protocol.OCS)
	ocsProto.U = file.Write.U
	verificationData := &vData{
		SB: readSB.Hash,
	}
	if req.Ephemeral != nil {
		var pub []byte
		pub, err = req.Ephemeral.MarshalBinary()
		if err != nil {
			return nil, errors.New("couldn't marshal ephemeral key")
		}
		if err = req.Signature.Verify(pub, &file.Write.Reader); err != nil {
			return nil, errors.New("wrong signature")
		}
		ocsProto.Xc = req.Ephemeral
		verificationData.Ephemeral = req.Ephemeral
		verificationData.Signature = req.Signature
	} else {
		ocsProto.Xc = read.Read.Signature.SignaturePath.Signer.Ed25519.Point
	}
	log.Lvlf2("Public key is: %s", ocsProto.Xc)
	ocsProto.VerificationData, err = network.Marshal(verificationData)
	if err != nil {
		return nil, errors.New("couldn't marshal verificationdata: " + err.Error())
	}

	// Make sure everything used from the s.Storage structure is copied, so
	// there will be no races.
	s.saveMutex.Lock()
	ocsProto.Shared = s.Storage.Shared[string(fileSB.GenesisID)]
	pp := s.Storage.Polys[string(fileSB.GenesisID)]
	reply.X = s.Storage.Shared[string(fileSB.GenesisID)].X.Clone()
	var commits []kyber.Point
	for _, c := range pp.Commits {
		commits = append(commits, c.Clone())
	}
	ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B.Clone(), commits)
	s.saveMutex.Unlock()

	ocsProto.SetConfig(&onet.GenericConfig{Data: fileSB.GenesisID})
	err = ocsProto.Start()
	if err != nil {
		return nil, err
	}
	log.Lvl3("Waiting for end of ocs-protocol")
	if !<-ocsProto.Reencrypted {
		return nil, errors.New("reencryption got refused")
	}
	reply.XhatEnc, err = share.RecoverCommit(cothority.Suite, ocsProto.Uis,
		threshold, nodes)
	if err != nil {
		return nil, err
	}
	reply.Cs = file.Write.Cs
	return
}

// storeSkipBlock calls directly the method of the service.
func (s *Service) storeSkipBlock(latest *skipchain.SkipBlock, d []byte) (sb *skipchain.SkipBlock, err error) {
	block := latest.Copy()
	block.Data = d
	reply, err := s.skipchain.StoreSkipBlock(&skipchain.StoreSkipBlock{
		LatestID: latest.Hash,
		NewBlock: block,
	})
	if err != nil {
		return nil, err
	}
	return reply.Latest, nil
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
		ocs.Verify = s.verifyReencryption
		return ocs, nil
	}
	return nil, nil
}

func (s *Service) verifyReencryption(rc *protocol.Reencrypt) bool {
	err := func() error {
		_, vdInt, err := network.Unmarshal(*rc.VerificationData, cothority.Suite)
		if err != nil {
			return err
		}
		verificationData, ok := vdInt.(*vData)
		if !ok {
			return errors.New("verificationData was not of type vData")
		}
		sb := s.db().GetByID(verificationData.SB)
		if sb == nil {
			return errors.New("received reencryption request with empty block")
		}
		o := ocs.NewOCS(sb.Data)
		if o == nil {
			return errors.New("not an OCS-data block")
		}
		if o.Read == nil {
			return errors.New("not an OCS-read block")
		}
		if verificationData.Ephemeral != nil {
			buf, err := verificationData.Ephemeral.MarshalBinary()
			if err != nil {
				return errors.New("couldn't marshal ephemeral key: " + err.Error())
			}
			darcs := *verificationData.Signature.SignaturePath.Darcs
			darc := darcs[len(darcs)-1]
			if !o.Read.Signature.SignaturePath.Signer.Ed25519.Point.Equal(
				verificationData.Signature.SignaturePath.Signer.Ed25519.Point) {
				return errors.New("ephemeral key signed by wrong reader")
			}
			if err := verificationData.Signature.Verify(buf, darc); err != nil {
				return errors.New("wrong signature on ephemeral key: " + err.Error())
			}
		} else {
			if !o.Read.Signature.SignaturePath.Signer.Ed25519.Point.Equal(rc.Xc) {
				return errors.New("wrong reader")
			}
		}
		return nil
	}()
	if err != nil {
		log.Lvl2(s.ServerIdentity(), "wrong reencryption:", err)
		return false
	}
	return true
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
			if identity.Equal(id) {
				return newpath
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
	genesis := s.db().GetByID(sb.SkipChainID())
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
		sb := s.db().GetByID(read.DataID)
		if sb == nil {
			log.Lvl2("Didn't find write-block")
			return false
		}
		wd := ocs.NewOCS(sb.Data)
		if wd == nil || wd.Write == nil {
			log.Lvl2("block was not a write-block")
			return false
		}
		writeBlock = wd.Write
		readersBlock = wd.Darc
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
	s.db().Store(sb)
	if r := dataOCS.Darc; r != nil {
		log.Lvlf2("Storing new darc %x - %x", r.GetID(), r.GetBaseID())
		s.addDarc(r)
	}
	s.save()
	if sb.Index == 0 {
		return
	}
	c := skipchain.NewClient()
	for _, sbID := range sb.BackLinkIDs {
		sbNew, err := c.GetSingleBlock(sb.Roster, sbID)
		if err != nil {
			log.Error(err)
		} else {
			s.db().Store(sbNew)
		}
	}
}

func (s *Service) db() *skipchain.SkipBlockDB {
	return s.skipchain.GetDB()
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3(s.String(), "Saving service")
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	err := s.Save("storage", s.Storage)
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
			Admins: make(map[string]*darc.Darc),
		},
		skipchain: c.Service(skipchain.ServiceName).(*skipchain.Service),
	}
	if err := s.RegisterHandlers(s.CreateSkipchains,
		s.WriteRequest, s.ReadRequest, s.GetReadRequests,
		s.DecryptKeyRequest, s.SharedPublic,
		s.UpdateDarc, s.GetDarcPath,
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
