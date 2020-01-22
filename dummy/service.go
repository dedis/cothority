package dummy

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math"
	"sync"

	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

var pairingSuite = suites.MustFind("bn256.Adapter").(*pairing.SuiteBn256)
var dummyID onet.ServiceID
var storageKey = []byte("dummyStorage")

const ServiceName = "DummyService"
const dummySubFtCosi = "dummy_sub_ftcosi"
const dummyFtCosi = "dummy_ftcosi"

type storage struct {
	Reencryptions map[string]kyber.Point
	sync.Mutex
}

type Service struct {
	*onet.ServiceProcessor
	storage *storage
	//roster  *onet.Roster
}

func init() {
	var err error
	dummyID, err = onet.RegisterNewServiceWithSuite(ServiceName, pairingSuite, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &DummyRequest{}, &DummyReply{})
}

func (s *Service) DummyRequest(req *DummyRequest) (*DummyReply, error) {
	roster := req.Roster
	n := len(roster.List)
	tree := roster.GenerateNaryTreeWithRoot(n, s.ServerIdentity())
	pi, err := s.CreateProtocol(dummyFtCosi, tree)
	if err != nil {
		log.Errorf("Creating protocol failed: %v", err)
		return nil, err
	}
	xhatEnc := s.GetReencryption(req.DKID)
	if xhatEnc == nil {
		return nil, errors.New("Cannot find result for the given DKID")
	}
	dd := &dummyData{DKID: req.DKID, XhatEnc: xhatEnc}
	dataBuf, err := protobuf.Encode(dd)
	if err != nil {
		log.Errorf("Protobuf encode failed: %v", err)
		return nil, err
	}
	msgBuf, err := dd.Hash()
	if err != nil {
		log.Errorf("Cannot compute the hash of point: %v", err)
		return nil, err
	}
	cosiProto := pi.(*protocol.BlsCosi)
	cosiProto.Msg = msgBuf
	cosiProto.Data = dataBuf
	cosiProto.CreateProtocol = s.CreateProtocol
	cosiProto.Threshold = n - (n-1)/3
	err = cosiProto.SetNbrSubTree(int(math.Pow(float64(n), 1.0/3.0)))
	if err != nil {
		log.Errorf("SetNbrSubTree failed: %v", err)
		return nil, err
	}
	err = cosiProto.Start()
	if err != nil {
		log.Errorf("Starting the cosi protocol failed: %v", err)
		return nil, err
	}
	reply := &DummyReply{}
	reply.Signature = <-cosiProto.FinalSignature
	return reply, nil
}

func (s *Service) dummyVerification(msg []byte, data []byte) bool {
	var dd dummyData
	err := protobuf.Decode(data, &dd)
	if err != nil {
		log.Errorf("%s protobuf decode error: %v", s.ServerIdentity(), err)
		return false
	}
	ddHash, err := dd.Hash()
	if err != nil {
		log.Errorf("%s cannot compute the hash of dummy data: %v", s.ServerIdentity(), err)
		return false
	}
	if bytes.Equal(msg, ddHash) {
		log.LLvl3("DKID in dummyverification is:", dd.DKID)
		storedXhatEnc := s.GetReencryption(dd.DKID)
		if storedXhatEnc != nil {
			log.LLvl3(s.ServerIdentity(), "===========> found reencryption")
			if storedXhatEnc.Equal(dd.XhatEnc) {
				return true
			} else {
				log.Errorf("Stored result does not match the result in the DummyRequest")
				return false
			}
		} else {
			log.Errorf("No result found for the given DKID %s", dd.DKID)
			return false
		}
	} else {
		log.Errorf("Hash of dummy data does not match msg")
		return false
	}
}

func (dd *dummyData) Hash() ([]byte, error) {
	ptBuf, err := dd.XhatEnc.MarshalBinary()
	if err != nil {
		return nil, err
	}
	sh := sha256.New()
	sh.Write([]byte(dd.DKID))
	sh.Write(ptBuf)
	return sh.Sum(nil), nil
}

func (s *Service) StoreReencryption(dkid string, xhatEnc kyber.Point) error {
	s.storage.Lock()
	s.storage.Reencryptions[dkid] = xhatEnc
	s.storage.Unlock()
	err := s.save()
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) GetReencryption(id string) kyber.Point {
	s.storage.Lock()
	defer s.storage.Unlock()
	xhatEnc, ok := s.storage.Reencryptions[id]
	if !ok {
		return nil
	}
	xhatEnc = xhatEnc.Clone()
	return xhatEnc
}

func (s *Service) save() error {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageKey, s.storage)
	if err != nil {
		log.Errorf("Could not save data: %v", err)
		return err
	}
	return nil
}

func (s *Service) tryLoad() error {
	s.storage = &storage{}
	// Make sure we don't have any unallocated maps.
	defer func() {
		if len(s.storage.Reencryptions) == 0 {
			s.storage.Reencryptions = make(map[string]kyber.Point)
		}
	}()
	msg, err := s.Load(storageKey)
	if err != nil {
		log.Errorf("Load storage failed: %v", err)
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

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	err := s.RegisterHandlers(s.DummyRequest)
	if err != nil {
		log.Errorf("Cannot register handlers: %v", err)
		return nil, err
	}
	_, err = s.ProtocolRegister(dummySubFtCosi, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, s.dummyVerification, pairingSuite)
	})
	if err != nil {
		log.Errorf("Cannot register protocol %s: %v", dummySubFtCosi, err)
		return nil, err
	}
	_, err = s.ProtocolRegister(dummyFtCosi, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, s.dummyVerification, dummySubFtCosi, pairingSuite)
	})
	if err != nil {
		log.Errorf("Cannot register protocol %s: %v", dummyFtCosi, err)
		return nil, err
	}
	err = s.tryLoad()
	if err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}
	return s, nil
}
