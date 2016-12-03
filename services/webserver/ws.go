package webserver

import (
	"bytes"
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
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/sidentity"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/random"
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
	//network.RegisterPacketType(&common_structs.My_Scalar{})
}

// WS handles site identities (usually only one)
type WS struct {
	sitesMutex sync.Mutex
	path       string

	*sda.ServiceProcessor
	si *sidentity.Identity
	*SiteMap
	// Private key for that WS/site pair (to be used for decryption of the tls private key)
	Private abstract.Scalar
	// Public key for that WS/site pair
	Public abstract.Point
	// holds the mapping between FDQNs and genesis skipblocks' IDs
	NameToID map[string]skipchain.SkipBlockID
}

// SiteMap holds the map to the sites so it can be marshaled.
type SiteMap struct {
	Sites map[string]*Site
}

// Site stores one site identity together with its latest skipblock & cert(s).
type Site struct {
	sync.Mutex
	si *sidentity.Identity
	// Site's ID (hash of the genesis block)
	ID skipchain.SkipBlockID
	// the whole site's skipchain (starting with the genesis block)
	SkipBlocks map[string]*skipchain.SkipBlock
	// Latest known skipblock
	Latest *skipchain.SkipBlock
	// Hash of the 'Latest' known block
	LatestHash skipchain.SkipBlockID
	//Certs []*ca.Cert
	// Certs keeps the mapping between the config (the hash of the skipblock that contains it) and the cert(s)
	// that was(were) issued for that particular config
	//Certs map[string][]*ca.Cert // only 1 cert is non expired at a given point of time, so only this one is stored here
	CertInfo *common_structs.CertInfo
	// TLS private key for that WS/site pair
	TLSPrivate abstract.Scalar
	// TLS public key for that WS/site pair
	TLSPublic abstract.Point
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (ws *WS) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3(ws.ServerIdentity(), "WS received New Protocol event", conf)
	tn.ProtocolName()

	return nil, nil
}

// To be called after initialization of a web server in order for its public key (which is going to be used
// for encryption of its future tls private keys for all the sites to which the web server is going to be
// attached) to be passed to the cothority (from which the devices are going to pull it).
func (ws *WS) WSPushPublicKey(cothority *sda.Roster) error {
	//log.LLvlf2("WSPushPublicKey(): Start")
	//suite := nist.NewAES128SHA256P256()
	suite := ed25519.NewAES128SHA256Ed25519(false)

	// Create a public/private keypair
	private := suite.Scalar().Pick(random.Stream) // web server's private key
	public := suite.Point().Mul(nil, private)     // web server's public key

	ws.si.Cothority = cothority
	//ws.si.ID = id
	//ws.si.LatestID = id

	// pass the public key to the cothority (from which the devices are going to pull it)
	err := ws.si.PushPublicKey(public, ws.ServerIdentity())
	if err != nil {
		return err
	}
	ws.Private = private
	ws.Public = public
	/*
		site := &Site{
			//ID:      id,
			Private: private,
			Public:  public,
		}
		ws.setSiteStorage(id, site)
	*/
	//log.LLvlf2("WSPushPublicKey(): End")
	return nil
}

func (ws *WS) WSAttach(name string, id skipchain.SkipBlockID, cothority *sda.Roster) error {
	log.LLvlf2("WSAttach(): attaching to site: %v", name)

	site := &Site{
		ID:         id,
		LatestHash: id,
		SkipBlocks: make(map[string]*skipchain.SkipBlock),
		//Certs:      make(map[string][]*common_structs.Cert),
	}
	site.si = sidentity.NewIdentity(nil, "", 0, "", "ws", nil, nil, 0)
	site.si.Cothority = cothority
	site.si.ID = id
	site.si.LatestID = id
	ws.setSiteStorage(id, site)

	ws.NameToID[name] = id

	_ = ws.WSUpdate(id)

	log.LLvlf2("Web server with ServerIdentity: %v is now attached to site with ID: %v", ws.ServerIdentity(), id)
	return nil
}

