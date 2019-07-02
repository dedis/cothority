// Package calypso implements the LTS functionality of the Calypso paper. It
// implements both the access-control cothority and the secret management
// cothority. (1) The access-control cothority is implemented using ByzCoin
// with two contracts, `Write` and `Read` (2) The secret-management cothority
// uses an onet service with methods to set up a Long Term Secret (LTS)
// distributed key and to request a re-encryption
//
// For more details, see
// https://github.com/dedis/cothority/tree/master/calypso/README.md
//
// There are two contracts implemented by this package:
//
// Contract "calypsoWrite" is used to store a secret in the ledger, so that an
// authorized reader can retrieve it by creating a Read-instance.
//
// Accepted Instructions:
//  - spawn:calypsoWrite creates a new write-request from the argument "write"
//  - spawn:calypsoRead creates a new read-request for this write-request.
//
// Contract "calypsoRead" is used to create read instances that prove a reader
// has access to a given write instance. They are only spawned by calling Spawn
// on an existing Write instance, with the proposed Read request in the "read"
// argument.
//
// TODO: correctly handle multi signatures for read requests: to whom should the
// secret be re-encrypted to? Perhaps for multi signatures we only want to have
// ephemeral keys.
package calypso

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"go.dedis.ch/kyber/v3/sign/schnorr"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/protocol"
	"go.dedis.ch/cothority/v3/darc"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// Used for tests
var calypsoID onet.ServiceID

// ServiceName of the secret-management part of Calypso.
const ServiceName = "Calypso"

// dkgTimeout is how long the system waits for the DKG to finish
const propagationTimeout = 20 * time.Second

const calypsoReshareProto = "calypso_reshare_proto"

var allowInsecureAdmin = false

