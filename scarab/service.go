// Package scarab implements the LTS functionality of the Scarab paper.
// It implements both the Access Control (AC) and the Secret Caring (SC):
//  - AC is implemented using OmniLedger with two contracts, `Write` and `Read`
//  - SC uses an onet service with methods to set up a Long Term Secret (LTS)
//   distributed key and to request a re-encryption
//
// For more details, see https://github.com/dedis/cothority/tree/master/scarab/README.md
package scarab

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/protocol"
	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// Used for tests
var scarabID onet.ServiceID

// ServiceName of the SC part of Scarab.
var ServiceName = "scarab"

// dkgTimeout is how long the system waits for the DKG to finish
const propagationTimeout = 10 * time.Second

func init() {
	var err error
	scarabID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})
}

// Service is our scarab-service. It stores all created LTSs.
type Service struct {
	*onet.ServiceProcessor
	saveMutex sync.Mutex
	storage   *storage
}

// storageID reflects the data we're storing - we could store more
// than one structure.
var storageID = []byte("main")

// pubPoly is a serializable version of share.PubPoly
type pubPoly struct {
	B       kyber.Point
	Commits []kyber.Point
}

// storage is used to save all elements of the DKG.
type storage struct {
	Shared  map[string]*protocol.SharedSecret
	Polys   map[string]*pubPoly
	Rosters map[string]*onet.Roster
	OLIDs   map[string]skipchain.SkipBlockID
}

// vData is sent to all nodes when re-encryption takes place. If Ephemeral
// is non-nil, Signature needs to hold a valid signature from the reader
// in the Proof.
type vData struct {
	Proof     ol.Proof
	Ephemeral kyber.Point
	Signature *darc.Signature
}

// CreateLTS takes as input a roster with a list of all nodes that should
// participate in the DKG. Every node will store its private key and wait
// for decryption requests.
// This method will create a random LTSID that can be used to reference
// the LTS group created.
func (s *Service) CreateLTS(cl *CreateLTS) (reply *CreateLTSReply, err error) {
	tree := cl.Roster.GenerateNaryTreeWithRoot(len(cl.Roster.List), s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameDKG, tree)
	setupDKG := pi.(*protocol.SetupDKG)
	setupDKG.Wait = true
	reply = &CreateLTSReply{LTSID: make([]byte, 32)}
	random.New().XORKeyStream(reply.LTSID, reply.LTSID)
	setupDKG.SetConfig(&onet.GenericConfig{Data: reply.LTSID})
	log.Lvlf3("%s: reply.LTSID is: %x", s.ServerIdentity(), reply.LTSID)
	if err := pi.Start(); err != nil {
		return nil, err
	}
	log.Lvl3("Started DKG-protocol - waiting for done", len(cl.Roster.List))
	select {
	case <-setupDKG.SetupDone:
		shared, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, err
		}
		s.saveMutex.Lock()
		s.storage.Shared[string(reply.LTSID)] = shared
		dks, err := setupDKG.DKG.DistKeyShare()
		if err != nil {
			s.saveMutex.Unlock()
			return nil, err
		}
		s.storage.Polys[string(reply.LTSID)] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
		s.storage.Rosters[string(reply.LTSID)] = &cl.Roster
		s.storage.OLIDs[string(reply.LTSID)] = cl.OLID
		s.saveMutex.Unlock()
		reply.X = shared.X
	case <-time.After(propagationTimeout):
		return nil, errors.New("dkg didn't finish in time")
	}
	return
}

