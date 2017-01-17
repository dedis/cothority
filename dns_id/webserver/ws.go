package webserver

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/sidentity"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/dedis/onet.v1/simul/monitor"
)

// ServiceWSName can be used to refer to the name of this service
const ServiceWSName = "WebServer"

var WSService onet.ServiceID

func init() {
	onet.RegisterNewService(ServiceWSName, newWSService)
	WSService = onet.ServiceFactory.ServiceID(ServiceWSName)
	network.RegisterMessage(&SiteMap{})
	network.RegisterMessage(&Site{})
	network.RegisterMessage(&common_structs.IdentityReady{})
	network.RegisterMessage(&common_structs.PushedPublic{})
	network.RegisterMessage(&common_structs.StartWebserver{})
	network.RegisterMessage(&common_structs.SiteInfo{})
	network.RegisterMessage(&GetValidSbPath{})
	network.RegisterMessage(&common_structs.ConnectClient{})
}

// WS handles site identities (usually only one)
type WS struct {
	sitesMutex sync.Mutex
	path       string

	*onet.ServiceProcessor
	si *sidentity.Identity
	*SiteMap
	// Private key for that WS/site pair (to be used for decryption of the tls private key)
	Private abstract.Scalar
	// Public key for that WS/site pair
	Public abstract.Point
	// holds the mapping between FDQNs and genesis skipblocks' IDs
	NameToID map[string]skipchain.SkipBlockID
	fqdn     string
	UpdateDuration time.Duration
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
	// PoF for the latest known skipblock
	PoF      *common_structs.SignatureResponse
	CertInfo *common_structs.CertInfo
	// TLS private key for that WS/site pair
	TLSPrivate abstract.Scalar
	// TLS public key for that WS/site pair
	TLSPublic abstract.Point
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (ws *WS) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3(ws.ServerIdentity(), "WS received New Protocol event", conf)
	tn.ProtocolName()

	return nil, nil
}

// To be called after initialization of a web server in order for its public key (which is going to be used
// for encryption of its future tls private keys for all the sites to which the web server is going to be
// attached) to be passed to the cothority (from which the devices are going to pull it).
func (ws *WS) WSPushPublicKey(cothority *onet.Roster) error {
	suite := ed25519.NewAES128SHA256Ed25519(false)

	// Create a public/private keypair
	private := suite.Scalar().Pick(random.Stream) // web server's private key
	public := suite.Point().Mul(nil, private)     // web server's public key

	ws.si.Cothority = cothority

	// pass the public key to the cothority (from which the devices are going to pull it)
	err := ws.si.PushPublicKey(public, ws.ServerIdentity())
	if err != nil {
		return err
	}
	ws.sitesMutex.Lock()
	defer ws.sitesMutex.Unlock()
	ws.Private = private
	ws.Public = public

	return nil
}

func (ws *WS) WSAttach(name string, id skipchain.SkipBlockID, cothority *onet.Roster) error {

	log.LLvlf3("WSAttach(): attaching to site: %v", name)

	site := &Site{
		ID:         id,
		LatestHash: id,
		SkipBlocks: make(map[string]*skipchain.SkipBlock),
	}
	site.si = sidentity.NewIdentity(nil, "", 0, "", "ws", nil, nil, 0)
	site.si.Cothority = cothority
	site.si.ID = id
	site.si.LatestID = id
	ws.setSiteStorage(id, site)

	ws.sitesMutex.Lock()
	ws.NameToID[name] = id

	site = ws.getSiteStorage(id)
	if site == nil {
		log.Lvlf2("WSAttach failed: web server not yet attached to the requested site")
		return errors.New("WSAttach failed: web server not yet attached to the requested site")
	}

	_ = ws.WSUpdate(id)
	ws.sitesMutex.Unlock()
	log.Lvlf2("Web server with ServerIdentity: %v is now attached to site with ID: %v", ws.ServerIdentity(), id)
	return nil
}

