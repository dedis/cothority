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
	"errors"
	"fmt"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/calypso/protocol"
	"github.com/dedis/cothority/darc"
	dkgprotocol "github.com/dedis/cothority/dkg/pedersen"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	dkg "github.com/dedis/kyber/share/dkg/pedersen"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// Used for tests
var calypsoID onet.ServiceID

// ServiceName of the secret-management part of Calypso.
const ServiceName = "Calypso"

// dkgTimeout is how long the system waits for the DKG to finish
const propagationTimeout = 10 * time.Second

const calypsoReshareProto = "calypso_reshare_proto"

func init() {
	var err error
	_, err = onet.GlobalProtocolRegister(calypsoReshareProto, dkgprotocol.NewSetup)
	log.ErrFatal(err)
	calypsoID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage1{}, &vData{})
}

// Service is our calypso-service. It stores all created LTSs.
type Service struct {
	*onet.ServiceProcessor
	storage *storage1
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

// AuthoriseByzcoinID adds a ByzCoinID to the list of authorized IDs. It should
// be called by the administrator at the beginning, before any other API calls
// are made. A ByzCoinID that is not authorised will not be allowed to call the
// other APIs.
func (s *Service) AuthoriseByzcoinID(req *AuthoriseByzcoinID) (*AuthoriseByzcoinIDReply, error) {
	s.storage.Lock()
	defer s.storage.Unlock()
	if len(req.ByzCoinID) == 0 {
		return nil, errors.New("empty ByzCoin ID")
	}
	key := string(req.ByzCoinID)
	if _, ok := s.storage.AuthorisedByzCoinIDs[key]; ok {
		return nil, errors.New("ByzCoinID already authorised")
	}
	s.storage.AuthorisedByzCoinIDs[key] = true
	return &AuthoriseByzcoinIDReply{}, nil
}

// CreateLTS takes as input a roster with a list of all nodes that should
// participate in the DKG. Every node will store its private key and wait for
// decryption requests. The LTSID should be the InstanceID.
func (s *Service) CreateLTS(req *CreateLTS) (reply *CreateLTSReply, err error) {
	if err := s.verifyProof(&req.Proof, nil); err != nil {
		return nil, err
	}

	roster, instID, err := s.getLtsRoster(&req.Proof)
	if err != nil {
		return nil, err
	}

	// NOTE: the roster stored in ByzCoin must have myself.
	tree := roster.GenerateNaryTreeWithRoot(len(roster.List), s.ServerIdentity())
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
	setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
	setupDKG.KeyPair = key.NewKeyPair(cothority.Suite)
	// TODO use the roster key pair
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
		key := string(reply.GetLTSID())
		s.storage.Shared[key] = shared
		s.storage.Polys[key] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.storage.Rosters[key] = roster
		s.storage.Replies[key] = reply
		s.storage.DKS[key] = dks
		s.storage.Unlock()
		s.save()
		log.Lvlf2("%v Created LTS with ID: %x, pk %v", s.ServerIdentity(), reply.GetLTSID(), reply.X)
	case <-time.After(propagationTimeout):
		return nil, errors.New("new-dkg didn't finish in time")
	}
	return
}