func init() {
	var err error
	_, err = onet.GlobalProtocolRegister(calypsoReshareProto, dkgprotocol.NewSetup)
	log.ErrFatal(err)
	calypsoID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})

	// The loopback check makes Java testing not work, because Java client commands
	// come from outside of the docker container. The Java testing Docker
	// container runs with this variable set.
	if os.Getenv("COTHORITY_ALLOW_INSECURE_ADMIN") != "" {
		log.Warn("COTHORITY_ALLOW_INSECURE_ADMIN is set; Calypso admin actions allowed from the public network.")
		allowInsecureAdmin = true
	}

	err = byzcoin.RegisterGlobalContract(ContractWriteID, contractWriteFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractReadID, contractReadFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractLongTermSecretID, contractLTSFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

// Service is our calypso-service. It stores all created LTSs.
type Service struct {
	*onet.ServiceProcessor
	storage *storage
	// Genesis blocks are stored here instead of the usual skipchain DB as we don't
	// want to override authorized skipchains or related security. The blocks are
	// only used to insure that proofs start with the expected roster.
	genesisBlocks     map[string]*skipchain.SkipBlock
	genesisBlocksLock sync.Mutex
	// for use by testing only
	afterReshare func()
}

// pubPoly is a serializable version of share.PubPoly
type pubPoly struct {
	B       kyber.Point
	Commits []kyber.Point
}

// vData is sent to all nodes when re-encryption takes place. If Ephemeral
// is non-nil, Signature needs to hold a valid signature from the reader
// in the Proof.
type vData struct {
	Proof     byzcoin.Proof
	Ephemeral kyber.Point
	Signature *darc.Signature
}

// ProcessClientRequest implements onet.Service. We override the version
// we normally get from embeddeding onet.ServiceProcessor in order to
// hook it and get a look at the http.Request.
func (s *Service) ProcessClientRequest(req *http.Request, path string, buf []byte) ([]byte, *onet.StreamingTunnel, error) {

	if !allowInsecureAdmin && path == "Authorise" {
		h, _, err := net.SplitHostPort(req.RemoteAddr)

		if err != nil {
			return nil, nil, err
		}
		ip := net.ParseIP(h)
		if !ip.IsLoopback() {

			return nil, nil, errors.New("authorise is only allowed on loopback")
		}
	}
	return s.ServiceProcessor.ProcessClientRequest(req, path, buf)
}

// Authorise adds a ByzCoinID to the list of authorized IDs. It can only be called
// from localhost, except if the COTHORITY_ALLOW_INSECURE_ADMIN is set to 'true'.
// Deprecated: please use Authorize.
func (s *Service) Authorise(req *Authorise) (*AuthoriseReply, error) {
	if len(req.ByzCoinID) == 0 {
		return nil, errors.New("empty ByzCoin ID")
	}

	s.storage.Lock()
	bcID := string(req.ByzCoinID)
	if _, ok := s.storage.AuthorisedByzCoinIDs[bcID]; ok {
		s.storage.Unlock()
		return nil, errors.New("ByzCoinID already authorised")
	}
	s.storage.AuthorisedByzCoinIDs[bcID] = true
	s.storage.Unlock()

	err := s.save()
	if err != nil {
		return nil, err
	}
	log.Lvl1("Stored ByzCoinID")
	return &AuthoriseReply{}, err
}

// Authorize adds a ByzCoinID to the list of authorized IDs. It should
// be called by the administrator at the beginning, before any other API calls
// are made. A ByzCoinID that is not authorised will not be allowed to call the
// other APIs.
//
// If COTHORITY_ALLOW_INSECURE_ADMIN='true', the signature verification is
// skipped.
func (s *Service) Authorize(req *Authorize) (*AuthorizeReply, error) {
	if len(req.ByzCoinID) == 0 {
		return nil, errors.New("empty ByzCoin ID")
	}

	if !allowInsecureAdmin {
		if len(req.Signature) == 0 {
			return nil, errors.New("no signature provided")
		}
		if math.Abs(time.Now().Sub(time.Unix(req.Timestamp, 0)).Seconds()) > 60 {
			return nil, errors.New("signature is too old")
		}
		msg := append(req.ByzCoinID, make([]byte, 8)...)
		binary.LittleEndian.PutUint64(msg[32:], uint64(req.Timestamp))
		err := schnorr.Verify(cothority.Suite, s.ServerIdentity().Public, msg, req.Signature)
		if err != nil {
			return nil, errors.New("signature verification failed: " + err.Error())
		}
	}

	s.storage.Lock()
	bcID := string(req.ByzCoinID)
	if _, ok := s.storage.AuthorisedByzCoinIDs[bcID]; ok {
		s.storage.Unlock()
		return nil, errors.New("ByzCoinID already authorised")
	}
	s.storage.AuthorisedByzCoinIDs[bcID] = true
	s.storage.Unlock()

	err := s.save()
	if err != nil {
		return nil, err
	}
	log.Lvl1("Stored ByzCoinID")
	return &AuthorizeReply{}, nil
}

// CreateLTS takes as input a roster with a list of all nodes that should
// participate in the DKG. Every node will store its private key and wait for
// decryption requests. The LTSID should be the InstanceID.
func (s *Service) CreateLTS(req *CreateLTS) (reply *CreateLTSReply, err error) {
	if err := s.verifyProof(&req.Proof); err != nil {
		return nil, err
	}

	roster, instID, err := s.getLtsRoster(&req.Proof)
	if err != nil {
		return nil, err
	}

	// NOTE: the roster stored in ByzCoin must have myself.
	tree := roster.GenerateNaryTreeWithRoot(len(roster.List), s.ServerIdentity())
	if tree == nil {
		log.Error("cannot create tree with roster", roster.List)
		return nil, errors.New("error while generating tree")
	}
	cfg := newLtsConfig{
		req.Proof,
	}
	cfgBuf, err := protobuf.Encode(&cfg)
	if err != nil {
		return nil, err
	}
	pi, err := s.CreateProtocol(dkgprotocol.Name, tree)
	if err != nil {
		return nil, err
	}
	setupDKG := pi.(*dkgprotocol.Setup)
	setupDKG.Wait = true
	err = setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
	if err != nil {
		return nil, err
	}
	setupDKG.KeyPair = s.getKeyPair()

	if err := pi.Start(); err != nil {
		return nil, err
	}

	log.Lvl3("Started DKG-protocol - waiting for done", len(roster.List))
	select {
	case <-setupDKG.Finished:
		shared, dks, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, err
		}
		reply = &CreateLTSReply{
			ByzCoinID:  req.Proof.Latest.SkipChainID(),
			InstanceID: instID,
			X:          shared.X,
		}
		s.storage.Lock()
		s.storage.Shared[instID] = shared
		s.storage.Polys[instID] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.storage.Rosters[instID] = roster
		s.storage.Replies[instID] = reply
		s.storage.DKS[instID] = dks
		s.storage.Unlock()
		err = s.save()
		if err != nil {
			return nil, err
		}
		log.Lvlf2("%v Created LTS with ID: %v, pk %v", s.ServerIdentity(), instID, reply.X)
	case <-time.After(propagationTimeout):
		return nil, errors.New("new-dkg didn't finish in time")
	}
	return
}