// DecryptKey takes as an input a Read- and a Write-proof. Proofs contain
// everything necessary to verify that a given instance is correct and
// stored in omniledger.
// Using the Read and the Write-instance, this method verifies that the
// requests match and then re-encrypts the secret to the public key given
// in the Read-instance.
// TODO: support ephemeral keys.
func (s *Service) DecryptKey(dkr *DecryptKey) (reply *DecryptKeyReply, err error) {
	reply = &DecryptKeyReply{}
	log.Lvl2("Re-encrypt the key to the public key of the reader")

	var read Read
	if err := dkr.Read.ContractValue(cothority.Suite, ContractReadID, &read); err != nil {
		return nil, errors.New("didn't get a read instance: " + err.Error())
	}
	var write Write
	if err := dkr.Write.ContractValue(cothority.Suite, ContractWriteID, &write); err != nil {
		return nil, errors.New("didn't get a write instance: " + err.Error())
	}
	if !read.Write.Equal(ol.NewInstanceID(dkr.Write.InclusionProof.Key)) {
		return nil, errors.New("read doesn't point to passed write")
	}
	s.saveMutex.Lock()
	roster := s.storage.Rosters[string(write.LTSID)]
	if roster == nil {
		s.saveMutex.Unlock()
		return nil, errors.New("don't know the LTSID stored in write")
	}
	scID := s.storage.OLIDs[string(write.LTSID)]
	s.saveMutex.Unlock()
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
	log.Lvlf2("Public key is: %s", ocsProto.Xc)
	ocsProto.VerificationData, err = network.Marshal(verificationData)
	if err != nil {
		return nil, errors.New("couldn't marshal verificationdata: " + err.Error())
	}

	// Make sure everything used from the s.Storage structure is copied, so
	// there will be no races.
	s.saveMutex.Lock()
	ocsProto.Shared = s.storage.Shared[string(write.LTSID)]
	pp := s.storage.Polys[string(write.LTSID)]
	reply.X = s.storage.Shared[string(write.LTSID)].X.Clone()
	var commits []kyber.Point
	for _, c := range pp.Commits {
		commits = append(commits, c.Clone())
	}
	ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B.Clone(), commits)
	s.saveMutex.Unlock()

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

// SharedPublic returns the shared public key of an LTSID group.
func (s *Service) SharedPublic(req *SharedPublic) (reply *SharedPublicReply, err error) {
	log.Lvl2("Getting shared public key")
	s.saveMutex.Lock()
	shared, ok := s.storage.Shared[string(req.LTSID)]
	s.saveMutex.Unlock()
	if !ok {
		return nil, errors.New("didn't find this skipchain")
	}
	return &SharedPublicReply{X: shared.X}, nil
}

// NewProtocol intercepts the DKG and OCS protocols to retrieve the values
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), tn.ProtocolName(), conf)
	switch tn.ProtocolName() {
	case protocol.NameDKG:
		pi, err := protocol.NewSetupDKG(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*protocol.SetupDKG)
		go func(conf *onet.GenericConfig) {
			<-setupDKG.SetupDone
			shared, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			log.Lvl3(s.ServerIdentity(), "Got shared", shared)
			s.saveMutex.Lock()
			s.storage.Shared[string(conf.Data)] = shared
			s.saveMutex.Unlock()
			s.save()
		}(conf)
		return pi, nil
	case protocol.NameOCS:
		s.saveMutex.Lock()
		shared, ok := s.storage.Shared[string(conf.Data)]
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

// verifyReencryption checks that the read and the write instances match.
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
		_, vs, err := verificationData.Proof.KeyValue()
		if err != nil {
			return errors.New("proof cannot return values: " + err.Error())
		}
		if bytes.Compare(vs[1], []byte(ContractReadID)) != 0 {
			return errors.New("proof doesn't point to read instance")
		}
		var r Read
		err = protobuf.DecodeWithConstructors(vs[0], &r, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return errors.New("couldn't decode read data: " + err.Error())
		}
		if verificationData.Ephemeral != nil {
			return errors.New("ephemeral keys not supported yet")
		} else {
			if !r.Xc.Equal(rc.Xc) {
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

// saves all data.
func (s *Service) save() {
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	err := s.Save(storageID, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}
	defer func() {
		if len(s.storage.Polys) == 0 {
			s.storage.Polys = make(map[string]*pubPoly)
		}
		if len(s.storage.Shared) == 0 {
			s.storage.Shared = make(map[string]*protocol.SharedSecret)
		}
		if len(s.storage.Rosters) == 0 {
			s.storage.Rosters = make(map[string]*onet.Roster)
		}
		if len(s.storage.OLIDs) == 0 {
			s.storage.OLIDs = make(map[string]skipchain.SkipBlockID)
		}
	}()
	msg, err := s.Load(storageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.CreateLTS, s.DecryptKey); err != nil {
		return nil, errors.New("Couldn't register messages")
	}
	ol.RegisterContract(c, ContractWriteID, s.ContractWrite)
	ol.RegisterContract(c, ContractReadID, s.ContractRead)
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