// ReshareLTS starts a request to reshare the LTS. The new roster which holds
// the new secret shares must exist in the InstanceID specified by the request.
// All hosts must be online in this step.
func (s *Service) ReshareLTS(req *ReshareLTS) (*ReshareLTSReply, error) {
	// Verify the request
	roster, ltsid, err := s.getLtsRoster(&req.Proof)
	if err != nil {
		return nil, err
	}
	if err := s.verifyProof(&req.Proof, roster); err != nil {
		return nil, err
	}

	// TODO(jallen): the key should be of type byzcoin.InstanceID
	key := string(ltsid)

	// Initialise the protocol
	setupDKG, err := func() (*dkgprotocol.Setup, error) {
		s.storage.Lock()
		defer s.storage.Unlock()
		// Check that we know the shared secret, otherwise don't do re-sharing
		if s.storage.Shared[key] == nil || s.storage.DKS[key] == nil {
			return nil, errors.New("cannot start resharing without an LTS")
		}

		// NOTE: the roster stored in ByzCoin must have myself.
		tree := roster.GenerateNaryTreeWithRoot(len(roster.List), s.ServerIdentity())
		cfg := reshareLtsConfig{
			Proof: req.Proof,
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
		setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
		c := &dkg.Config{
			Suite:    cothority.Suite,
			Longterm: setupDKG.KeyPair.Private,
			OldNodes: s.storage.Rosters[key].Publics(),
			NewNodes: roster.Publics(),
			Share:    s.storage.DKS[key],
		}
		setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
			return dkg.NewDistKeyHandler(c)
		}
		return setupDKG, nil
	}()
	if err != nil {
		return nil, err
	}
	if err := setupDKG.Start(); err != nil {
		return nil, err
	}
	log.Lvl3(s.ServerIdentity(), "Started resharing DKG-protocol - waiting for done", len(roster.List))

	select {
	case <-setupDKG.Finished:
		shared, dks, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, err
		}
		s.storage.Lock()
		// Check the secret shares are different
		if shared.V.Equal(s.storage.Shared[key].V) {
			s.storage.Unlock()
			return nil, errors.New("the reshared secret is the same")
		}
		// Check the public key remains the same
		if !shared.X.Equal(s.storage.Shared[key].X) {
			s.storage.Unlock()
			return nil, errors.New("the reshared public point is different")
		}
		s.storage.Shared[key] = shared
		s.storage.Polys[key] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.storage.Rosters[key] = roster
		s.storage.DKS[key] = dks
		s.storage.Unlock()
		s.save()
	case <-time.After(propagationTimeout):
		return nil, errors.New("resharing-dkg didn't finish in time")
	}

	log.Lvl2(s.ServerIdentity(), "resharing protocol finished")
	return &ReshareLTSReply{}, nil
}

func (s *Service) verifyProof(proof *byzcoin.Proof, roster *onet.Roster) error {
	/*
		scID := proof.Latest.SkipChainID()
		s.storage.Lock()
		defer s.storage.Unlock()
		if _, ok := s.storage.AuthorisedByzCoinIDs[string(scID)]; !ok {
			return errors.New("this ByzCoin ID is not authorised")
		}
		if roster == nil || !roster.ID.Equal(proof.Latest.Roster.ID) {
			return errors.New("roster ID not equal")
		}
		return proof.Verify(scID)
	*/
	return nil
}

func (s *Service) getLtsRoster(proof *byzcoin.Proof) (*onet.Roster, []byte, error) {
	instanceID, buf, _, _, err := proof.KeyValue()
	if err != nil {
		return nil, nil, err
	}

	var info LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(buf, &info, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, err
	}
	return &info.Roster, instanceID, nil
}

// DecryptKey takes as an input a Read- and a Write-proof. Proofs contain
// everything necessary to verify that a given instance is correct and
// stored in ByzCoin.
// Using the Read and the Write-instance, this method verifies that the
// requests match and then re-encrypts the secret to the public key given
// in the Read-instance.
// TODO: support ephemeral keys.
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
	roster := s.storage.Rosters[string(write.LTSID)]
	if roster == nil {
		s.storage.Unlock()
		return nil, fmt.Errorf("don't know the LTSID '%x' stored in write", write.LTSID)
	}
	scID := make([]byte, 32)
	copy(scID, s.storage.Replies[string(write.LTSID)].ByzCoinID)
	s.storage.Unlock()
	if err = dkr.Read.Verify(scID); err != nil {
		return nil, errors.New("read proof cannot be verified to come from scID: " + err.Error())
	}
	if err = dkr.Write.Verify(scID); err != nil {
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
	ocsProto.Shared = s.storage.Shared[string(write.LTSID)]
	pp := s.storage.Polys[string(write.LTSID)]
	reply.X = s.storage.Shared[string(write.LTSID)].X.Clone()
	var commits []kyber.Point
	for _, c := range pp.Commits {
		commits = append(commits, c.Clone())
	}
	ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B.Clone(), commits)
	s.storage.Unlock()

	log.Lvl3("Starting reencryption protocol")
	ocsProto.SetConfig(&onet.GenericConfig{Data: write.LTSID})
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
	reply.Cs = write.Cs
	log.Lvl3("Successfully reencrypted the key")
	return
}