// Asks the cothority for new skipblocks, fetches all of them starting with the latest known
// till the current head one and (possibly) updates the tls keypair of the ws
// Also updates the cert and the PoF
func (ws *WS) WSUpdate(id skipchain.SkipBlockID) error {

	log.Lvlf3("WSUpdate(): Start")
	// Check whether the reached ws has been configured as a valid web server of the requested site
	site := ws.getSiteStorage(id)
	if site == nil {
		log.Lvlf2("WSUpdate failed: web server not yet attached to the requested site")
		return errors.New("WSUpdate failed: web server not yet attached to the requested site")
	}
	site.Lock() //have been commented before
	defer site.Unlock() //have been commented before

	log.Lvlf2("Web server %v has latest block with hash: %v", ws.ServerIdentity(), site.LatestHash)
	sbs, cert, hash, pof, err := site.si.GetValidSbPath(id, site.LatestHash, []byte{0})
	times := 0
	for len(sbs) == 0 && times < 10 {
		times++
		log.Lvlf2("%v: ws %v resends message", times, ws.ServerIdentity())
		// retry after 1 sec
		time.Sleep(1000 * time.Millisecond)
		sbs, cert, hash, pof, err = site.si.GetValidSbPath(id, site.LatestHash, []byte{0})
	}

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
	log.Lvlf3("Web server %v has now public key: %v (prev_key: %v) - Latest block has hash: %v", ws.ServerIdentity(), tlspublic, site.TLSPublic, site.LatestHash)
	site.TLSPublic = tlspublic
	site.TLSPrivate = tlsprivate

	// TODO: verify it
	certinfo := &common_structs.CertInfo{
		Cert:   cert,
		SbHash: hash,
	}
	site.CertInfo = certinfo

	if pof == nil {
		log.Print("null pof  !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}
	site.PoF = pof

	ws.setSiteStorage(id, site)
	log.Lvlf3("WSUpdate(): End")
	return err
}

// if h2==0, fetch all the skipblocks from the latest known till the current head one
func (ws *WS) FetchSkipblocks(id skipchain.SkipBlockID, h1, h2 skipchain.SkipBlockID) ([]*skipchain.SkipBlock, error) {
	log.Lvlf3("FetchSkipblocks(): Start")
	//_ = ws.WSUpdate(id)

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
			log.Lvlf2("Skipblock with hash: %v not found", h1)
			return nil, nil
		}
	} else {
		// fetch all the blocks starting from the one for the config of
		// which the latest cert is acquired
		h1 = site.CertInfo.SbHash
		sb1, ok = site.getSkipBlockByID(h1)
		if !ok {
			log.Lvlf2("NO VALID PATH: Skipblock with hash: %v not found", h1)
			return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", h1)
		}
		log.Lvlf3("Last certified skipblock has hash: %v", h1)
	}

	var sb2 *skipchain.SkipBlock
	if !bytes.Equal(h2, []byte{0}) {
		sb2, ok = site.getSkipBlockByID(h2)
		if !ok {
			log.Lvlf2("NO VALID PATH: Skipblock with hash: %v not found", h2)
			return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", h2)
		}
	} else {
		// fetch skipblocks until finding the current head of the skipchain
		h2 = site.Latest.Hash
		sb2 = site.Latest
		log.Lvlf3("Current head skipblock has hash: %v", h2)
	}

	oldest := sb1
	newest := sb2

	log.Lvlf3("Oldest skipblock has hash: %v", oldest.Hash)
	log.Lvlf3("Newest skipblock has hash: %v", newest.Hash)
	sbs := make([]*skipchain.SkipBlock, 0)
	sbs = append(sbs, oldest)
	block := oldest
	log.Lvlf3("Skipblock with hash: %v", block.Hash)
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[0]
		hash := link.Hash
		block, ok = site.getSkipBlockByID(hash)
		if !ok {
			log.Lvlf2("Skipblock with hash: %v not found", hash)
			return nil, fmt.Errorf("Skipblock with hash: %v not found", hash)
		}
		sbs = append(sbs, block)
		if bytes.Equal(hash, site.Latest.Hash) || bytes.Equal(hash, newest.Hash) {
			break
		}
	}

	log.Lvlf3("FetchSkipblocks(): End")
	return sbs, nil
}

// fetch the latest cert (should exist only one not-yet-expired cert at every given point of time)
func (ws *WS) FetchCert(id skipchain.SkipBlockID) (*common_structs.Cert, error) {
	//_ = ws.WSUpdate(id)

	site := ws.getSiteStorage(id)
	if site == nil {
		return nil, errors.New("FetchCerts() failed: web server not yet attached to the requested site")
	}

	return site.CertInfo.Cert, nil
}

