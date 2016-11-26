package webserver

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	//"reflect"
	"sync"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	//"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/sidentity"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/config"
)

// ServiceWSName can be used to refer to the name of this service
const ServiceWSName = "WebServer"

var WSService sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceWSName, newWSService)
	WSService = sda.ServiceFactory.ServiceID(ServiceWSName)
	network.RegisterPacketType(&SiteMap{})
	network.RegisterPacketType(&Site{})
}

// WS handles site identities (usually only one)
type WS struct {
	*sda.ServiceProcessor
	si *sidentity.Identity
	*SiteMap
	sitesMutex sync.Mutex
	path       string
}

// SiteMap holds the map to the sites so it can be marshaled.
type SiteMap struct {
	Sites map[string]*Site
}

// Site stores one site identity together with its latest skipblock & cert(s).
type Site struct {
	sync.Mutex
	// Site's ID (hash of the genesis block)
	ID skipchain.SkipBlockID
	// Latest known skipblock
	Latest *skipchain.SkipBlock
	//Certs  []*Cert
	Certs []*ca.Cert
	// Private key for that WS/site pair
	Private abstract.Scalar
	// Public key for that WS/site pair
	Public abstract.Point
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (ws *WS) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3(ws.ServerIdentity(), "CA received New Protocol event", conf)
	tn.ProtocolName()

	return nil, nil
}

// To be called after initialization of a web server
func (ws *WS) WSAttach(cothority *sda.Roster, id skipchain.SkipBlockID, public abstract.Point, private abstract.Scalar) error {
	ws.si.Cothority = cothority
	ws.si.ID = id
	ws.si.LatestID = id
	err := ws.si.ConfigUpdate()
	if err != nil {
		return err
	}

	//keypair := config.NewKeyPair(network.Suite)
	site := &Site{
		ID:      id,
		Latest:  ws.si.Latest,
		Certs:   ws.si.Certs,
		Public:  public,
		Private: private,
	}
	ws.setSiteStorage(id, site)
	site = ws.getSiteStorage(id)
	if site == nil {
		log.LLvlf2("WSAttach(): it wasn't possible to attach the web server to the requested site")
		return errors.New("WSAttach(): it wasn't possible to attach the web server to the requested site")
	}
	log.LLvlf2("WSAttach(): web server attached to the requested site (id: %v)", id)
	return nil
}

// Check for existence of new skipblocks/certs and update them
func (ws *WS) WSUpdate(id skipchain.SkipBlockID) error {
	log.LLvlf2("WSUpdate(): Start")
	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(id)
	if site == nil {
		log.LLvlf2("WSUpdate(): Update failed: web server not yet attached to the requested site (id: %v)", id)
		return errors.New("Update failed: web server not yet attached to the requested site")
	}
	//log.LLvlf2("WSUpdate(): the web server's public key for site: %v is: %v", id, site.Public)
	err := ws.si.ConfigUpdate()
	if err != nil {
		return err
	}

	site2 := site.Copy()
	site2.Latest = ws.si.Latest
	site2.Certs = ws.si.Certs

	ws.setSiteStorage(id, site2)

	return nil
}

func (ws *WS) WSGetSkipblocks(req *GetSkipblocks) ([]*skipchain.SkipBlock, error) {
	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(req.ID)
	if site == nil {
		return nil, errors.New("GetChain failed: web server not yet attached to the requested site")
	}

	sbs, err := ws.si.GetSkipblocks(req.ID, req.Latest)
	return sbs, err
}

func (ws *WS) WSGetValidSbPath(req *GetValidSbPath) ([]*skipchain.SkipBlock, error) {
	log.LLvlf2("WSGetValidSbPath(): Start processing the challenge for site identity: %v", req.ID)
	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(req.ID)
	if site == nil {
		return nil, errors.New("GetChain failed: web server not yet attached to the requested site")
	}

	sbs, err := ws.si.GetValidSbPath(req.ID, req.Sb1, req.Sb2)
	return sbs, err
}

