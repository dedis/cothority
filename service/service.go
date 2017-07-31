package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"errors"

	"bytes"

	"github.com/dedis/onchain-secrets"
	"gopkg.in/dedis/cothority.v1/messaging"
	"gopkg.in/dedis/cothority.v1/skipchain"
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
	templateID, err = onet.RegisterNewService(onchain_secrets.ServiceName, newService)
	log.ErrFatal(err)
}

// Service holds all data for the onchain-secrets service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	propagateACL messaging.PropagationFunc
	propagateDoc messaging.PropagationFunc

	Storage *Storage
}

// Storage holds the skipblock-bunches for the ACL- and Doc-skipchains.
type Storage struct {
	ACLs *onchain_secrets.SBBStorage
	Docs *onchain_secrets.SBBStorage
}

// CreateSkipchains sets up a new pair of ACL/Doc-skipchain.
func (s *Service) CreateSkipchains(req *onchain_secrets.CreateSkipchainsRequest) (reply *onchain_secrets.CreateSkipchainsReply,
	cerr onet.ClientError) {
	log.Lvl2("Creating ACL-skipchain")

	c := skipchain.NewClient()
	reply = &onchain_secrets.CreateSkipchainsReply{}
	reply.ACL, cerr = c.CreateGenesis(req.Roster, 4, 4, onchain_secrets.VerificationOCSACL, req.ACL, nil)
	if cerr != nil {
		return
	}

	log.Lvl2("Creating Doc-skipchain")
	docData := &onchain_secrets.DataOCS{
		Config: &onchain_secrets.DataOCSConfig{
			ACL: reply.ACL.SkipChainID(),
		},
	}
	reply.Doc, cerr = c.CreateGenesis(req.Roster, 4, 4, onchain_secrets.VerificationOCSDoc, docData, nil)
	replies, err := s.propagateACL(req.Roster, reply.ACL, propagationTimeout)
	if err != nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorProtocol, err.Error())
	}
	if replies != len(req.Roster.List) {
		log.Warn("Got only", replies, "replies for acl-propagation")
	}
	replies, err = s.propagateDoc(req.Roster, reply.Doc, propagationTimeout)
	if err != nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorProtocol, err.Error())
	}
	if replies != len(req.Roster.List) {
		log.Warn("Got only", replies, "replies for doc-propagation")
	}
	s.save()
	return
}

// EvolveACL adds a new block to the ACL-skipchain.
func (s *Service) EvolveACL(req *onchain_secrets.EvolveACLRequest) (reply *onchain_secrets.EvolveACLReply,
	cerr onet.ClientError) {
	log.Lvl2("Evolving ACL")
	reply = &onchain_secrets.EvolveACLReply{}
	bunch := s.Storage.ACLs.GetBunch(req.ACL)
	if bunch == nil {
		cerr = onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "Didn't find acl")
		return
	}
	reply.SB, cerr = onchain_secrets.NewClient().BunchAddBlock(bunch, nil, req.NewAcls)
	if cerr != nil {
		return
	}

	replies, err := s.propagateACL(bunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(onchain_secrets.ErrorProtocol, err.Error())
		return
	}
	if replies != len(bunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for acl-propagation")
	}
	return
}

