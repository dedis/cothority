package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"errors"

	"bytes"

	"github.com/dedis/cothority/messaging"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/logread"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Used for tests
var templateID onet.ServiceID

const propagationTimeout = 10000

func init() {
	network.RegisterMessage(Storage{})
	var err error
	templateID, err = onet.RegisterNewService(logread.ServiceName, newService)
	log.ErrFatal(err)
}

// Service holds all data for the logread-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	propagateACL messaging.PropagationFunc
	propagateWLR messaging.PropagationFunc

	Storage *Storage
}

// Storage holds the skipblock-bunches for the ACL- and WLR-skipchains.
type Storage struct {
	ACLs *logread.SBBStorage
	WLRs *logread.SBBStorage
}

// CreateSkipchains sets up a new pair of ACL/WLR-skipchain.
func (s *Service) CreateSkipchains(req *logread.CreateSkipchainsRequest) (reply *logread.CreateSkipchainsReply,
	cerr onet.ClientError) {
	log.Lvl3("Creating ACL")

	c := skipchain.NewClient()
	reply = &logread.CreateSkipchainsReply{}
	reply.ACL, cerr = c.CreateGenesis(req.Roster, 4, 4, logread.VerificationLogreadACL, req.ACL, nil)
	if cerr != nil {
		return
	}
	wlrData := &logread.DataWlr{
		Config: &logread.DataWlrConfig{
			ACL: reply.ACL.SkipChainID(),
		},
	}
	reply.Wlr, cerr = c.CreateGenesis(req.Roster, 4, 4, logread.VerificationLogreadWLR, wlrData, nil)
	replies, err := s.propagateACL(req.Roster, reply.ACL, propagationTimeout)
	if err != nil {
		return nil, onet.NewClientErrorCode(logread.ErrorProtocol, err.Error())
	}
	if replies != len(req.Roster.List) {
		log.Warn("Got only", replies, "replies for acl-propagation")
	}
	replies, err = s.propagateWLR(req.Roster, reply.Wlr, propagationTimeout)
	if err != nil {
		return nil, onet.NewClientErrorCode(logread.ErrorProtocol, err.Error())
	}
	if replies != len(req.Roster.List) {
		log.Warn("Got only", replies, "replies for wlr-propagation")
	}
	s.save()
	return
}

// EvolveACL adds a new block to the ACL-skipchain.
func (s *Service) EvolveACL(req *logread.EvolveACLRequest) (reply *logread.EvolveACLReply,
	cerr onet.ClientError) {
	reply = &logread.EvolveACLReply{}
	bunch := s.Storage.ACLs.GetBunch(req.ACL)
	if bunch == nil {
		cerr = onet.NewClientErrorCode(logread.ErrorParameter, "Didn't find acl")
		return
	}
	reply.SB, cerr = logread.NewClient().BunchAddBlock(bunch, nil, req.NewAcls)
	if cerr != nil {
		return
	}

	replies, err := s.propagateACL(bunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(logread.ErrorProtocol, err.Error())
		return
	}
	if replies != len(bunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for acl-propagation")
	}
	return
}

