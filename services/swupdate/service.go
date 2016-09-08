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
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

// ServiceName is the name to refer to the CoSi service
const ServiceName = "Swupdate"

var swupdateService sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceName, newSwupdate)
	swupdateService = sda.ServiceFactory.ServiceID(ServiceName)
	network.RegisterPacketType(&storage{})
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
	Policy *Policy
	Root   *skipchain.SkipBlock
	Data   *skipchain.SkipBlock
}

// CreateProject is the starting point of the software-update and will
// - initialize the skipchains
// - return an id of how this project can be referred to
func (cs *Service) CreateProject(e *network.ServerIdentity, cp *CreateProject) (network.Body, error) {
	log.Lvlf3("%s Creating project %x", cs, cp.Policy)
	ids := &storage{
		Policy: cp.Policy,
	}
	log.Lvl3("Creating Root-skipchain")
	var err error
	ids.Root, err = cs.skipchain.CreateRoster(cp.Roster, 2, 10,
		skipchain.VerifyNone, nil)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Creating Data-skipchain")
	ids.Root, ids.Data, err = cs.skipchain.CreateData(ids.Root, 2, 10,
		skipchain.VerifyNone, cp.Policy)
	if err != nil {
		return nil, err
	}

	//roster := ids.Root.Roster
	//replies, err := manage.PropagateStartAndWait(cs.Context, roster,
	//	&PropagateIdentity{ids}, 1000, cs.Propagate)
	//if err != nil {
	//	return nil, err
	//}
	//if replies != len(roster.List) {
	//	log.Warn("Did only get", replies, "out of", len(roster.List))
	//}
	//log.Lvlf2("New chain is\n%x", []byte(ids.Data.Hash))
	cs.save()

	return &CreateProjectRet{
		ProjectID: ProjectID(ids.Data.Hash),
	}, nil
}

// SignatureRequest treats external request to this service.
func (cs *Service) SignMsg(e *network.ServerIdentity, sm *SignBuild) (network.Body, error) {
	return nil, nil
}

// NewProtocol will instantiate a new protocol if needed.
func (cs *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
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
		log.Lvl3("Successfully loaded")
		s.StorageMap = msg.(*storage)
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
		StorageMap:       &storage{},
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	err := s.RegisterMessages(s.SignMsg)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}
	return s
}
