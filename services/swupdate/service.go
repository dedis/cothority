package swupdate

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"sync"

	"time"

	"errors"

	"os/exec"

	"strings"

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
	path      string
	skipchain *skipchain.Client
	Storage   *storage
	tsChannel chan string
	sync.Mutex
}

type storage struct {
	// A timestamp over all skipchains where all skipblocks are
	// included in a Merkle-tree.
	Timestamp         *Timestamp
	SwupChainsGenesis map[string]*SwupChain
	SwupChains        map[string]*SwupChain
	Root              *skipchain.SkipBlock
	TSInterval        time.Duration
}

// CreateProject is the starting point of the software-update and will
// - initialize the skipchains
// - return an id of how this project can be referred to
func (cs *Service) CreatePackage(si *network.ServerIdentity, cp *CreatePackage) (network.Body, error) {
	policy := cp.Release.Policy
	log.Lvlf3("%s Creating package %s version %s", cs,
		policy.Name, policy.Version)
	sc := &SwupChain{
		Release: cp.Release,
		Root:    cs.Storage.Root,
	}
	if cs.Storage.Root == nil {
		log.Lvl3("Creating Root-skipchain")
		var err error
		cs.Storage.Root, err = cs.skipchain.CreateRoster(cp.Roster, cp.Base, cp.Height,
			skipchain.VerifyNone, nil)
		if err != nil {
			return nil, err
		}
		sc.Root = cs.Storage.Root
	}
	log.Lvl3("Creating Data-skipchain")
	var err error
	sc.Root, sc.Data, err = cs.skipchain.CreateData(sc.Root, 2, 10,
		verifierID, cp.Release)
	if err != nil {
		return nil, err
	}
	cs.Storage.SwupChainsGenesis[policy.Name] = sc
	cs.Storage.SwupChains[policy.Name] = sc
	cs.save()

	return &CreatePackageRet{sc}, nil
}

// SignatureRequest treats external request to this service.
func (cs *Service) UpdatePackage(si *network.ServerIdentity, up *UpdatePackage) (network.Body, error) {
	sc := &SwupChain{
		Release: up.Release,
	}
	rel := up.Release
	log.Lvl3("Creating Data-skipchain")
	var err error
	sc.Root, sc.Data, err = cs.skipchain.CreateData(up.SwupChain.Root, 2, 10,
		verifierID, rel)
	if err != nil {
		return nil, err
	}
	cs.Storage.SwupChains[rel.Policy.Name] = sc
	cs.save()

	return &UpdatePackageRet{sc}, nil
}

// PackageSC searches for the skipchain containing the package. If it finds a
// skipchain, it returns the first and the last block. If no skipchain for
// that package is found, it returns nil for the first and last block.
func (cs *Service) PackageSC(si *network.ServerIdentity, psc *PackageSC) (network.Body, error) {
	sc, ok := cs.Storage.SwupChains[psc.PackageName]
	if !ok {
		return nil, errors.New("Does not exist.")
	}
	lbRet, err := cs.LatestBlock(nil, &LatestBlock{sc.Data.Hash})
	if err != nil {
		return nil, err
	}
	update := lbRet.(*LatestBlockRet).Update
	return &PackageSCRet{
		First: cs.Storage.SwupChainsGenesis[psc.PackageName].Data,
		Last:  update[len(update)-1],
	}, nil
}

// LatestBlock returns the hash of the latest block together with a timestamp
// signed by all nodes of the swupdate-skipchain responsible for that package.
func (cs *Service) LatestBlock(si *network.ServerIdentity, lb *LatestBlock) (network.Body, error) {
	gucRet, err := cs.skipchain.GetUpdateChain(cs.Storage.Root, lb.LastKnownSB)
	if err != nil {
		return nil, err
	}
	return &LatestBlockRet{nil, gucRet.Update}, nil
}

// NewProtocol will instantiate a new protocol if needed.
func (cs *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}

// timestamper waits for n minutes before asking all nodes to timestamp
// on the latest version of all skipblocks.
// This function only returns when "close" is sent through the
// tsChannel.
func (cs *Service) timestamper() {
	for true {
		select {
		case msg := <-cs.tsChannel:
			switch msg {
			case "close":
				return
			case "update":
			default:
				log.Error("Don't know message", msg)
			}
		case <-time.After(cs.Storage.TSInterval):
			log.Lvl2("Interval is over - timestamping")
		}
		// Start timestamping
	}
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
			log.Lvl2("Wrong signature")
			return false
		}
	}
	ver.Record()
	if release.VerifyBuild {
		build := monitor.NewTimeMeasure("build_" + policy.Name)
		// Verify the reproducible build
		wd, _ := os.Getwd()
		cmd := exec.Command("../../reproducible_builds/crawler.py",
			"cli", policy.Name)
		cmd.Stderr = os.Stderr
		resultB, err := cmd.Output()
		result := string(resultB)
		build.Record()
		if err != nil {
			log.Error("While creating reproducible build:", err, result, wd)
			return false
		}
		log.Lvl2("Build-output is", result)
		pkgbuild := fmt.Sprintf("Failed to build: ['%s']", policy.Name)
		if strings.Index(result, pkgbuild) >= 0 {
			return false
		}
	}
	//log.Print("Congrats, verified")
	return true
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(s.Storage)
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
		if len(msg.(*storage).SwupChains) > 0 {
			log.Lvl3("Successfully loaded")
			s.Storage = msg.(*storage)
		}
	}
	return nil
}

// timestamp creates a merkle tree of all the latests skipblocks of each
// skipchains, run a timestamp protocol and store the results in
// s.latestTimestamps.
func (s *Service) timestamp() error {
	// order all packets and marshal them
	ids := orderedLatestSkipblocksID()
	// create merkle tree + proofs

	// run protocol

	// verify & store signature

}

// orderedLatestSkipblocksID sorts the latests blocks of all skipchains and
// return all ids in an array (array of slice of byte for ease of use with
// merkle tree).
func (s *Service) orderedLatestSkipblocksID() [][]byte {
	keys := make([]string, 0)
	for k := range s.Storage.SwupChains {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ids := make([][]byte, 0)
	for _, v := range s.Storage.SwupChains {
		ids = append(ids, []byte(v))
	}
	return ids
}

// newSwupdate create a new service and tries to load an eventually
// already existing one.
func newSwupdate(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		skipchain:        skipchain.NewClient(),
		Storage: &storage{
			SwupChains:        map[string]*SwupChain{},
			SwupChainsGenesis: map[string]*SwupChain{},
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
