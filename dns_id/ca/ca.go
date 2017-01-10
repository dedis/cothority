package ca

import (
	"bytes"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/onet/crypto"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"io/ioutil"
	"os"
	"sync"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/onet"
)

// ServiceName can be used to refer to the name of this service
const ServiceCAName = "CA"

var CAService onet.ServiceID

func init() {
	onet.RegisterNewService(ServiceCAName, newCAService)
	CAService = onet.ServiceFactory.ServiceID(ServiceCAName)
	network.RegisterPacketType(&SiteMap{})
	network.RegisterPacketType(&Site{})
}

// CA handles per site certificates
type CA struct {
	*onet.ServiceProcessor
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
	Cert   *common_structs.Cert
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (ca *CA) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(ca.ServerIdentity(), "CA received New Protocol event", conf)
	tn.ProtocolName()

	return nil, nil
}

/*
 * API messages
 */

// SignCert will use the CA's public key to sign a new cert
func (ca *CA) SignCert(csr *CSR) (network.Body, onet.ClientError) {
	log.Lvlf2("SignCert(): Start (CA's public key: %v)", ca.Public)
	id := csr.ID
	config := csr.Config
	prevconfig := csr.PrevConfig
	var hash []byte
	var err error

	if config == nil {
		log.Lvlf2("Nil config")
		return nil, onet.NewClientErrorCode(4100, "Nil config")
	}

	var trustedconf *common_structs.Config
	if prevconfig == nil {
		trustedconf = config
	} else {
		trustedconf = prevconfig
	}
	thr := trustedconf.Threshold

	// Verify that a threshold 'thr' of the 'trustedconf' devices have voted for the
	// 'config' for which the cert was asked
	cnt := 0
	for key, device := range config.Device {
		if _, exists := trustedconf.Device[key]; exists {
			b1, _ := network.MarshalRegisteredType(device.Point)
			b2, _ := network.MarshalRegisteredType(trustedconf.Device[key].Point)
			if bytes.Equal(b1, b2) {

				// Check whether there is a non-nil signature
				if device.Vote != nil {
					hash, err = config.Hash()
					if err != nil {
						log.Lvlf2("Couldn't get hash")
						return false, onet.NewClientErrorCode(4100, "Couldn't get hash")
					}

					// Verify signature
					err = crypto.VerifySchnorr(network.Suite, device.Point, hash, *device.Vote)
					if err != nil {
						log.Lvlf2("Wrong signature")
						return false, onet.NewClientErrorCode(4100, "Wrong signature")
					}
					cnt++
				}
			}
		}
	}

	if cnt < thr {
		log.Lvlf2("Not enough valid signatures (votes: %v, threshold: %v)", cnt, config.Threshold)
		return nil, onet.NewClientErrorCode(4100, "Not enough valid signatures")
	}

	// Check whether our clock is relatively close or not to the proposed timestamp
	err = config.CheckTimeDiff(maxdiff_sign)
	if err != nil {
		log.Lvlf2("CA with public key: %v %v refused to sign because of bad config timestamp", ca.Public, err)
		return nil, onet.NewClientError(err)
	}

	// Check that the validity period does not exceed an upper bound
	if config.MaxDuration > bound {
		log.Lvlf2("CA with public key: %v %v refused to sign because config's validity period exceeds an upper bound", ca.Public, err)
		return nil, onet.NewClientErrorCode(4100, "CA refused to sign because config's validity period exceeds an upper bound")
	}

	// Sign the config's hash using CA's private key
	var signature crypto.SchnorrSig

	signature, err = crypto.SignSchnorr(network.Suite, ca.Private, hash)
	if err != nil {
		log.Lvlf2("error: %v", err)
		return nil, onet.NewClientError(err)
	}

	cert := &common_structs.Cert{
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
	log.Lvlf3("SignCert(): End %v", cert)
	//log.Lvlf2("SignCert(): End with ID: %v, Hash: %v, Sig: %v, Public: %v", id, hash, signature, ca.Public)
	return &CSRReply{
		Cert: cert,
	}, nil
}

func (ca *CA) GetPublicKey(req *GetPublicKey) (network.Body, onet.ClientError) {
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

func newCAService(c *onet.Context, path string) onet.Service {
	keypair := config.NewKeyPair(network.Suite)
	ca := &CA{
		ServiceProcessor: onet.NewServiceProcessor(c),
		SiteMap:          &SiteMap{make(map[string]*Site)},
		path:             path,
		Public:           keypair.Public,
		Private:          keypair.Secret,
	}
	if err := ca.tryLoad(); err != nil {
		log.Error(err)
	}

	for _, f := range []interface{}{ca.SignCert, ca.GetPublicKey} {
		if err := ca.RegisterHandler(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return ca
}