// fetch the latest PoF
func (ws *WS) FetchPoF(id skipchain.SkipBlockID) (*common_structs.SignatureResponse, error) {
	//_ = ws.WSUpdate(id)

	site := ws.getSiteStorage(id)
	if site == nil {
		return nil, errors.New("FetchPoF() failed: web server not yet attached to the requested site")
	}
	if site.PoF==nil {
		log.Print("FetchPoF(): NULL POF!!!!!!!!!!")
		return nil, errors.New("FetchPoF(): NULL POF!!!!!!!!!!")
	}
	return site.PoF, nil
}

/*
 * API messages
 */

func (ws *WS) UserGetValidSbPath(req *GetValidSbPath) (network.Message, onet.ClientError) {
	//ws.sitesMutex.Lock()
	//defer ws.sitesMutex.Unlock()

	log.Lvlf1("UserGetValidSbPath(): received connection req for %s", req.FQDN)

	id := ws.NameToID[req.FQDN]
	site := ws.getSiteStorage(id)
	if site == nil {
		log.Print("error")
		return nil, onet.NewClientErrorCode(4100, "UserGetValidSbPath() failed: web server not yet attached to the requested site")
	}

	site.Lock() //have not existed before
	defer site.Unlock()  //have not existed before

	//_ = ws.WSUpdate(id)
	sbs, err := ws.FetchSkipblocks(id, req.Hash1, req.Hash2)
	if err != nil {
		log.Print("error")
		return nil, onet.NewClientError(err)
	}
	log.Lvlf3("UserGetValidSbPath(): Skipblocks fetched")

	pof, _ := ws.FetchPoF(id)
	// process challenge
	sig, err := crypto.SignSchnorr(network.Suite, site.TLSPrivate, req.Challenge)
	if err != nil {
		log.Print("error")
		return nil, onet.NewClientError(err)
	}
	log.Lvlf4("public key of server: %v is %v (latest known block: %v)", ws.ServerIdentity(), site.TLSPublic, site.LatestHash)

	if bytes.Equal(req.Hash2, []byte{0}) {
		cert, _ := ws.FetchCert(id)
		log.LLvlf3("UserGetValidSbPath(): Cert fetched")
		return &GetValidSbPathReply{
			Skipblocks: sbs,
			Cert:       cert,
			PoF:        pof,
			Signature:  &sig,
		}, nil

	}
	log.Print(ws.Context, "Sending back GetValidSbPathReply ")
	return &GetValidSbPathReply{
		Skipblocks: sbs,
		Cert:       nil,
		PoF:        pof,
		Signature:  &sig,
	}, nil
}

func (ws *WS) getSiteStorage(id skipchain.SkipBlockID) *Site {
	//ws.sitesMutex.Lock()
	//defer ws.sitesMutex.Unlock()
	is, ok := ws.Sites[string(id)]
	if !ok {
		return nil
	}
	return is
}

func (ws *WS) setSiteStorage(id skipchain.SkipBlockID, is *Site) {
	//ws.sitesMutex.Lock()
	//defer ws.sitesMutex.Unlock()
	ws.Sites[string(id)] = is
}

// takes a site id and a skipblock and returns the public & (decrypted) private key that was assigned to
// the specific web server (asymmetric crypto is used for the encryption/decryption of the tls private key)
func (ws *WS) WSgetTLSconf(id skipchain.SkipBlockID, latest_sb *skipchain.SkipBlock) (abstract.Point, abstract.Scalar, error) {
	website := ws.getSiteStorage(id)
	if website == nil {
		log.Lvlf2("WSgetTLSconf() failed: web server not yet attached to the requested site")
		return nil, nil, errors.New("WSgetTLSconf() failed: web server not yet attached to the requested site")
	}

	config, err := common_structs.GetConfFromSb(latest_sb)
	if err != nil {
		return nil, nil, err
	}

	serverID := ws.ServerIdentity()
	key := fmt.Sprintf("tls:%v", serverID)
	our_data_entry := config.Data[key]
	tlspublic := our_data_entry.TLSPublic

	K1 := our_data_entry.K1
	C1 := our_data_entry.C1
	K2 := our_data_entry.K2
	C2 := our_data_entry.C2

	// Decrypt it using the corresponding private key.
	suite := ed25519.NewAES128SHA256Ed25519(false)
	decrypted1, err := common_structs.ElGamalDecrypt(suite, ws.Private, K1, C1)
	decrypted2, err := common_structs.ElGamalDecrypt(suite, ws.Private, K2, C2)

	decrypted := make([]byte, 0)
	for _, b := range decrypted1 {
		decrypted = append(decrypted, b)
	}
	for _, b := range decrypted2 {
		decrypted = append(decrypted, b)
	}
	_, data, err2 := network.Unmarshal(decrypted)
	if err2 != nil {
		log.Lvlf2("%v", err2)
	}

	rec := data.(*common_structs.My_Scalar)
	tlsprivate := rec.Private
	log.Lvlf3("reconstructed private key: %v", tlsprivate)

	return tlspublic, tlsprivate, nil
}

