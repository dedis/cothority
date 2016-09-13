package swupdate

import (
	"fmt"
	"io/ioutil"
	"os"

	"sync"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
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
	// Timestamps of all known skipblocks, indexed by the
	// skipchain-ID (which is the hash of the genesis-skipblock).
	Storage    map[string]*storage
	Timestamps map[string]*Timestamp
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
		Timestamp: &Timestamp{},
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
	sc := &SwupChain{
		Release:   up.Release,
		Timestamp: &Timestamp{},
	}
	rel := up.Release
	log.Lvl3("Creating Data-skipchain")
	var err error
	sc.Root, sc.Data, err = cs.skipchain.CreateData(up.SwupChain.Root, 2, 10,
		verifierID, rel)
	if err != nil {
		return nil, err
	}
	cs.StorageMap.Storage[rel.Policy.Name] = &storage{sc}
	cs.save()

	return &UpdatePackageRet{sc}, nil
}

// PackageSC searches for the skipchain containing the package. If it finds a
// skipchain, it returns the first and the last block. If no skipchain for
// that package is found, it returns nil for the first and last block.
func (cs *Service) PackageSC(si *network.ServerIdentity, psc *PackageSC) (network.Body, error) {
	return &PackageSCRet{}, nil
}

// LatestBlock returns the hash of the latest block together with a timestamp
// signed by all nodes of the swupdate-skipchain responsible for that package.
func (cs *Service) LatestBlock(si *network.ServerIdentity, lb *LatestBlock) (network.Body, error) {
	return &LatestBlockRet{}, nil
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
	ver := monitor.NewTimeMeasure("verification_" + policy.Name)
	//log.Printf("Verifying release %s/%s", policy.Name, policy.Version)
	for i, s := range release.Signatures {
		err := NewPGPPublic(policy.Keys[i]).Verify(
			policyBin, s)
		if err != nil {
			log.Error("Wrong signature")
			return false
		}
	}
	ver.Record()
	build := monitor.NewTimeMeasure("build_" + policy.Name)
	// Verify the reproducible build
	time.Sleep(1 * time.Second)
	build.Record()
	//log.Print("Congrats, verified")
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
		StorageMap: &storageMap{
			Storage:    map[string]*storage{},
			Timestamps: map[string]*Timestamp{},
		},
	}
	//if err := s.tryLoad(); err != nil {
	//	log.Error(err)
	//}
	err := s.RegisterMessages(s.CreatePackage, s.UpdatePackage)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message")
	}
	return s
}
