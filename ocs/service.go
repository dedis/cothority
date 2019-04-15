// Package OCS is a general-purpose re-encryption service that can be used
// either in ByzCoin with the Calypso-service and its contracts, or with
// Ethereum/Fabric. It is extensible to work also with other kind of
// Access-control backends, e.g., Ethereum.
package ocs

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"go.dedis.ch/cothority/v3/ocs/certs"

	"go.dedis.ch/kyber/v3/sign/schnorr"

	"go.dedis.ch/kyber/v3/suites"

	"go.dedis.ch/cothority/v3"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// ServiceName of the secret-management part of Calypso.
const ServiceName = "OCS"

// dkgTimeout is how long the system waits for the DKG to finish
const propagationTimeout = 20 * time.Second

const calypsoReshareProto = "ocs_reshare_proto"

var disableLoopbackCheck = false

func init() {
	var err error
	_, err = onet.GlobalProtocolRegister(calypsoReshareProto, dkgprotocol.NewSetup)
	log.ErrFatal(err)
	_, err = onet.RegisterNewServiceWithSuite(ServiceName, suites.MustFind("ed25519"), newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &storageElement{})

	// The loopback check makes Java testing not work, because Java client commands
	// come from outside of the docker container. The Java testing Docker
	// container runs with this variable set.
	if os.Getenv("COTHORITY_ALLOW_INSECURE_ADMIN") != "" {
		log.Warn("COTHORITY_ALLOW_INSECURE_ADMIN is set; Calypso admin actions allowed from the public network.")
		disableLoopbackCheck = true
	}
}

// Service is our calypso-service. It stores all created LTSs.
type Service struct {
	*onet.ServiceProcessor
	storage      *storage
	afterReshare func() // for use by testing only
}

// pubPoly is a serializable version of share.PubPoly
type pubPoly struct {
	B       kyber.Point
	Commits []kyber.Point
}