// WriteRequest adds a block the Doc-skipchain with a new file.
func (s *Service) WriteRequest(req *onchain_secrets.WriteRequest) (reply *onchain_secrets.WriteReply,
	cerr onet.ClientError) {
	log.Lvl2("Writing a file to the skipchain")
	reply = &onchain_secrets.WriteReply{}
	docBunch := s.Storage.Docs.GetBunch(req.Doc)
	doc := docBunch.GetByID(req.Doc)
	if doc == nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "Didn't find doc-skipchain")
	}
	data := &onchain_secrets.DataOCS{
		Write: req.Write,
	}
	reply.SB, cerr = onchain_secrets.NewClient().BunchAddBlock(docBunch, doc.Roster, data)
	if cerr != nil {
		return
	}

	replies, err := s.propagateDoc(docBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(onchain_secrets.ErrorProtocol, err.Error())
		return
	}
	if replies != len(docBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// ReadRequest asks for a read-offer on the skipchain for a reader on a file.
func (s *Service) ReadRequest(req *onchain_secrets.ReadRequest) (reply *onchain_secrets.ReadReply,
	cerr onet.ClientError) {
	log.Lvl2("Requesting a file. Reader:", req.Read.Pseudonym)
	reply = &onchain_secrets.ReadReply{}
	docBunch := s.Storage.Docs.GetBunch(req.Doc)
	doc := docBunch.GetByID(req.Doc)
	if doc == nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "Didn't find doc-skipchain")
	}
	data := &onchain_secrets.DataOCS{
		Read: req.Read,
	}
	reply.SB, cerr = onchain_secrets.NewClient().BunchAddBlock(docBunch, doc.Roster, data)
	if cerr != nil {
		return
	}

	replies, err := s.propagateDoc(docBunch.Latest.Roster, reply.SB, propagationTimeout)
	if err != nil {
		cerr = onet.NewClientErrorCode(onchain_secrets.ErrorProtocol, err.Error())
		return
	}
	if replies != len(docBunch.Latest.Roster.List) {
		log.Warn("Got only", replies, "replies for write-propagation")
	}
	return
}

// GetReadRequests returns up to a maximum number of read-requests.
func (s *Service) GetReadRequests(req *onchain_secrets.GetReadRequests) (reply *onchain_secrets.GetReadRequestsReply, cerr onet.ClientError) {
	reply = &onchain_secrets.GetReadRequestsReply{}
	current := s.Storage.Docs.GetByID(req.Start)
	if current == nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "didn't find starting skipblock")
	}
	for len(reply.Documents) < req.Count {
		// Search next read-request
		_, ddoci, err := network.Unmarshal(current.Data)
		if err == nil && ddoci != nil {
			ddoc, ok := ddoci.(*onchain_secrets.DataOCS)
			if !ok {
				return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter,
					"unknown block in doc-skipchain")
			}
			if ddoc.Read != nil {
				doc := &onchain_secrets.ReadDoc{
					Reader: ddoc.Read.Pseudonym,
					ReadID: current.Hash,
					FileID: ddoc.Read.File,
				}
				reply.Documents = append(reply.Documents, doc)
			}
		}
		if len(current.ForwardLink) > 0 {
			current = s.Storage.Docs.GetFromGenesisByID(current.SkipChainID(),
				current.ForwardLink[0].Hash)
		} else {
			log.Lvl3("No forward-links, stopping")
			break
		}
	}
	log.Lvl3("Found", len(reply.Documents), "out of a maximum of", req.Count, "documents.")
	return
}

// EncryptKeyRequest - TODO: Return the public secret key for encryption
// The returned value should be something that the client can use to encrypt his
// key, so that a reader can use DecryptKeyRequest in order to get the key
// encrypted under the reader's keypair.
func (s *Service) EncryptKeyRequest(req *onchain_secrets.EncryptKeyRequest) (reply *onchain_secrets.EncryptKeyReply,
	cerr onet.ClientError) {
	reply = &onchain_secrets.EncryptKeyReply{}
	log.Lvl2("Return the public shared secret")
	return
}