// WriteRequest adds a block the WLR-skipchain with a new file.
func (s *Service) WriteRequest(req *logread.WriteRequest) (reply *logread.WriteReply,
	cerr onet.ClientError) {
	reply = &logread.WriteReply{}
	wlrBunch := s.Storage.WLRs.GetBunch(req.Wlr)
	wlr := wlrBunch.GetByID(req.Wlr)
	if wlr == nil {
		return nil, onet.NewClientErrorCode(logread.ErrorParameter, "Didn't find wlr-skipchain")
	}
	data := &logread.DataWlr{
		Write: req.Write,
	}
	reply.SB, cerr = logread.NewClient().BunchAddBlock(wlrBunch, wlr.Roster, data)
	if cerr != nil {
		return
	}

	replies, err := s.propagateWLR(wlrBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(logread.ErrorProtocol, err.Error())
		return
	}
	if replies != len(wlrBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// ReadRequest asks for a read-offer on the skipchain for a reader on a file.
func (s *Service) ReadRequest(req *logread.ReadRequest) (reply *logread.ReadReply,
	cerr onet.ClientError) {
	reply = &logread.ReadReply{}
	wlrBunch := s.Storage.WLRs.GetBunch(req.Wlr)
	wlr := wlrBunch.GetByID(req.Wlr)
	if wlr == nil {
		return nil, onet.NewClientErrorCode(logread.ErrorParameter, "Didn't find wlr-skipchain")
	}
	data := &logread.DataWlr{
		Read: req.Read,
	}
	reply.SB, cerr = logread.NewClient().BunchAddBlock(wlrBunch, wlr.Roster, data)
	if cerr != nil {
		return
	}

	replies, err := s.propagateWLR(wlrBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(logread.ErrorProtocol, err.Error())
		return
	}
	if replies != len(wlrBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// EncryptKeyRequest - TODO: do something cool here
// The returned value should be something that the client can use to encrypt his
// key, so that a reader can use DecryptKeyRequest in order to get the key
// encrypted under the reader's keypair.
func (s *Service) EncryptKeyRequest(req *logread.EncryptKeyRequest) (reply *logread.EncryptKeyReply,
	cerr onet.ClientError) {
	reply = &logread.EncryptKeyReply{}
	log.Lvl2("Do something funky in here")
	return
}

// DecryptKeyRequest - TODO: do something cool here
// This should return the key encrypted under the public-key of the reader, so
// that the reader can use his private key to decrypt the file-key.
func (s *Service) DecryptKeyRequest(req *logread.DecryptKeyRequest) (reply *logread.DecryptKeyReply,
	cerr onet.ClientError) {
	reply = &logread.DecryptKeyReply{
		Key: []byte{},
	}
	log.Lvl2("Unfunky that stuff")

	readSB := s.Storage.WLRs.GetByID(req.Read)
	read := logread.NewDataWlr(readSB.Data)
	if read == nil || read.Read == nil {
		return nil, onet.NewClientErrorCode(logread.ErrorParameter, "This is not a read-block")
	}
	fileSB := s.Storage.WLRs.GetByID(read.Read.File)
	file := logread.NewDataWlr(fileSB.Data)
	if file == nil || file.Write == nil {
		return nil, onet.NewClientErrorCode(logread.ErrorParameter, "File-block is broken")
	}
	reply.Key = file.Write.Key
	return
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	err := s.Save("storage", s.Storage)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	if !s.DataAvailable("storage") {
		return nil
	}
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	var ok bool
	s.Storage, ok = msg.(*Storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	log.Lvl3("Successfully loaded")
	return nil
}

func (s *Service) verifyACL(newID []byte, sb *skipchain.SkipBlock) bool {
	log.Lvl3(s.ServerIdentity(), "Verifying ACL")
	if sb.Index == 0 {
		log.Lvl3("Genesis-block")
		return true
	}
	_, aclI, err := network.Unmarshal(sb.Data)
	if err != nil {
		log.Lvl3("couldn't unmarshal data")
		return false
	}
	acl, ok := aclI.(*logread.DataACLEvolve)
	if !ok {
		log.Lvl3("Not correct acl")
		return false
	}
	prev := s.Storage.ACLs.GetByID(sb.BackLinkIDs[0])
	if prev == nil {
		log.Lvl3("No valid backlink")
		return false
	}

	_, prevACLI, err := network.Unmarshal(prev.Data)
	if err != nil {
		log.Lvl3("Couldn't unmarshal data")
		return false
	}
	prevACL, ok := prevACLI.(*logread.DataACLEvolve)
	if !ok {
		log.Lvl3("Not correct acl")
		return false
	}
	log.Lvlf3("Found signature %s", acl.Signature)
	for _, a := range prevACL.ACL.Admins.List {
		log.Lvlf3("Verifying admin %s/%s", a.Pseudonym, a.Public)
		if acl.VerifySig(prev, a.Public) == nil {
			log.Lvl3("Found signature by", a.Pseudonym)
			return true
		}
	}
	log.Lvl3("Didn't find correct signature")
	return false
}
func (s *Service) verifyWLR(newID []byte, sb *skipchain.SkipBlock) bool {
	log.Lvl3(s.ServerIdentity(), "Verifying WLR")
	wlr := logread.NewDataWlr(sb.Data)
	if wlr == nil {
		log.Lvl3("Didn't find wlr")
		return false
	}
	if wlr.Config != nil {
		// Only accept config in genesis-block
		if sb.Index > 0 {
			log.Lvl3("Config-block in non-genesis block")
			return false
		}
	}
	genesis := s.Storage.WLRs.GetFromGenesisByID(sb.SkipChainID(), sb.SkipChainID())
	if genesis == nil {
		log.Lvl3("No genesis-block")
		return false
	}
	wlrData := logread.NewDataWlr(genesis.Data)
	if wlrData == nil {
		log.Lvl3("No wlr-data in genesis-block")
		return false
	}

	aclBunch := s.Storage.ACLs.GetBunch(wlrData.Config.ACL)
	if aclBunch == nil {
		log.Lvl3("Didn't find corresponding acl-bunch")
		return false
	}
	_, aclEvolveInt, err := network.Unmarshal(aclBunch.Latest.Data)
	if err != nil {
		log.Error(err)
		return false
	}
	aclEvolve, ok := aclEvolveInt.(*logread.DataACLEvolve)
	if !ok {
		log.Lvl3("Didn't find ACL")
		return false
	}
	acl := aclEvolve.ACL
	if write := wlr.Write; write != nil {
		// Write has to check if the signature comes from a valid writer.
		log.Lvl3("It's a write", acl.Writers)
		for _, w := range acl.Writers.List {
			if crypto.VerifySchnorr(network.Suite, w.Public, write.Key, *write.Signature) == nil {
				return true
			}
		}
		return false
	} else if read := wlr.Read; read != nil {
		// Read has to check that it's a valid reader
		log.Lvl3("It's a read")
		// Search file
		found := false
		for _, sb := range s.Storage.WLRs.GetBunch(genesis.Hash).SkipBlocks {
			wd := logread.NewDataWlr(sb.Data)
			if wd != nil && wd.Write != nil {
				if bytes.Compare(sb.Hash, read.File) == 0 {
					found = true
					break
				}
			}
		}
		if found == false {
			log.Lvl3("Didn't find file")
			return false
		}
		for _, w := range acl.Readers.List {
			if crypto.VerifySchnorr(network.Suite, w.Public, read.File, *read.Signature) == nil {
				log.Lvl3("OK for file")
				return true
			}
		}
		return false
	}
	return false
}

func (s *Service) propagateACLFunc(sbI network.Message) {
	sb := sbI.(*skipchain.SkipBlock)
	s.Storage.ACLs.Store(sb)
}
func (s *Service) propagateWLRFunc(sbI network.Message) {
	sb := sbI.(*skipchain.SkipBlock)
	s.Storage.WLRs.Store(sb)
}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the configuration on our own from time to time.
func newService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Storage: &Storage{
			ACLs: logread.NewSBBStorage(),
			WLRs: logread.NewSBBStorage(),
		},
	}
	if err := s.RegisterHandlers(s.CreateSkipchains, s.EvolveACL,
		s.WriteRequest, s.ReadRequest,
		s.EncryptKeyRequest, s.DecryptKeyRequest); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	skipchain.RegisterVerification(c, logread.VerifyLogreadACL, s.verifyACL)
	skipchain.RegisterVerification(c, logread.VerifyLogreadWLR, s.verifyWLR)
	var err error
	s.propagateACL, err = messaging.NewPropagationFunc(c, "LogreadPropagateAcl", s.propagateACLFunc)
	log.ErrFatal(err)
	s.propagateWLR, err = messaging.NewPropagationFunc(c, "LogreadPropagateWlr", s.propagateWLRFunc)
	log.ErrFatal(err)
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	return s
}