/*
 * API messages
 */

func (ws *WS) UserAttachTo(wsi *network.ServerIdentity, con *Connect) (network.Body, error) {
	log.LLvlf2("UserAttachTo(): Start")
	err := ws.WSUpdate(con.ID)
	if err != nil {
		return nil, err
	}
	site := ws.getSiteStorage(con.ID)
	return &ConnectReply{
		Latest: site.Latest,
		Certs:  site.Certs,
	}, nil
}

func (ws *WS) UserGetSkipblocks(wsi *network.ServerIdentity, req *GetSkipblocks) (network.Body, error) {
	sbs, err := ws.WSGetSkipblocks(req)
	if err != nil {
		return nil, err
	}

	return &GetSkipblocksReply{
		Skipblocks: sbs,
	}, nil
}

func (ws *WS) UserGetValidSbPath(wsi *network.ServerIdentity, req *GetValidSbPath) (network.Body, error) {
	log.LLvlf2("UserGetValidSbPath(): Start processing the challenge for site identity: %v", req.ID)
	sbs, err := ws.WSGetValidSbPath(req)
	if err != nil {
		return nil, err
	}

	return &GetValidSbPathReply{
		Skipblocks: sbs,
	}, nil
}

func (ws *WS) UserChallenge(wsi *network.ServerIdentity, c *ChallengeReq) (network.Body, error) {
	log.LLvlf2("UserChallenge(): Start processing the challenge for site identity: %v", c.ID)
	website := ws.getSiteStorage(c.ID)
	if website == nil {
		log.LLvlf2("UserChallenge() failed: web server not yet attached to the requested site")
		return nil, errors.New("UserChallenge() failed: web server not yet attached to the requested site")
	}
	log.LLvlf2("Web server's public key for this site is: %v", website.Public)
	log.LLvlf2("UserChallenge(): Before signing: Private: %v, Public: %v", website.Private, website.Public)
	sig, err := crypto.SignSchnorr(network.Suite, website.Private, c.Challenge)
	if err != nil {
		return nil, err
	}
	log.LLvlf2("UserChallenge(): Before returning")
	return &ChallengeReply{
		Signature: &sig,
	}, nil

}

func (ws *WS) getSiteStorage(id skipchain.SkipBlockID) *Site {
	ws.sitesMutex.Lock()
	defer ws.sitesMutex.Unlock()
	is, ok := ws.Sites[string(id)]
	if !ok {
		return nil
	}
	return is
}

func (ws *WS) setSiteStorage(id skipchain.SkipBlockID, is *Site) {
	ws.sitesMutex.Lock()
	defer ws.sitesMutex.Unlock()
	ws.Sites[string(id)] = is
}

func (site *Site) Copy() *Site {
	site2 := &Site{
		ID:      site.ID,
		Latest:  site.Latest,
		Certs:   site.Certs,
		Public:  site.Public,
		Private: site.Private,
	}
	return site2
}

func (ws *WS) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(ws.SiteMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(ws.path+"/webserver.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

func (ws *WS) clearSites() {
	ws.Sites = make(map[string]*Site)
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (ws *WS) tryLoad() error {
	configFile := ws.path + "/webserver.bin"
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
		ws.SiteMap = msg.(*SiteMap)
	}
	return nil
}

func newWSService(c *sda.Context, path string) sda.Service {
	pinstate := &common_structs.PinState{
		Ctype: "ws",
	}
	ws := &WS{
		ServiceProcessor: sda.NewServiceProcessor(c),
		si:               sidentity.NewIdentity(nil, 0, "", pinstate, nil, nil),
		SiteMap:          &SiteMap{make(map[string]*Site)},
		path:             path,
	}
	if err := ws.tryLoad(); err != nil {
		log.Error(err)
	}
	for _, f := range []interface{}{ws.UserAttachTo, ws.UserGetSkipblocks, ws.UserGetValidSbPath, ws.UserChallenge} {
		if err := ws.RegisterMessage(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return ws
}
