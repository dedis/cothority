package pqots

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/pqots/protocol"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"sync"
)

var pqOtsID onet.ServiceID

const ServiceName = "PQOTS"

func init() {
	var err error
	pqOtsID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})
	err = byzcoin.RegisterGlobalContract(ContractPQOTSWriteID, contractPQOTSWriteFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractPQOTSReadID, contractPQOTSReadFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

type Service struct {
	*onet.ServiceProcessor
	storage           *storage
	genesisBlocks     map[string]*skipchain.SkipBlock
	genesisBlocksLock sync.Mutex
}

// vData is sent to all nodes when re-encryption takes place.
type vData struct {
	Read  *byzcoin.Proof
	Write *byzcoin.Proof
}

func (s *Service) VerifyWrite(req *VerifyWriteRequest) (*VerifyWriteReply,
	error) {
	err := verifyCommitment(req)
	if err != nil {
		return nil, err
	}
	wb, err := protobuf.Encode(req.Write)
	if err != nil {
		return nil, xerrors.Errorf("Cannot sign write txn: %v", err)
	}
	h := sha256.New()
	h.Write(wb)
	buf := h.Sum(nil)
	sig, err := schnorr.Sign(cothority.Suite, s.ServerIdentity().GetPrivate(), buf)
	if err != nil {
		return nil, err
	}
	s.storage.Lock()
	s.storage.Shares[hex.EncodeToString(buf)] = req.Share
	s.storage.Unlock()
	return &VerifyWriteReply{Sig: sig}, nil
}

func verifyCommitment(req *VerifyWriteRequest) error {
	cmt := req.Write.Commitments[req.Idx]
	shb, err := req.Share.V.MarshalBinary()
	if err != nil {
		return xerrors.Errorf("Cannot verify commitment: %v", err)
	}
	h := sha256.New()
	h.Write(shb)
	h.Write(req.Rand)
	tmpCmt := h.Sum(nil)
	if !bytes.Equal(cmt, tmpCmt) {
		return xerrors.New("Commitments do not match")
	}
	return nil
}

func (s *Service) DecryptKey(req *DecryptKeyRequest) (*DecryptKeyReply, error) {
	log.Lvl2(s.ServerIdentity(), "Re-encrypt the key to the public key of the reader")

	var read Read
	if err := req.Read.VerifyAndDecode(cothority.Suite, ContractPQOTSReadID,
		&read); err != nil {
		return nil, xerrors.New("didn't get a read instance: " + err.Error())
	}

	nodes := len(req.Roster.List)
	tree := req.Roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NamePQOTS, tree)
	if err != nil {
		return nil, xerrors.Errorf("failed to create the pqots-protocol: %v",
			err)
	}
	pqotsProto := pi.(*protocol.PQOTS)
	pqotsProto.Xc = read.Xc
	verificationData := &vData{
		Read:  &req.Read,
		Write: &req.Write,
	}
	pqotsProto.VerificationData, err = protobuf.Encode(verificationData)
	if err != nil {
		return nil, xerrors.Errorf("couldn't marshall verification data: %v",
			err)
	}
	pqotsProto.Verify = s.verifyReencryption
	err = pqotsProto.Start()
	if err != nil {
		return nil, xerrors.Errorf("failed to start pqots-protocol: %v", err)
	}
	if !<-pqotsProto.Reencrypted {
		return nil, xerrors.New("reencryption got refused")
	}
	log.Lvl3("Reencryption protocol is done.")
	return &DecryptKeyReply{Reencryptions: pqotsProto.Reencryptions}, nil
}

func (s *Service) verifyProof(proof *byzcoin.Proof) error {
	scID := proof.Latest.SkipChainID()
	sb, err := s.fetchGenesisBlock(scID, proof.Latest.Roster)
	if err != nil {
		return xerrors.Errorf("fetching genesis block: %v", err)
	}

	return cothority.ErrorOrNil(proof.VerifyFromBlock(sb),
		"verifying proof from block")
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
		return nil, xerrors.Errorf("getting single block: %v", err)
	}

	// Genesis block can be reused later on.
	s.genesisBlocks[string(scID)] = sb

	return sb, nil
}

func (s *Service) NewProtocol(tn *onet.TreeNodeInstance,
	conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case protocol.NamePQOTS:
		pi, err := protocol.NewPQOTS(tn)
		if err != nil {
			return nil, xerrors.Errorf(
				"creating PQOTS protocol instance: %v", err)
		}
		pqOts := pi.(*protocol.PQOTS)
		pqOts.Verify = s.verifyReencryption
		return pqOts, nil
	}
	return nil, nil
}

func (s *Service) verifyReencryption(rc *protocol.Reencrypt) *share.PriShare {
	sh, err := func() (*share.PriShare, error) {
		var verificationData vData
		err := protobuf.DecodeWithConstructors(*rc.VerificationData,
			&verificationData, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, xerrors.Errorf("decoding verification data: %v", err)
		}
		if err = s.verifyProof(verificationData.Read); err != nil {
			return nil, xerrors.Errorf(
				"read proof cannot be verified to come from scID: %v",
				err)
		}
		if err = s.verifyProof(verificationData.Write); err != nil {
			return nil, xerrors.Errorf(
				"write proof cannot be verified to come from scID: %v",
				err)
		}
		var read Read
		if err := verificationData.Read.VerifyAndDecode(cothority.Suite,
			ContractPQOTSReadID, &read); err != nil {
			return nil, xerrors.New("didn't get a read instance: " + err.Error())
		}
		var write Write
		if err := verificationData.Write.VerifyAndDecode(cothority.Suite,
			ContractPQOTSWriteID, &write); err != nil {
			return nil, xerrors.New("didn't get a write instance: " + err.Error())
		}
		if !read.Write.Equal(byzcoin.NewInstanceID(verificationData.Write.
			InclusionProof.Key())) {
			return nil, xerrors.New("read doesn't point to passed write")
		}
		if !read.Xc.Equal(rc.Xc) {
			return nil, xerrors.New("wrong reader")
		}
		// Get the encrypted share from storage
		wb, err := protobuf.Encode(&write)
		if err != nil {
			return nil, xerrors.Errorf("cannot encode write: %v", err)
		}
		h := sha256.New()
		h.Write(wb)
		key := hex.EncodeToString(h.Sum(nil))
		s.storage.Lock()
		defer s.storage.Unlock()
		sh, ok := s.storage.Shares[key]
		if !ok {
			return nil, xerrors.Errorf("could not find the share for key %v", key)
		}
		return sh, nil
	}()
	if err != nil {
		log.Lvl2(s.ServerIdentity(), "Verifying reencryption failed:", err)
	}
	return sh
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		genesisBlocks:    make(map[string]*skipchain.SkipBlock),
	}
	if err := s.RegisterHandlers(s.VerifyWrite, s.DecryptKey); err != nil {
		return nil, xerrors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}
	return s, nil
}