// ReshareLTS starts a request to reshare the LTS. The new roster which holds
// the new secret shares must exist in the proof specified by the request.
// All hosts must be online in this step.
func (s *Service) ReshareLTS(req *ReshareLTS) (*ReshareLTSReply, error) {
	// Verify the request
	roster, id, err := s.getLtsRoster(&req.Proof)
	if err != nil {
		return nil, err
	}
	if err := s.verifyProof(&req.Proof); err != nil {
		return nil, err
	}

	// Initialise the protocol
	setupDKG, err := func() (*dkgprotocol.Setup, error) {
		s.storage.Lock()
		defer s.storage.Unlock()

		// Check that we know the shared secret, otherwise don't do re-sharing
		if s.storage.Shared[id] == nil || s.storage.DKS[id] == nil {
			return nil, errors.New("cannot start resharing without an LTS")
		}

		// NOTE: the roster stored in ByzCoin must have myself.
		tree := roster.GenerateNaryTreeWithRoot(len(roster.List), s.ServerIdentity())
		cfg := reshareLtsConfig{
			Proof: req.Proof,
			// We pass the public coefficients out with the protocol,
			// because new nodes will need it for their dkg.Config.PublicCoeffs.
			Commits:  s.storage.DKS[id].Commits,
			OldNodes: s.storage.Rosters[id].Publics(),
		}
		cfgBuf, err := protobuf.Encode(&cfg)
		if err != nil {
			return nil, err
		}
		pi, err := s.CreateProtocol(calypsoReshareProto, tree)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.Wait = true
		setupDKG.KeyPair = s.getKeyPair()
		err = setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
		if err != nil {
			return nil, err
		}

		// Because we are the node starting the resharing protocol, by
		// definition, we are inside the old group. (Checked first thing
		// in this function.) So we have only Share, not PublicCoeffs.
		oldn := len(s.storage.Rosters[id].List)
		n := len(roster.List)
		c := &dkg.Config{
			Suite:        cothority.Suite,
			Longterm:     setupDKG.KeyPair.Private,
			OldNodes:     s.storage.Rosters[id].Publics(),
			NewNodes:     roster.Publics(),
			Share:        s.storage.DKS[id],
			Threshold:    n - (n-1)/3,
			OldThreshold: oldn - (oldn-1)/3,
		}
		setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
			d, err := dkg.NewDistKeyHandler(c)
			return d, err
		}
		return setupDKG, nil
	}()
	if err != nil {
		return nil, err
	}
	if err := setupDKG.Start(); err != nil {
		return nil, err
	}
	log.Lvl3(s.ServerIdentity(), "Started resharing DKG-protocol - waiting for done")

	var pk kyber.Point
	select {
	case <-setupDKG.Finished:
		shared, dks, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, err
		}
		pk = shared.X
		s.storage.Lock()
		// Check the secret shares are different
		if shared.V.Equal(s.storage.Shared[id].V) {
			s.storage.Unlock()
			return nil, errors.New("the reshared secret is the same")
		}
		// Check the public key remains the same
		if !shared.X.Equal(s.storage.Shared[id].X) {
			s.storage.Unlock()
			return nil, errors.New("the reshared public point is different")
		}
		s.storage.Shared[id] = shared
		s.storage.Polys[id] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.storage.Rosters[id] = roster
		s.storage.DKS[id] = dks
		s.storage.Unlock()
		err = s.save()
		if err != nil {
			return nil, err
		}
		if s.afterReshare != nil {
			s.afterReshare()
		}
	case <-time.After(propagationTimeout):
		return nil, errors.New("resharing-dkg didn't finish in time")
	}

	log.Lvl2(s.ServerIdentity(), "resharing protocol finished")
	log.Lvlf2("%v Reshared LTS with ID: %v, pk %v", s.ServerIdentity(), id, pk)
	return &ReshareLTSReply{}, nil
}