func (s *Site) setSkipBlock(latest *skipchain.SkipBlock) bool {
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
	b, err := network.Marshal(ws.SiteMap)
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
		_, msg, err := network.Unmarshal(b)
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal: %s", err)
		}
		log.Lvl3("Successfully loaded")
		ws.SiteMap = msg.(*SiteMap)
	}
	return nil
}

func newWSService(c *onet.Context) onet.Service {
	ws := &WS{
		ServiceProcessor: onet.NewServiceProcessor(c),
		si:               sidentity.NewIdentity(nil, "", 0, "", "ws", nil, nil, 0),
		SiteMap:          &SiteMap{make(map[string]*Site)},
		NameToID:         make(map[string]skipchain.SkipBlockID),
		UpdateDuration: time.Millisecond * 1000 * 1,
	}
	//if err := ws.tryLoad(); err != nil {
	//	log.Error(err)
	//}
	for _, f := range []interface{}{ws.UserGetValidSbPath, ws.StartWebserver, ws.AttachWebserver, ws.ConnectClient} {
		if err := ws.RegisterHandler(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return ws
}

func (ws *WS) AttachWebserver(req *common_structs.IdentityReady) (network.Message, onet.ClientError) {

	wss := make([]common_structs.WSInfo, 0)
	wss = append(wss, common_structs.WSInfo{ServerID: ws.ServerIdentity()})

	siteInfo := &common_structs.SiteInfo{
		FQDN: ws.fqdn,
		WSs:  wss,
	}

	err := ws.WSAttach(ws.fqdn, req.ID, req.Cothority)
	log.ErrFatal(err)

	site := ws.getSiteStorage(req.ID)
	if site == nil {
		log.Lvlf2("WSUpdate failed: web server not yet attached to the requested site")
		return nil,  onet.NewClientErrorCode(4100,"WSUpdate failed: web server not yet attached to the requested site")
	}

	go func(){
		c := time.Tick(ws.UpdateDuration)

		for _ = range c {
			log.Lvlf2("Webserver update starts")
			ws.WSUpdate(req.ID)
			log.Lvlf2("Webserver update ends")
		}
	}()


	log.Print("Webserver is now attached, Sending back to CKH: ", req.CkhIdentity)
	client := onet.NewClient(sidentity.ServiceName)
	log.ErrFatal(client.SendProtobuf(req.CkhIdentity, siteInfo, nil))
	log.Lvlf2("Webserver dispatched the attached message")
	return nil, nil
}

func (ws *WS) StartWebserver(req *common_structs.StartWebserver) (network.Message, onet.ClientError) {
	roster := req.Roster
	roster_WK := req.Roster_WK
	index_CK := req.Index_CK
	index_ws, _ := roster.Search(ws.ServerIdentity().ID)
	ws.fqdn = fmt.Sprintf("site%d", index_ws)
	ckIdentity := roster.List[index_CK]
	log.Print(ws.Context, "StartWebServer WSPublishPublicKey")
	log.ErrFatal(ws.WSPushPublicKey(roster_WK))

	client := onet.NewClient(sidentity.ServiceName)
	log.Print(ws.Context, "StartWebServer", index_ws, "Sending back to ColdKeyHolder", index_CK)
	log.ErrFatal(client.SendProtobuf(ckIdentity, &common_structs.PushedPublic{}, nil))
	return nil, nil
}


func (ws *WS) ConnectClient(req *common_structs.ConnectClient) (network.Message, onet.ClientError) {
	s := req.Info
	round := monitor.NewTimeMeasure("client_time")
	NewUser("", s)
	round.Record()
	return nil, nil
}