// asks the cothority for new skipblocks, fetches all of them starting with the latest known
// till the current head one and (possibly) updates the tls keypair of the ws
func (ws *WS) WSUpdate(id skipchain.SkipBlockID) error {
	log.LLvlf2("WSUpdate(): Start")
	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(id)
	if site == nil {
		log.LLvlf2("WSUpdate failed: web server not yet attached to the requested site")
		return errors.New("WSUpdate failed: web server not yet attached to the requested site")
	}

	//sbs, err := ws.si.GetValidSbPath(id, site.Latest.Hash, []byte{0})
	sbs, err := site.si.GetValidSbPath(id, site.LatestHash, []byte{0})

	// Store the not previously known skipblocks (the latest known is stored again because the
	// the genesis block of the site's skipchain must be stored the first time WSUpdate() is invoked)
	// (Trust delegation between each pair of subsequent skipblocks already verified by 'GetValidSbPath')
	for _, sb := range sbs {
		_ = site.setSkipBlock(sb)
	}

	site.Latest = sbs[len(sbs)-1]
	site.LatestHash = site.Latest.Hash
	site.si.LatestID = site.Latest.Hash

	// update web server's tls keypair
	tlspublic, tlsprivate, _ := ws.WSgetTLSconf(id, sbs[len(sbs)-1])
	site.TLSPublic = tlspublic
	site.TLSPrivate = tlsprivate

	// check for new cert
	// TODO: verify them
	cert, hash, _ := site.si.GetCert(id)
	certinfo := &common_structs.CertInfo{
		Cert:   cert,
		SbHash: hash,
	}
	site.CertInfo = certinfo

	ws.setSiteStorage(id, site)
	log.LLvlf2("WSUpdate(): End")
	return err
}

// if h2==0, fetch all the skipblocks from the latest known till the current head one
func (ws *WS) FetchSkipblocks(id skipchain.SkipBlockID, h1, h2 skipchain.SkipBlockID) ([]*skipchain.SkipBlock, error) {
	log.LLvlf2("FetchSkipblocks(): Start")
	_ = ws.WSUpdate(id)

	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(id)
	if site == nil {
		return nil, errors.New("FetchSkipblocks failed: web server not yet attached to the requested site")
	}

	var ok bool
	var sb1 *skipchain.SkipBlock
	if !bytes.Equal(h1, []byte{0}) {
		sb1, ok = site.getSkipBlockByID(h1)
		if !ok {
			log.LLvlf2("Skipblock with hash: %v not found", h1)
			return nil, nil
		}
	} else {
		// fetch all the blocks starting from the one for the config of
		// which the latest cert is acquired
		h1 = site.CertInfo.SbHash
		sb1, ok = site.getSkipBlockByID(h1)
		if !ok {
			log.LLvlf2("NO VALID PATH: Skipblock with hash: %v not found", h1)
			return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", h1)
		}
		log.LLvlf2("Last certified skipblock has hash: %v", h1)
	}

	var sb2 *skipchain.SkipBlock
	if !bytes.Equal(h2, []byte{0}) {
		sb2, ok = site.getSkipBlockByID(h2)
		if !ok {
			log.LLvlf2("NO VALID PATH: Skipblock with hash: %v not found", h2)
			return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", h2)
		}
	} else {
		// fetch skipblocks until finding the current head of the skipchain
		h2 = site.Latest.Hash
		sb2 = site.Latest
		log.LLvlf2("Current head skipblock has hash: %v", h2)
	}

	oldest := sb1
	newest := sb2

	log.LLvlf2("Oldest skipblock has hash: %v", oldest.Hash)
	log.LLvlf2("Newest skipblock has hash: %v", newest.Hash)
	sbs := make([]*skipchain.SkipBlock, 0)
	sbs = append(sbs, oldest)
	block := oldest
	log.LLvlf2("Skipblock with hash: %v", block.Hash)
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[0]
		hash := link.Hash
		//log.LLvlf2("Appending skipblock with hash: %v", hash)
		block, ok = site.getSkipBlockByID(hash)
		if !ok {
			log.LLvlf2("Skipblock with hash: %v not found", hash)
			return nil, fmt.Errorf("Skipblock with hash: %v not found", hash)
		}
		sbs = append(sbs, block)
		if bytes.Equal(hash, site.Latest.Hash) || bytes.Equal(hash, newest.Hash) {
			break
		}
	}

	log.LLvlf2("FetchSkipblocks(): End")
	return sbs, nil
}