func (s *Service) verifyProof(proof *byzcoin.Proof) error {
	scID := proof.Latest.SkipChainID()
	s.storage.Lock()
	defer s.storage.Unlock()
	if _, ok := s.storage.AuthorisedByzCoinIDs[string(scID)]; !ok {
		return errors.New("this ByzCoin ID is not authorised")
	}

	sb, err := s.fetchGenesisBlock(scID, proof.Links[0].NewRoster)
	if err != nil {
		return err
	}

	return proof.VerifyFromBlock(sb)
}

func (s *Service) fetchGenesisBlock(scID skipchain.SkipBlockID, roster *onet.Roster) (*skipchain.SkipBlock, error) {
	s.genesisBlocksLock.Lock()
	defer s.genesisBlocksLock.Unlock()
	sb := s.genesisBlocks[string(scID)]
	if sb != nil {
		return sb, nil
	}

	cl := skipchain.NewClient()
	sb, err := cl.GetSingleBlock(roster, scID)
	if err != nil {
		return nil, err
	}

	// Genesis block can be reused later on.
	s.genesisBlocks[string(scID)] = sb

	return sb, nil
}

func (s *Service) getLtsRoster(proof *byzcoin.Proof) (*onet.Roster, byzcoin.InstanceID, error) {
	instanceID, buf, _, _, err := proof.KeyValue()
	if err != nil {
		return nil, byzcoin.InstanceID{}, err
	}

	var info LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(buf, &info, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, byzcoin.InstanceID{}, err
	}
	return &info.Roster, byzcoin.NewInstanceID(instanceID), nil
}

// DecryptKey takes as an input a Read- and a Write-proof. Proofs contain
// everything necessary to verify that a given instance is correct and
// stored in ByzCoin.
// Using the Read and the Write-instance, this method verifies that the
// requests match and then re-encrypts the secret to the public key given
// in the Read-instance.
func (s *Service) DecryptKey(dkr *DecryptKey) (reply *DecryptKeyReply, err error) {
	reply = &DecryptKeyReply{}
	log.Lvl2(s.ServerIdentity(), "Re-encrypt the key to the public key of the reader")

	var read Read
	if err := dkr.Read.VerifyAndDecode(cothority.Suite, ContractReadID, &read); err != nil {
		return nil, errors.New("didn't get a read instance: " + err.Error())
	}

	var write Write
	if err := dkr.Write.VerifyAndDecode(cothority.Suite, ContractWriteID, &write); err != nil {
		return nil, errors.New("didn't get a write instance: " + err.Error())
	}
	if !read.Write.Equal(byzcoin.NewInstanceID(dkr.Write.InclusionProof.Key())) {
		return nil, errors.New("read doesn't point to passed write")
	}
	s.storage.Lock()
	id := write.LTSID
	roster := s.storage.Rosters[id]
	if roster == nil {
		s.storage.Unlock()
		return nil, fmt.Errorf("don't know the LTSID '%v' stored in write", id)
	}
	s.storage.Unlock()

	if err = s.verifyProof(&dkr.Read); err != nil {
		return nil, errors.New("read proof cannot be verified to come from scID: " + err.Error())
	}
	if err = s.verifyProof(&dkr.Write); err != nil {
		return nil, errors.New("write proof cannot be verified to come from scID: " + err.Error())
	}

	// Start ocs-protocol to re-encrypt the file's symmetric key under the
	// reader's public key.
	nodes := len(roster.List)
	threshold := nodes - (nodes-1)/3
	tree := roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameOCS, tree)
	if err != nil {
		return nil, err
	}
	ocsProto := pi.(*protocol.OCS)
	ocsProto.U = write.U
	verificationData := &vData{
		Proof: dkr.Read,
	}
	ocsProto.Xc = read.Xc
	log.Lvlf2("%v Public key is: %s", s.ServerIdentity(), ocsProto.Xc)
	ocsProto.VerificationData, err = protobuf.Encode(verificationData)
	if err != nil {
		return nil, errors.New("couldn't marshal verification data: " + err.Error())
	}

	// Make sure everything used from the s.Storage structure is copied, so
	// there will be no races.
	s.storage.Lock()
	ocsProto.Shared = s.storage.Shared[id]
	pp := s.storage.Polys[id]
	reply.X = s.storage.Shared[id].X.Clone()
	var commits []kyber.Point
	for _, c := range pp.Commits {
		commits = append(commits, c.Clone())
	}
	ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B.Clone(), commits)
	s.storage.Unlock()

	log.Lvl3("Starting reencryption protocol")
	err = ocsProto.SetConfig(&onet.GenericConfig{Data: id.Slice()})
	if err != nil {
		return nil, err
	}
	err = ocsProto.Start()
	if err != nil {
		return nil, err
	}
	if !<-ocsProto.Reencrypted {
		return nil, errors.New("reencryption got refused")
	}
	log.Lvl3("Reencryption protocol is done.")
	reply.XhatEnc, err = share.RecoverCommit(cothority.Suite, ocsProto.Uis,
		threshold, nodes)
	if err != nil {
		return nil, err
	}
	reply.C = write.C
	log.Lvl3("Successfully reencrypted the key")
	return
}

