package ots

import (
	"sync"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/ots/protocol"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

var otsID onet.ServiceID

const ServiceName = "OTS"

func init() {
	var err error
	otsID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})
	err = byzcoin.RegisterGlobalContract(ContractOTSWriteID,
		contractOTSWriteFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractOTSReadID,
		contractOTSReadFromBytes)
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

func (s *Service) DecryptKey(req *DecryptKeyRequest) (*DecryptKeyReply, error) {
	log.Lvl2(s.ServerIdentity(), "Re-encrypt the key to the public key of the reader")

	var read Read
	if err := req.Read.VerifyAndDecode(cothority.Suite, ContractOTSReadID,
		&read); err != nil {
		return nil, xerrors.New("didn't get a read instance: " + err.Error())
	}

	nodes := len(req.Roster.List)
	tree := req.Roster.GenerateNaryTreeWithRoot(nodes, s.ServerIdentity())
	pi, err := s.CreateProtocol(protocol.NameOTS, tree)
	if err != nil {
		return nil, xerrors.Errorf("failed to create the ots-protocol: %v",
			err)
	}
	otsProto := pi.(*protocol.OTS)
	otsProto.Xc = read.Xc
	verificationData := &vData{
		Read:  &req.Read,
		Write: &req.Write,
	}
	otsProto.VerificationData, err = protobuf.Encode(verificationData)
	if err != nil {
		return nil, xerrors.Errorf("couldn't marshall verification data: %v",
			err)
	}
	otsProto.Verify = s.verifyReencryption
	err = otsProto.Start()
	if err != nil {
		return nil, xerrors.Errorf("failed to start ots-protocol: %v", err)
	}
	if !<-otsProto.Reencrypted {
		return nil, xerrors.New("reencryption got refused")
	}
	log.Lvl3("Reencryption protocol is done.")
	return &DecryptKeyReply{Reencryptions: otsProto.Reencryptions}, nil
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

func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), tn.ProtocolName(), conf)
	switch tn.ProtocolName() {
	case protocol.NameOTS:
		pi, err := protocol.NewOTS(tn)
		if err != nil {
			return nil, xerrors.Errorf("creating OTS protocol instance: %v",
				err)
		}
		ots := pi.(*protocol.OTS)
		ots.Verify = s.verifyReencryption
		return ots, nil
	}
	return nil, nil
}

func (s *Service) verifyReencryption(rc *protocol.Reencrypt,
	idx int) (*pvss.PubVerShare, kyber.Point, darc.ID) {
	sh, pr, pid, err := func() (*pvss.PubVerShare, kyber.Point, darc.ID,
		error) {
		var verificationData vData
		err := protobuf.DecodeWithConstructors(*rc.VerificationData,
			&verificationData, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, nil, xerrors.Errorf(
				"decoding verification data: %v", err)
		}
		if err = s.verifyProof(verificationData.Read); err != nil {
			return nil, nil, nil, xerrors.Errorf(
				"read proof cannot be verified to come from scID: %v",
				err)
		}
		if err = s.verifyProof(verificationData.Write); err != nil {
			return nil, nil, nil, xerrors.Errorf(
				"write proof cannot be verified to come from scID: %v",
				err)
		}
		var read Read
		if err := verificationData.Read.VerifyAndDecode(cothority.Suite,
			ContractOTSReadID, &read); err != nil {
			return nil, nil, nil, xerrors.New("didn't get a read instance: " + err.Error())
		}
		var write Write
		if err := verificationData.Write.VerifyAndDecode(cothority.Suite,
			ContractOTSWriteID, &write); err != nil {
			return nil, nil, nil, xerrors.New("didn't get a write instance: " + err.Error())
		}
		if !read.Write.Equal(byzcoin.NewInstanceID(verificationData.Write.
			InclusionProof.Key())) {
			return nil, nil, nil, xerrors.New("read doesn't point to passed write")
		}
		if !read.Xc.Equal(rc.Xc) {
			return nil, nil, nil, xerrors.New("wrong reader")
		}
		return write.Shares[idx], write.Proofs[idx], write.PolicyID, nil
	}()
	if err != nil {
		log.Lvl2(s.ServerIdentity(), "Verifying reencryption failed:", err)
	}
	return sh, pr, pid
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		genesisBlocks:    make(map[string]*skipchain.SkipBlock),
	}
	if err := s.RegisterHandlers(s.DecryptKey); err != nil {
		return nil, xerrors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}
	return s, nil
}