// fetch the latest cert (should exist only one not-yet-expired cert at every given point of time)
func (ws *WS) FetchCert(id skipchain.SkipBlockID) (*common_structs.Cert, error) {
	_ = ws.WSUpdate(id)

	site := ws.getSiteStorage(id)
	if site == nil {
		return nil, errors.New("FetchCerts() failed: web server not yet attached to the requested site")
	}
	/*
		certs := make([]*ca.Cert, 0)
		for _, certarray := range site.Certs {
			for _, cert := range certarray {
				certs = append(certs, cert)
			}
		}

		return certs[0], nil
	*/
	return site.CertInfo.Cert, nil
}

/*
func (ws *WS) WSGetSkipblocks(req *GetSkipblocks) ([]*skipchain.SkipBlock, error) {
	_ = ws.WSUpdate(req.ID)

	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(req.ID)
	if site == nil {
		return nil, errors.New("GetChain failed: web server not yet attached to the requested site")
	}

	sbs, err := ws.si.GetSkipblocks(req.ID, req.Latest)

	return sbs, err
}
*/
/*
 * API messages
 */
/*
func (ws *WS) UserAttachTo(wsi *network.ServerIdentity, con *Connect) (network.Body, error) {
	log.LLvlf2("UserAttachTo(): Start")
	err := ws.WSGetTLSPrivateKey(con.ID)
	if err != nil {
		log.LLvlf2("PROBLEM")
		return nil, err
	}
	site := ws.getSiteStorage(con.ID)
	return &ConnectReply{
		Latest: site.Latest,
		Certs:  site.Certs,
	}, nil
}
*/
/*
func (ws *WS) UserGetSkipblocks(wsi *network.ServerIdentity, req *GetSkipblocks) (network.Body, error) {
	sbs, err := ws.WSGetSkipblocks(req)

	if err != nil {
		return nil, err
	}

	return &GetSkipblocksReply{
		Skipblocks: sbs,
	}, nil
}
*/
func (ws *WS) UserGetValidSbPath(wsi *network.ServerIdentity, req *GetValidSbPath) (network.Body, error) {
	log.LLvlf2("UserGetValidSbPath(): Start")

	id := ws.NameToID[req.FQDN]

	sbs, err := ws.FetchSkipblocks(id, req.Hash1, req.Hash2)
	if err != nil {
		return nil, err
	}
	log.LLvlf2("UserGetValidSbPath(): Skipblocks fetched")

	if bytes.Equal(req.Hash2, []byte{0}) {
		cert, _ := ws.FetchCert(id)
		log.LLvlf2("UserGetValidSbPath(): Cert fetched")
		return &GetValidSbPathReply{
			Skipblocks: sbs,
			Cert:       cert,
		}, nil

	}

	return &GetValidSbPathReply{
		Skipblocks: sbs,
		Cert:       nil,
	}, nil
}