// DecryptKeyRequest - TODO: Re-encrypt under the public key of the reader
// This should return the key encrypted under the public-key of the reader, so
// that the reader can use his private key to decrypt the file-key.
func (s *Service) DecryptKeyRequest(req *onchain_secrets.DecryptKeyRequest) (reply *onchain_secrets.DecryptKeyReply,
	cerr onet.ClientError) {
	reply = &onchain_secrets.DecryptKeyReply{
		KeyParts: []*onchain_secrets.ElGamal{},
	}
	log.Lvl2("Re-encrypt the key to the public key of the reader")

	readSB := s.Storage.Docs.GetByID(req.Read)
	read := onchain_secrets.NewDataOCS(readSB.Data)
	if read == nil || read.Read == nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "This is not a read-block")
	}
	fileSB := s.Storage.Docs.GetByID(read.Read.File)
	file := onchain_secrets.NewDataOCS(fileSB.Data)
	if file == nil || file.Write == nil {
		return nil, onet.NewClientErrorCode(onchain_secrets.ErrorParameter, "File-block is broken")
	}

	// Use multiple Elgamal-encryptions for the key-parts, as they might not
	// fit inside.
	remainder := file.Write.Key
	var eg *onchain_secrets.ElGamal
	for len(remainder) > 0 {
		eg, remainder = onchain_secrets.ElGamalEncrypt(network.Suite, read.Read.Public, remainder)
		reply.KeyParts = append(reply.KeyParts, eg)
	}
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
	acl, ok := aclI.(*onchain_secrets.DataACLEvolve)
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
	prevACL, ok := prevACLI.(*onchain_secrets.DataACLEvolve)
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
func (s *Service) verifyDoc(newID []byte, sb *skipchain.SkipBlock) bool {
	log.Lvl3(s.ServerIdentity(), "Verifying Doc")
	ocs := onchain_secrets.NewDataOCS(sb.Data)
	if ocs == nil {
		log.Lvl3("Didn't find ocs")
		return false
	}
	if ocs.Config != nil {
		// Only accept config in genesis-block
		if sb.Index > 0 {
			log.Lvl3("Config-block in non-genesis block")
			return false
		}
	}
	genesis := s.Storage.Docs.GetFromGenesisByID(sb.SkipChainID(), sb.SkipChainID())
	if genesis == nil {
		log.Lvl3("No genesis-block")
		return false
	}
	ocsData := onchain_secrets.NewDataOCS(genesis.Data)
	if ocsData == nil {
		log.Lvl3("No ocs-data in genesis-block")
		return false
	}

	aclBunch := s.Storage.ACLs.GetBunch(ocsData.Config.ACL)
	if aclBunch == nil {
		log.Lvl3("Didn't find corresponding acl-bunch")
		return false
	}
	_, aclEvolveInt, err := network.Unmarshal(aclBunch.Latest.Data)
	if err != nil {
		log.Error(err)
		return false
	}
	aclEvolve, ok := aclEvolveInt.(*onchain_secrets.DataACLEvolve)
	if !ok {
		log.Lvl3("Didn't find ACL")
		return false
	}
	acl := aclEvolve.ACL
	if write := ocs.Write; write != nil {
		// Write has to check if the signature comes from a valid writer.
		log.Lvl3("It's a write", acl.Writers)
		for _, w := range acl.Writers.List {
			if crypto.VerifySchnorr(network.Suite, w.Public, write.Key, *write.Signature) == nil {
				return true
			}
		}
		return false
	} else if read := ocs.Read; read != nil {
		// Read has to check that it's a valid reader
		log.Lvl3("It's a read")
		// Search file
		found := false
		for _, sb := range s.Storage.Docs.GetBunch(genesis.Hash).SkipBlocks {
			wd := onchain_secrets.NewDataOCS(sb.Data)
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
func (s *Service) propagateDocFunc(sbI network.Message) {
	sb := sbI.(*skipchain.SkipBlock)
	s.Storage.Docs.Store(sb)
	if sb.Index == 0 {
		return
	}
	c := skipchain.NewClient()
	for _, sbID := range sb.BackLinkIDs {
		sbNew, cerr := c.GetSingleBlock(sb.Roster, sbID)
		if cerr != nil {
			log.Error(cerr)
		} else {
			s.Storage.Docs.Store(sbNew)
		}
	}
}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the configuration on our own from time to time.
func newService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Storage: &Storage{
			ACLs: onchain_secrets.NewSBBStorage(),
			Docs: onchain_secrets.NewSBBStorage(),
		},
	}
	if err := s.RegisterHandlers(s.CreateSkipchains, s.EvolveACL,
		s.WriteRequest, s.ReadRequest, s.GetReadRequests,
		s.EncryptKeyRequest, s.DecryptKeyRequest); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	skipchain.RegisterVerification(c, onchain_secrets.VerifyOCSACL, s.verifyACL)
	skipchain.RegisterVerification(c, onchain_secrets.VerifyOCSDoc, s.verifyDoc)
	var err error
	s.propagateACL, err = messaging.NewPropagationFunc(c, "OCSPropagateAcl", s.propagateACLFunc)
	log.ErrFatal(err)
	s.propagateDoc, err = messaging.NewPropagationFunc(c, "OCSPropagateDoc", s.propagateDocFunc)
	log.ErrFatal(err)
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	return s
}