// GetLTSReply returns the CreateLTSReply message of a previous LTS.
func (s *Service) GetLTSReply(req *GetLTSReply) (*CreateLTSReply, error) {
	log.Lvlf2("Getting LTS Reply for ID: %v", req.LTSID)
	s.storage.Lock()
	defer s.storage.Unlock()
	reply, ok := s.storage.Replies[req.LTSID]
	if !ok {
		return nil, fmt.Errorf("didn't find this LTS: %v", req.LTSID)
	}
	return &CreateLTSReply{
		ByzCoinID:  append([]byte{}, reply.ByzCoinID...),
		InstanceID: reply.InstanceID,
		X:          reply.X.Clone(),
	}, nil
}

func (s *Service) getKeyPair() *key.Pair {
	return &key.Pair{
		Public:  s.ServerIdentity().ServicePublic(ServiceName),
		Private: s.ServerIdentity().ServicePrivate(ServiceName),
	}
}

// NewProtocol intercepts the DKG and OCS protocols to retrieve the values
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), tn.ProtocolName(), conf)
	switch tn.ProtocolName() {
	case dkgprotocol.Name:
		var cfg newLtsConfig
		if err := protobuf.DecodeWithConstructors(conf.Data, &cfg, network.DefaultConstructors(cothority.Suite)); err != nil {
			return nil, err
		}
		if err := s.verifyProof(&cfg.Proof); err != nil {
			return nil, err
		}
		inst, _, _, _, err := cfg.KeyValue()
		if err != nil {
			return nil, err
		}
		instID := byzcoin.NewInstanceID(inst)

		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()

		go func(bcID skipchain.SkipBlockID, id byzcoin.InstanceID) {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			reply := &CreateLTSReply{
				ByzCoinID:  bcID,
				InstanceID: instID,
				X:          shared.X,
			}
			log.Lvlf3("%v got shared %v on inst %v", s.ServerIdentity(), shared, id)
			s.storage.Lock()
			s.storage.Shared[id] = shared
			s.storage.DKS[id] = dks
			s.storage.Replies[id] = reply
			s.storage.Rosters[id] = tn.Roster()
			s.storage.Unlock()
			err = s.save()
			if err != nil {
				log.Error(err)
			}
		}(cfg.Latest.SkipChainID(), instID)
		return pi, nil
	case calypsoReshareProto:
		// Decode and verify config
		var cfg reshareLtsConfig
		if err := protobuf.DecodeWithConstructors(conf.Data, &cfg, network.DefaultConstructors(cothority.Suite)); err != nil {
			return nil, err
		}
		if err := s.verifyProof(&cfg.Proof); err != nil {
			return nil, err
		}

		_, id, err := s.getLtsRoster(&cfg.Proof)

		// Set up the protocol
		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()

		s.storage.Lock()
		oldn := len(cfg.OldNodes)
		n := len(tn.Roster().List)
		c := &dkg.Config{
			Suite:        cothority.Suite,
			Longterm:     setupDKG.KeyPair.Private,
			NewNodes:     tn.Roster().Publics(),
			OldNodes:     cfg.OldNodes,
			Threshold:    n - (n-1)/3,
			OldThreshold: oldn - (oldn-1)/3,
		}
		s.storage.Unlock()

		// Set Share and PublicCoeffs according to if we are an old node or a new one.
		inOld := pointInList(setupDKG.KeyPair.Public, cfg.OldNodes)
		if inOld {
			c.Share = s.storage.DKS[id]
		} else {
			c.PublicCoeffs = cfg.Commits
		}

		setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
			d, err := dkg.NewDistKeyHandler(c)
			return d, err
		}

		if err != nil {
			return nil, err
		}

		// Wait for DKG in reshare mode to end
		go func(id byzcoin.InstanceID) {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}

			s.storage.Lock()
			// If we had an old share, check the new share before saving it.
			if s.storage.Shared[id] != nil {
				// Check the secret shares are different
				if shared.V.Equal(s.storage.Shared[id].V) {
					s.storage.Unlock()
					log.Error("the reshared secret is the same")
					return
				}

				// Check the public key remains the same
				if !shared.X.Equal(s.storage.Shared[id].X) {
					s.storage.Unlock()
					log.Error("the reshared public point is different")
					return
				}
			}
			s.storage.Shared[id] = shared
			s.storage.DKS[id] = dks
			s.storage.Unlock()
			err = s.save()
			if err != nil {
				log.Error(err)
			}
			if s.afterReshare != nil {
				s.afterReshare()
			}
		}(id)
		return setupDKG, nil
	case protocol.NameOCS:
		id := byzcoin.NewInstanceID(conf.Data)
		s.storage.Lock()
		shared, ok := s.storage.Shared[id]
		shared = shared.Clone()
		s.storage.Unlock()
		if !ok {
			return nil, fmt.Errorf("didn't find LTSID %v", id)
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

func pointInList(p1 kyber.Point, l []kyber.Point) bool {
	for _, p2 := range l {
		if p2.Equal(p1) {
			return true
		}
	}
	return false
}

// verifyReencryption checks that the read and the write instances match.
func (s *Service) verifyReencryption(rc *protocol.Reencrypt) bool {
	err := func() error {
		var verificationData vData
		err := protobuf.DecodeWithConstructors(*rc.VerificationData, &verificationData, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return err
		}
		_, v0, contractID, _, err := verificationData.Proof.KeyValue()
		if err != nil {
			return errors.New("proof cannot return values: " + err.Error())
		}
		if contractID != ContractReadID {
			return errors.New("proof doesn't point to read instance")
		}
		var r Read
		err = protobuf.DecodeWithConstructors(v0, &r, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return errors.New("couldn't decode read data: " + err.Error())
		}
		if verificationData.Ephemeral != nil {
			return errors.New("ephemeral keys not supported yet")
		}
		if !r.Xc.Equal(rc.Xc) {
			return errors.New("wrong reader")
		}
		return nil
	}()
	if err != nil {
		log.Lvl2(s.ServerIdentity(), "wrong reencryption:", err)
		return false
	}
	return true
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		genesisBlocks:    make(map[string]*skipchain.SkipBlock),
	}
	if err := s.RegisterHandlers(s.CreateLTS, s.ReshareLTS, s.DecryptKey,
		s.GetLTSReply, s.Authorise, s.Authorize); err != nil {
		return nil, errors.New("couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
