package ca

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	//"reflect"
	//"bytes"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"sync"
	//"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
)

// ServiceName can be used to refer to the name of this service
const ServiceCAName = "CA"

var CAService sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceCAName, newCAService)
	CAService = sda.ServiceFactory.ServiceID(ServiceCAName)
	network.RegisterPacketType(&SiteMap{})
	network.RegisterPacketType(&Site{})
}

// CA handles per site certificates
type CA struct {
	*sda.ServiceProcessor
	*SiteMap
	// Private key for that CA
	Private abstract.Scalar
	// Public key for that CA
	Public     abstract.Point
	sitesMutex sync.Mutex
	path       string
}

// SiteMap holds the map to the sites so it can be marshaled.
type SiteMap struct {
	Sites map[string]*Site
}

// Site stores one site identity together with its cert.
type Site struct {
	sync.Mutex
	// Site's ID (hash of the genesis block)
	ID skipchain.SkipBlockID
	// The Config corresponding to the Cert
	Config *common_structs.Config
	Cert   *Cert
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (ca *CA) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3(ca.ServerIdentity(), "CA received New Protocol event", conf)
	tn.ProtocolName()

	return nil, nil
}

/*
 * API messages
 */

// SignCert will use the CA's public key to sign a new cert
func (ca *CA) SignCert(si *network.ServerIdentity, csr *CSR) (network.Body, error) {
	log.Lvlf2("SignCert(): Start")
	id := csr.ID
	config := csr.Config
	hash, _ := config.Hash()
	if config == nil {
		log.Lvlf2("Nil config")
	}
	//log.Lvlf2("ID: %v, Hash: %v", id, hash)
	// Check that the Config part of the CSR was signed by a threshold of the containing devices
	cnt := 0
	for _, dev := range config.Device {
		public := dev.Point
		sig := dev.Vote
		if sig != nil {
			err := crypto.VerifySchnorr(network.Suite, public, hash, *sig)
			if err != nil {
				return nil, errors.New("Wrong signature")
			}
			cnt++
		}
	}
	if cnt < config.Threshold {
		log.Lvlf2("Not enough valid signatures")
		return nil, errors.New("Not enough valid signatures")
	}

	// Check whether our clock is relatively close or not to the proposed timestamp
	err := config.CheckTimeDiff(maxdiff_sign)
	if err != nil {
		log.Lvlf2("CA with public key: %v %v", ca.Public, err)
		return nil, err
	}

	// Sign the config's hash using CA's private key
	var signature crypto.SchnorrSig
	//log.Lvlf2("SignCert(): before signing: CApublic: %v", ca.Public)
	signature, err = crypto.SignSchnorr(network.Suite, ca.Private, hash)
	if err != nil {
		return nil, err
	}
	//log.Lvlf2("SignCert(): 3")

	cert := &Cert{
		ID:        id,
		Hash:      hash,
		Signature: &signature,
		Public:    ca.Public,
	}

	site := &Site{
		ID:     id,
		Config: config,
		Cert:   cert,
	}

	ca.setSiteStorage(id, site)
	//log.Lvlf2("SignCert(): End with ID: %v, Hash: %v, Sig: %v, Public: %v", id, hash, signature, ca.Public)
	return &CSRReply{
		Cert: cert,
	}, nil
}

func (ca *CA) GetPublicKey(si *network.ServerIdentity, req *GetPublicKey) (network.Body, error) {
	return &GetPublicKeyReply{Public: ca.Public}, nil
}

func (ca *CA) clearSites() {
	ca.Sites = make(map[string]*Site)
}

func (ca *CA) getSiteStorage(id skipchain.SkipBlockID) *Site {
	ca.sitesMutex.Lock()
	defer ca.sitesMutex.Unlock()
	is, ok := ca.Sites[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setSiteStorage saves a SiteStorage
func (ca *CA) setSiteStorage(id skipchain.SkipBlockID, is *Site) {
	ca.sitesMutex.Lock()
	defer ca.sitesMutex.Unlock()
	ca.Sites[string(id)] = is
}

// saves the actual identity
func (ca *CA) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(ca.SiteMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(ca.path+"/ca.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (ca *CA) tryLoad() error {
	configFile := ca.path + "/ca.bin"
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
		ca.SiteMap = msg.(*SiteMap)
	}
	return nil
}

func newCAService(c *sda.Context, path string) sda.Service {
	keypair := config.NewKeyPair(network.Suite)
	ca := &CA{
		ServiceProcessor: sda.NewServiceProcessor(c),
		SiteMap:          &SiteMap{make(map[string]*Site)},
		path:             path,
		Public:           keypair.Public,
		Private:          keypair.Secret,
	}
	if err := ca.tryLoad(); err != nil {
		log.Error(err)
	}
	for _, f := range []interface{}{ca.SignCert, ca.GetPublicKey} {
		if err := ca.RegisterMessage(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return ca
}