// GetLTSReply returns the CreateLTSReply message of a previous LTS.
func (s *Service) GetLTSReply(req *GetLTSReply) (*CreateLTSReply, error) {
	log.Lvlf2("Getting LTS Reply for ID: %x", req.LTSID)
	s.storage.Lock()
	defer s.storage.Unlock()
	reply, ok := s.storage.Replies[string(req.LTSID)]
	if !ok {
		return nil, fmt.Errorf("didn't find this LTS: %x", req.LTSID)
	}
	return &CreateLTSReply{
		ByzCoinID:  append([]byte{}, reply.ByzCoinID...),
		InstanceID: append([]byte{}, reply.InstanceID...),
		X:          reply.X.Clone(),
	}, nil
}

func (s *Service) getKeyPair() *key.Pair {
	tree := onet.NewRoster([]*network.ServerIdentity{s.ServerIdentity()}).GenerateBinaryTree()
	tni := s.NewTreeNodeInstance(tree, tree.Root, "dummy")
	return &key.Pair{
		Public:  tni.Public(),
		Private: tni.Private(),
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
		if err := s.verifyProof(&cfg.Proof, tn.Roster()); err != nil {
			return nil, err
		}
		instID, _, _, _, err := cfg.KeyValue()
		if err != nil {
			return nil, err
		}

		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()

		go func(bcID skipchain.SkipBlockID, instID []byte) {
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
			key := string(instID)
			log.Lvlf3("%v got shared %v on key %x", s.ServerIdentity(), shared, []byte(key))
			s.storage.Lock()
			s.storage.Shared[key] = shared
			s.storage.DKS[key] = dks
			s.storage.Replies[key] = reply
			s.storage.Rosters[key] = tn.Roster()
			s.storage.Unlock()
			s.save()
		}(cfg.Latest.SkipChainID(), instID)
		return pi, nil
	case calypsoReshareProto:
		// Decode and verify config
		var cfg reshareLtsConfig
		if err := protobuf.DecodeWithConstructors(conf.Data, &cfg, network.DefaultConstructors(cothority.Suite)); err != nil {
			return nil, err
		}
		if err := s.verifyProof(&cfg.Proof, tn.Roster()); err != nil {
			return nil, err
		}

		_, ltsid, err := s.getLtsRoster(&cfg.Proof)
		key := string(ltsid)

		// Set up the protocol
		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()
		s.storage.Lock()
		c := &dkg.Config{
			Suite:    cothority.Suite,
			Longterm: setupDKG.KeyPair.Private,
			OldNodes: s.storage.Rosters[key].Publics(), // TODO won't exist for a new node
			NewNodes: tn.Roster().Publics(),
			Share:    s.storage.DKS[key], // TODO won't exist for a new node
		}
		s.storage.Unlock()
		setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
			return dkg.NewDistKeyHandler(c)
		}

		if err != nil {
			return nil, err
		}

		// Wait for DKG to end
		go func(k string) {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			log.Lvl3(s.ServerIdentity(), "Got shared", shared)
			s.storage.Lock()
			// Check the secret shares are different
			if shared.V.Equal(s.storage.Shared[k].V) {
				s.storage.Unlock()
				log.Error("the reshared secret is the same")
				return
			}
			// Check the public key remains the same
			if !shared.X.Equal(s.storage.Shared[k].X) {
				s.storage.Unlock()
				log.Error("the reshared public point is different")
				return
			}
			s.storage.Shared[k] = shared
			s.storage.DKS[k] = dks
			s.storage.Unlock()
			s.save()
		}(key)
		return setupDKG, nil
	case protocol.NameOCS:
		s.storage.Lock()
		shared, ok := s.storage.Shared[string(conf.Data)]
		for k := range s.storage.Shared {
			log.Infof("%x", []byte(k))
		}
		shared = shared.Clone()
		s.storage.Unlock()
		if !ok {
			return nil, fmt.Errorf("didn't find LTSID %x", conf.Data)
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
	}
	if err := s.RegisterHandlers(s.CreateLTS, s.ReshareLTS, s.DecryptKey, s.GetLTSReply); err != nil {
		return nil, errors.New("couldn't register messages")
	}
	byzcoin.RegisterContract(c, ContractWriteID, contractWriteFromBytes)
	byzcoin.RegisterContract(c, ContractReadID, contractReadFromBytes)
	byzcoin.RegisterContract(c, ContractLongTermSecretID, contractLTSFromBytes)
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