func (ws *WS) UserChallenge(wsi *network.ServerIdentity, c *ChallengeReq) (network.Body, error) {
	id := ws.NameToID[c.FQDN]
	log.LLvlf2("UserChallenge(): Start processing the challenge for site: %v", c.FQDN)
	website := ws.getSiteStorage(id)
	if website == nil {
		log.LLvlf2("UserChallenge() failed: web server not yet attached to the requested site")
		return nil, errors.New("UserChallenge() failed: web server not yet attached to the requested site")
	}
	//log.LLvlf2("Web server's public key for this site is: %v", website.TLSPublic)
	//log.LLvlf2("UserChallenge(): Before signing: Private: %v, Public: %v", website.TLSPrivate, website.TLSPublic)
	sig, err := crypto.SignSchnorr(network.Suite, website.TLSPrivate, c.Challenge)
	if err != nil {
		return nil, err
	}
	log.LLvlf2("UserChallenge(): End")
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

// takes a site id and a skipblock and returns the public & (decrypted) private key that was assigned to
// the specific web server (asymmetric crypto is used for the encryption/decryption of the tls private key)
func (ws *WS) WSgetTLSconf(id skipchain.SkipBlockID, latest_sb *skipchain.SkipBlock) (abstract.Point, abstract.Scalar, error) {
	//log.LLvlf2("WSgetTLSconf(): Start")
	website := ws.getSiteStorage(id)
	if website == nil {
		log.LLvlf2("WSgetTLSconf() failed: web server not yet attached to the requested site")
		return nil, nil, errors.New("WSgetTLSconf() failed: web server not yet attached to the requested site")
	}

	config, err := common_structs.GetConfFromSb(latest_sb)
	if err != nil {
		return nil, nil, err
	}
	//log.LLvlf2("WSgetTLSconf(): 1")
	serverID := ws.ServerIdentity()
	key := fmt.Sprintf("tls:%v", serverID)
	our_data_entry := config.Data[key]
	tlspublic := our_data_entry.TLSPublic

	K1 := our_data_entry.K1
	C1 := our_data_entry.C1
	K2 := our_data_entry.K2
	C2 := our_data_entry.C2
	//log.LLvlf2("K1: %v, C1: %v", K1, C1)
	//log.LLvlf2("K2: %v, C2: %v", K2, C2)
	//log.LLvlf2("WSgetTLSconf(): 2")
	// Decrypt it using the corresponding private key.
	//suite := nist.NewAES128SHA256P256()
	suite := ed25519.NewAES128SHA256Ed25519(false)
	decrypted1, err := common_structs.ElGamalDecrypt(suite, ws.Private, K1, C1)
	decrypted2, err := common_structs.ElGamalDecrypt(suite, ws.Private, K2, C2)
	//log.LLvlf2("WSgetTLSconf(): 3: decrypted1 bytes: %v", decrypted1)
	//log.LLvlf2("WSgetTLSconf(): 3: decrypted2 bytes: %v", decrypted2)

	decrypted := make([]byte, 0)
	for _, b := range decrypted1 {
		decrypted = append(decrypted, b)
	}
	for _, b := range decrypted2 {
		decrypted = append(decrypted, b)
	}
	_, data, err2 := network.UnmarshalRegistered(decrypted)
	if err2 != nil {
		log.LLvlf2("%v", err2)
	}
	//log.LLvlf2("WSgetTLSconf(): 4: data: %v", data)
	rec := data.(*common_structs.My_Scalar)
	tlsprivate := rec.Private
	log.LLvlf2("reconstructed private key: %v", tlsprivate)
	//log.LLvlf2("WSgetTLSconf(): End")
	return tlspublic, tlsprivate, nil
}

func (s *Site) setSkipBlock(latest *skipchain.SkipBlock) bool {
	//log.LLvlf2("Storing skipblock with id: %v", latest.Hash)
	s.SkipBlocks[string(latest.Hash)] = latest
	return true
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Site) getSkipBlockByID(sbID skipchain.SkipBlockID) (*skipchain.SkipBlock, bool) {
	b, ok := s.SkipBlocks[string(sbID)]
	return b, ok
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
	ws := &WS{
		ServiceProcessor: sda.NewServiceProcessor(c),
		si:               sidentity.NewIdentity(nil, "", 0, "", "ws", nil, nil, 0),
		SiteMap:          &SiteMap{make(map[string]*Site)},
		path:             path,
		NameToID:         make(map[string]skipchain.SkipBlockID),
	}
	if err := ws.tryLoad(); err != nil {
		log.Error(err)
	}
	for _, f := range []interface{}{ws.UserGetValidSbPath, ws.UserChallenge} {
		if err := ws.RegisterMessage(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return ws
}