// ProcessClientRequest implements onet.Service. We override the version
// we normally get from embedding onet.ServiceProcessor in order to
// hook it and get a look at the http.Request.
func (s *Service) ProcessClientRequest(req *http.Request, path string, buf []byte) ([]byte, *onet.StreamingTunnel, error) {
	if !disableLoopbackCheck && path == "AddPolicyCreateOCS" {
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

// AddPolicyCreateOCS defines who is allowed to create new OCS instances. Per default
// it only accepts requests on the localhost interface. But this can be overridden
// by the COTHORITY_ALLOW_INSECURE_ADMIN variable.
func (s *Service) AddPolicyCreateOCS(req *AddPolicyCreateOCS) (reply *AddPolicyCreateOCSReply, err error) {
	s.storage.Lock()
	s.storage.PolicyCreateOCS = append(s.storage.PolicyCreateOCS, req.Create)
	s.storage.Unlock()
	return &AddPolicyCreateOCSReply{}, s.save()
}

// CreateOCS takes as input a roster with a list of all nodes that should
// participate in the DKG. Every node will store its private key and wait for
// decryption requests.
func (s *Service) CreateOCS(req *CreateOCS) (reply *CreateOCSReply, err error) {
	if err = req.verify(); err != nil {
		return nil, certs.Erret(err)
	}

	tree := req.Roster.GenerateNaryTreeWithRoot(len(req.Roster.List), s.ServerIdentity())
	cfgBuf, err := protobuf.Encode(req)
	if err != nil {
		return nil, certs.Erret(err)
	}
	pi, err := s.CreateProtocol(dkgprotocol.Name, tree)
	if err != nil {
		return nil, certs.Erret(err)
	}
	setupDKG := pi.(*dkgprotocol.Setup)
	setupDKG.Wait = true
	err = setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
	if err != nil {
		return nil, certs.Erret(err)
	}
	setupDKG.KeyPair = s.getKeyPair()

	if err := pi.Start(); err != nil {
		return nil, certs.Erret(err)
	}

	log.Lvl3("Started DKG-protocol - waiting for done", len(req.Roster.List))
	var oid []byte
	select {
	case <-setupDKG.Finished:
		shared, dks, err := setupDKG.SharedSecret()
		if err != nil {
			return nil, certs.Erret(err)
		}
		ocsID, err := NewOCSID(shared.X)
		if err != nil {
			return nil, certs.Erret(err)
		}
		reply = &CreateOCSReply{
			OcsID: ocsID,
		}
		oid, err = shared.X.MarshalBinary()
		if err != nil {
			return nil, certs.Erret(err)
		}
		s.storage.Lock()
		s.storage.Element[string(oid)] = &storageElement{
			PolicyReencrypt: req.PolicyReencrypt,
			PolicyReshare:   req.PolicyReshare,
			Shared:          *shared,
			Polys:           pubPoly{s.Suite().Point().Base(), dks.Commits},
			Roster:          req.Roster,
			DKS:             *dks,
		}
		s.storage.Unlock()
		err = s.save()
		if err != nil {
			return nil, err
		}
		log.Lvlf2("%v Created LTS with ID (=^pubKey): %x", s.ServerIdentity(), oid)
	case <-time.After(propagationTimeout):
		return nil, errors.New("new-dkg didn't finish in time")
	}
	return
}

// GetProof returns a signed proof of the requested OcsID.
func (s *Service) GetProof(req *GetProof) (reply *GetProofReply, err error) {
	s.storage.Lock()
	es, found := s.storage.Element[string(req.OcsID)]
	if !found {
		return nil, errors.New("didn't find the given OCS")
	}
	s.storage.Unlock()
	reply = &GetProofReply{
		Proof: OCSProof{
			OcsID:           req.OcsID,
			Roster:          es.Roster,
			PolicyReencrypt: es.PolicyReencrypt,
			PolicyReshare:   es.PolicyReshare,
		},
	}
	msg, err := reply.Proof.Message()
	if err != nil {
		return nil, certs.Erret(err)
	}
	sig, err := schnorr.Sign(cothority.Suite, s.ServerIdentity().ServicePrivate(ServiceName), msg)
	if err != nil {
		return nil, certs.Erret(err)
	}
	reply.Proof.Signatures = [][]byte{sig}
	return
}

// Reencrypt takes as an input a Read- and a Write-proof. Proofs contain
// everything necessary to verify that a given instance is correct and
// stored in ByzCoin.
// Using the Read and the Write-instance, this method verifies that the
// requests match and then re-encrypts the secret to the public key given
// in the Read-instance.
func (s *Service) Reencrypt(dkr *Reencrypt) (reply *ReencryptReply, err error) {
	reply = &ReencryptReply{}
	log.Lvl2(s.ServerIdentity(), "Re-encrypt the key to the public key of the reader")

	s.storage.Lock()
	es, found := s.storage.Element[string(dkr.OcsID)]
	s.storage.Unlock()
	if !found {
		return nil, errors.New("didn't find this OCS")
	}

	var threshold int
	var nodes int
	var ocsProto *OCS
	err = func() error {
		// Start the ocs-protocol to re-encrypt the data under the public key of the reader.
		nodes = len(es.Roster.List)
		threshold = nodes - (nodes-1)/3
		tree := es.Roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
		pi, err := s.CreateProtocol(NameOCS, tree)
		if err != nil {
			return certs.Erret(err)
		}
		ocsProto = pi.(*OCS)
		ocsProto.U, err = dkr.Auth.U()
		if err != nil {
			return certs.Erret(err)
		}
		X, err := dkr.OcsID.X()
		if err != nil {
			return certs.Erret(err)
		}
		if err = dkr.Auth.verify(es.PolicyReencrypt, X, ocsProto.U); err != nil {
			return certs.Erret(err)
		}
		ocsProto.Xc, err = dkr.Auth.Xc()
		if err != nil {
			return certs.Erret(err)
		}
		log.Lvlf2("%v Public key is: %s", s.ServerIdentity(), ocsProto.Xc)
		ocsProto.VerificationData, err = protobuf.Encode(&dkr.Auth)
		if err != nil {
			return errors.New("couldn't marshal verification data: " + err.Error())
		}

		// Make sure everything used from the s.Storage structure is copied, so
		// there will be no races.
		ocsProto.Shared = &es.Shared
		pp := es.Polys
		reply.X = es.Shared.X.Clone()
		var commits []kyber.Point
		for _, c := range pp.Commits {
			commits = append(commits, c.Clone())
		}
		ocsProto.Poly = share.NewPubPoly(s.Suite(), pp.B.Clone(), commits)
		return nil
	}()
	if err != nil {
		if ocsProto != nil {
			ocsProto.Done()
		}
		return nil, err
	}

	log.Lvl3("Starting reencryption protocol", ocsProto.TreeNodeInstance.TokenID())
	err = ocsProto.SetConfig(&onet.GenericConfig{Data: dkr.OcsID})
	if err != nil {
		return nil, certs.Erret(err)
	}
	err = ocsProto.Start()
	if err != nil {
		return nil, certs.Erret(err)
	}
	if !<-ocsProto.Reencrypted {
		return nil, errors.New("reencryption got refused")
	}
	log.Lvl3("Reencryption protocol is done.")
	reply.XhatEnc, err = share.RecoverCommit(cothority.Suite, ocsProto.Uis,
		threshold, nodes)
	if err != nil {
		return nil, certs.Erret(err)
	}
	if err != nil {
		return nil, certs.Erret(err)
	}
	log.Lvl3("Successfully reencrypted the key")
	return
}

// ReshareLTS starts a request to reshare the LTS. The new roster which holds
// the new secret shares must exist in the proof specified by the request.
// All hosts must be online in this step.
func (s *Service) ReshareLTS(req *Reshare) (*ReshareReply, error) {
	return nil, errors.New("not yet implemented")
	//// Verify the request
	//roster, id, err := s.getLtsRoster(&req.Proof)
	//if err != nil {
	//	return nil, err
	//}
	//if err := s.verifyProof(&req.Proof, roster); err != nil {
	//	return nil, err
	//}
	//
	//// Initialise the protocol
	//setupDKG, err := func() (*dkgprotocol.Setup, error) {
	//	s.storage.Lock()
	//	defer s.storage.Unlock()
	//
	//	// Check that we know the shared secret, otherwise don't do re-sharing
	//	if s.storage.Shared[id] == nil || s.storage.DKS[id] == nil {
	//		return nil, errors.New("cannot start resharing without an LTS")
	//	}
	//
	//	// NOTE: the roster stored in ByzCoin must have myself.
	//	tree := roster.GenerateNaryTreeWithRoot(len(roster.List), s.ServerIdentity())
	//	cfg := reshareLtsConfig{
	//		Proof: req.Proof,
	//		// We pass the public coefficients out with the protocol,
	//		// because new nodes will need it for their dkg.Config.PublicCoeffs.
	//		Commits:  s.storage.DKS[id].Commits,
	//		OldNodes: s.storage.Rosters[id].Publics(),
	//	}
	//	cfgBuf, err := protobuf.Encode(&cfg)
	//	if err != nil {
	//		return nil, err
	//	}
	//	pi, err := s.CreateProtocol(calypsoReshareProto, tree)
	//	if err != nil {
	//		return nil, err
	//	}
	//	setupDKG := pi.(*dkgprotocol.Setup)
	//	setupDKG.Wait = true
	//	setupDKG.KeyPair = s.getKeyPair()
	//	setupDKG.SetConfig(&onet.GenericConfig{Data: cfgBuf})
	//
	//	// Because we are the node starting the resharing protocol, by
	//	// definition, we are inside the old group. (Checked first thing
	//	// in this function.) So we have only Share, not PublicCoeffs.
	//	n := len(roster.List)
	//	c := &dkg.Config{
	//		Suite:     cothority.Suite,
	//		Longterm:  setupDKG.KeyPair.Private,
	//		OldNodes:  s.storage.Rosters[id].Publics(),
	//		NewNodes:  roster.Publics(),
	//		Share:     s.storage.DKS[id],
	//		Threshold: n - (n-1)/3,
	//	}
	//	setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
	//		d, err := dkg.NewDistKeyHandler(c)
	//		return d, err
	//	}
	//	return setupDKG, nil
	//}()
	//if err != nil {
	//	return nil, err
	//}
	//if err := setupDKG.Start(); err != nil {
	//	return nil, err
	//}
	//log.Lvl3(s.ServerIdentity(), "Started resharing DKG-protocol - waiting for done")
	//
	//var pk kyber.Point
	//select {
	//case <-setupDKG.Finished:
	//	shared, dks, err := setupDKG.SharedSecret()
	//	if err != nil {
	//		return nil, err
	//	}
	//	pk = shared.X
	//	s.storage.Lock()
	//	// Check the secret shares are different
	//	if shared.V.Equal(s.storage.Shared[id].V) {
	//		s.storage.Unlock()
	//		return nil, errors.New("the reshared secret is the same")
	//	}
	//	// Check the public key remains the same
	//	if !shared.X.Equal(s.storage.Shared[id].X) {
	//		s.storage.Unlock()
	//		return nil, errors.New("the reshared public point is different")
	//	}
	//	s.storage.Shared[id] = shared
	//	s.storage.Polys[id] = &pubPoly{s.Suite().Point().Base(), dks.Commits}
	//	s.storage.Rosters[id] = roster
	//	s.storage.DKS[id] = dks
	//	s.storage.Unlock()
	//	s.save()
	//	if s.afterReshare != nil {
	//		s.afterReshare()
	//	}
	//case <-time.After(propagationTimeout):
	//	return nil, errors.New("resharing-dkg didn't finish in time")
	//}
	//
	//log.Lvl2(s.ServerIdentity(), "resharing protocol finished")
	//log.Lvlf2("%v Reshared LTS with ID: %v, pk %v", s.ServerIdentity(), id, pk)
	//return &ReshareLTSReply{}, nil
}

func (s *Service) getKeyPair() *key.Pair {
	return &key.Pair{
		Public:  s.ServerIdentity().Public,
		Private: s.ServerIdentity().GetPrivate(),
	}
}

// NewProtocol intercepts the DKG and OCS protocols to retrieve the values
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), tn.ProtocolName(), len(conf.Data), tn.TokenID())
	switch tn.ProtocolName() {
	case dkgprotocol.Name:
		var cfg CreateOCS
		if err := protobuf.DecodeWithConstructors(conf.Data, &cfg, network.DefaultConstructors(cothority.Suite)); err != nil {
			return nil, err
		}
		if err := cfg.verify(); err != nil {
			return nil, err
		}

		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()

		go func() {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			idBuf, err := shared.X.MarshalBinary()
			if err != nil {
				log.Error(err)
				return
			}
			id := string(idBuf)
			log.Lvlf3("%v got shared %v on inst %v", s.ServerIdentity(), shared, id)
			s.storage.Lock()
			s.storage.Element[string(id)] = &storageElement{
				PolicyReencrypt: cfg.PolicyReencrypt,
				PolicyReshare:   cfg.PolicyReshare,
				Shared:          *shared,
				Polys:           pubPoly{s.Suite().Point().Base(), dks.Commits},
				Roster:          *tn.Roster(),
				DKS:             *dks,
			}
			s.storage.Unlock()
			err = s.save()
			if err != nil {
				log.Error(err)
			}
		}()
		return pi, nil

	case calypsoReshareProto:
		// Decode and verify config
		var cfg Reshare
		if err := protobuf.DecodeWithConstructors(conf.Data, &cfg, network.DefaultConstructors(cothority.Suite)); err != nil {
			return nil, err
		}
		if err := cfg.verify(); err != nil {
			return nil, err
		}

		// Set up the protocol
		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		setupDKG.KeyPair = s.getKeyPair()

		s.storage.Lock()
		id := string(cfg.OcsID)
		es, found := s.storage.Element[id]
		if !found {
			// TODO: we might not have this yet - so probably we need to put the old roster in cfg, too.
			return nil, errors.New("this OCSID is not known here")
		}
		oldNodes := es.Roster.Publics()
		n := len(tn.Roster().List)
		c := &dkg.Config{
			Suite:     cothority.Suite,
			Longterm:  setupDKG.KeyPair.Private,
			NewNodes:  tn.Roster().Publics(),
			OldNodes:  oldNodes,
			Threshold: n - (n-1)/3,
		}

		// Set Share and PublicCoeffs according to if we are an old node or a new one.
		inOld := pointInList(setupDKG.KeyPair.Public, oldNodes)
		if inOld {
			c.Share = &es.DKS
		} else {
			// TODO: add commits here
			//c.PublicCoeffs = cfg.Commits
		}
		s.storage.Unlock()

		setupDKG.NewDKG = func() (*dkg.DistKeyGenerator, error) {
			d, err := dkg.NewDistKeyHandler(c)
			return d, err
		}

		if err != nil {
			return nil, err
		}

		// Wait for DKG in reshare mode to end
		go func() {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}

			s.storage.Lock()
			es, found := s.storage.Element[id]
			// If we had an old share, check the new share before saving it.
			if found {
				// Check the secret shares are different
				if shared.V.Equal(es.Shared.V) {
					s.storage.Unlock()
					log.Error("the reshared secret is the same")
					return
				}

				// Check the public key remains the same
				if !shared.X.Equal(es.Shared.X) {
					s.storage.Unlock()
					log.Error("the reshared public point is different")
					return
				}
			} else {
				es = &storageElement{}
			}
			// TODO: what happens with Polys here?
			es.Roster = cfg.NewRoster
			es.Shared = *shared
			es.DKS = *dks

			s.storage.Unlock()
			err = s.save()
			if err != nil {
				log.Error("Couldn't save storage")
			}
			if s.afterReshare != nil {
				s.afterReshare()
			}
		}()
		return setupDKG, nil

	case NameOCS:
		id := string(conf.Data)
		s.storage.Lock()
		es, ok := s.storage.Element[id]
		s.storage.Unlock()
		if !ok {
			return nil, fmt.Errorf("didn't find OCSID %v", id)
		}
		pi, err := NewOCS(tn)
		if err != nil {
			return nil, err
		}
		ocs := pi.(*OCS)
		ocs.Shared = es.Shared.Clone()
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
func (s *Service) verifyReencryption(rc *MessageReencrypt) bool {
	// TODO: check the correct authentication
	err := func() error {
		if rc.VerificationData == nil {
			return errors.New("need verification data")
		}
		var arc AuthReencrypt
		err := protobuf.DecodeWithConstructors(*rc.VerificationData, &arc, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return certs.Erret(err)
		}
		Xc, err := arc.Xc()
		if err != nil {
			return certs.Erret(err)
		}
		if !Xc.Equal(rc.Xc) {
			return errors.New("xcs don't match up")
		}
		return nil
	}()
	if err != nil {
		log.Error(err)
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
	if err := s.RegisterHandlers(s.CreateOCS, s.ReshareLTS, s.Reencrypt,
		s.GetProof, s.AddPolicyCreateOCS); err != nil {
		return nil, errors.New("couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
