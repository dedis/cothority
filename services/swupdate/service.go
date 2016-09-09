package swupdate

import (
	"fmt"
	"io/ioutil"
	"os"

	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/satori/go.uuid"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

// ServiceName is the name to refer to the CoSi service
const ServiceName = "Swupdate"

var swupdateService sda.ServiceID
var verifierID = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, ServiceName))

func init() {
	sda.RegisterNewService(ServiceName, newSwupdate)
	swupdateService = sda.ServiceFactory.ServiceID(ServiceName)
	network.RegisterPacketType(&storage{})
	skipchain.VerificationRegistration(verifierID, verifierFunc)
}

// Swupdate allows decentralized software-update-signing and verification.
type Service struct {
	*sda.ServiceProcessor
	path       string
	skipchain  *skipchain.Client
	StorageMap *storageMap
	sync.Mutex
}

type storageMap struct {
	Storage map[string]*storage
}

type storage struct {
	*SwupChain
}

// CreateProject is the starting point of the software-update and will
// - initialize the skipchains
// - return an id of how this project can be referred to
func (cs *Service) CreatePackage(si *network.ServerIdentity, cp *CreatePackage) (network.Body, error) {
	policy := cp.Release.Policy
	log.LLvlf3("%s Creating package %s version %s", cs,
		policy.Name, policy.Version)
	sc := &SwupChain{
		Release:   cp.Release,
		Timestamp: &Timestamp{"", []byte{}, ""},
	}
	log.Lvl3("Creating Root-skipchain")
	var err error
	sc.Root, err = cs.skipchain.CreateRoster(cp.Roster, cp.Base, cp.Height,
		skipchain.VerifyNone, nil)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Creating Data-skipchain")
	sc.Root, sc.Data, err = cs.skipchain.CreateData(sc.Root, 2, 10,
		verifierID, cp.Release)
	if err != nil {
		return nil, err
	}
	cs.StorageMap.Storage[policy.Name] =
		&storage{sc}
	cs.save()

	return &CreatePackageRet{sc}, nil
}

// SignatureRequest treats external request to this service.
func (cs *Service) UpdatePackage(si *network.ServerIdentity, up *UpdatePackage) (network.Body, error) {
	return nil, nil
}

// NewProtocol will instantiate a new protocol if needed.
func (cs *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}

// verifierFunc will return whether the block is valid.
func verifierFunc(msg, data []byte) bool {
	_, sbBuf, err := network.UnmarshalRegistered(data)
	sb, ok := sbBuf.(*skipchain.SkipBlock)
	if err != nil || !ok {
		log.Error(err, ok)
		return false
	}
	_, relBuf, err := network.UnmarshalRegistered(sb.Data)
	release, ok := relBuf.(*Release)
	if err != nil || !ok {
		log.Error(err, ok)
		return false
	}
	policy := release.Policy
	policyBin, err := network.MarshalRegisteredType(policy)
	if err != nil {
		log.Error(err)
		return false
	}
	log.Printf("Verifying release %s/%s", policy.Name, policy.Version)
	for i, s := range release.Signatures {
		err := NewPGPPublic(policy.Keys[i]).Verify(
			policyBin, s)
		if err != nil {
			log.Print("Wrong signature")
			return false
		}
	}
	log.Print("Congrats, verified")
	return true
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(s.StorageMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(s.path+"/swupdate.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	configFile := s.path + "/swupdate.bin"
	b, err := ioutil.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Error while reading %s: %s", configFile, err)
	}
	if len(b) > 0 {
		_, msg, err := network.UnmarshalRegistered(b)
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal: %s", err)
		}
		// Only overwrite storage if we have a content,
		// else keep the pre-defined storage-map.
		if len(msg.(*storageMap).Storage) > 0 {
			log.LLvl3("Successfully loaded")
			s.StorageMap = msg.(*storageMap)
		}
	}
	return nil
}

// newSwupdate create a new service and tries to load an eventually
// already existing one.
func newSwupdate(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		skipchain:        skipchain.NewClient(),
		StorageMap:       &storageMap{map[string]*storage{}},
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	err := s.RegisterMessages(s.CreatePackage, s.UpdatePackage)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message")
	}
	return s
}
