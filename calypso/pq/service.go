package pq

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// Used for tests
var pqOtsID onet.ServiceID

const ServiceName = "PQ-OTS"

func init() {
	var err error
	pqOtsID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})
	err = byzcoin.RegisterGlobalContract(ContractPQWriteID, contractPQWriteFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractReadID, contractReadFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

type Service struct {
	*onet.ServiceProcessor
	storage *storage
}

// vData is sent to all nodes when re-encryption takes place. If Ephemeral
// is non-nil, Signature needs to hold a valid signature from the reader
// in the Proof.
type vData struct {
	Proof     byzcoin.Proof
	Ephemeral kyber.Point
	Signature *darc.Signature
}

func (s *Service) VerifyWrite(req *VerifyWrite) (*VerifyWriteReply,
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

func verifyCommitment(req *VerifyWrite) error {
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

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.VerifyWrite); err != nil {
		return nil, xerrors.New("Couldn't register messages")
	}
	//if err := s.RegisterHandlers(s.CreateLTS, s.ReshareLTS, s.DecryptKey,
	//	s.GetLTSReply, s.Authorise, s.Authorize, s.updateValidPeers); err != nil {
	//	return nil, xerrors.New("couldn't register messages")
	//}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}

	// Initialize the sets of valid peers for all existing LTS
	//for ltsID, roster := range s.storage.Rosters {
	//	s.SetValidPeers(s.NewPeerSetID(ltsID[:]), roster.List)
	//}

	return s, nil
}
