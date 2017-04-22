package webserver

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	//"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	for _, s := range []interface{}{
		// Structures
		&User{},
		&Data{},
		&WebSite{},
		&common_structs.Config{},
		&common_structs.PinState{},
		&skipchain.SkipBlockFix{},
		&skipchain.SkipBlock{},
		&common_structs.My_Scalar{},
		&common_structs.WSconfig{},

		&common_structs.CAInfo{},
		&common_structs.WSInfo{},
		&common_structs.SiteInfo{},
		&common_structs.Key{},

		// API messages
		&Connect{},
		&ConnectReply{},
		&GetValidSbPathLight{},
		&GetValidSbPathLightReply{},
		&common_structs.IdentityReady{},
		&common_structs.PushedPublic{},
		&common_structs.StartWebserver{},
	} {
		network.RegisterMessage(s)
	}
}

type User struct {
	// WSClient is included for easy `Send`-methods.
	WSClient *onet.Client
	// Data holds all the data related to this web site user
	// It can be stored and loaded from a config file.
	Data
}

type Data struct {
	UserName string
	*WebSiteMap
	sitesMutex sync.Mutex
}

// WebSiteMap holds the map to the sites so it can be marshaled.
type WebSiteMap struct {
	WebSites map[string]*WebSite // site's FQDN is the key of the map
}

type WebSite struct {
	sync.Mutex
	// Site's FQDN
	FQDN string
	// Config is the latest, valid configuration of the site.
	Config *common_structs.Config
	// Latest known skipblock
	Latest *skipchain.SkipBlock
	// Pin-state
	PinState *common_structs.PinState
	// only one not expired cert per site at a given point of time
	Cert *common_structs.Cert
	// Addresses of the site's web servers
	WSs []common_structs.WSInfo
}

func NewUser(username string, sitesToAttach []*common_structs.SiteInfo) *User {
	user := &User{
		WSClient: onet.NewClient(ServiceWSName),
		Data: Data{
			UserName: username,
			WebSiteMap: &WebSiteMap{
				WebSites: make(map[string]*WebSite),
			},
		},
	}
	user.NewAttachments(sitesToAttach)
	return user
}

func (u *User) NewAttachments(sitesInfo []*common_structs.SiteInfo) {
	for _, siteInfo := range sitesInfo {
		log.Lvlf2("NewAttachments(): Trying to attach to site: %v", siteInfo.FQDN)
		err := u.Connect(siteInfo)
		if err != nil {
			log.ErrFatal(err)
		}
	}
	return
}

func (u *User) Connect(siteInfo *common_structs.SiteInfo) error {
	u.sitesMutex.Lock()
	defer u.sitesMutex.Unlock()
	name := siteInfo.FQDN
	log.Lvlf2("Connecting user: %v to site: %v", u.UserName, name)

	// Check whether we are trying or not to re-attach the user to a site identity
	username := u.UserName
	if _, exists := u.WebSites[name]; exists {
		log.Lvlf2("Trying to re-attach the user to site: %v", name)
		return fmt.Errorf("Trying to re-attach the user to site: %v", name)
	}

	website := WebSite{
		FQDN: name,
		WSs:  siteInfo.WSs,
		PinState: &common_structs.PinState{
			//Window: int64(86400), // 86400ms = 1 day * 24 hours/day * 3600 sec/hour
			Window: int64(1),
		},
	}

	wss := website.WSs
	serverID := wss[rand.Int()%len(wss)].ServerID

	log.Lvlf1("Connect(): Before fetching skipblocks: serverID=%v, FQDN=%v", serverID, name)
	reply := &GetValidSbPathLightReply{}
	cerr := u.WSClient.SendProtobuf(serverID, &GetValidSbPathLight{FQDN: name, Hash1: []byte{0}, Hash2: []byte{0}, Challenge: []byte{}}, reply)
	if cerr != nil {
		log.ErrFatal(cerr)
	}
	sbs := reply.Configblocks
	latestconf := sbs[len(sbs)-1]
	cert := reply.Cert
	pof := reply.PoF
	if pof == nil {
		log.Print("!!!!!! Null pof!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}
	log.LLvlf3("Connect(): %v skipblocks fetched", len(sbs))

	// Check whether the latest config was recently signed by the Cold Key Holders
	// If not, then check if there exists a "good" PoF signed by the Warm Key Holders
	err := latestconf.CheckTimeDiff(maxdiff)
	if err != nil {
		log.Print("Stale block, check the pof")
		err = pof.Validate(latestconf, maxdiff)
		if err != nil {
			log.Lvlf2("%v", err)
			return err
		}
	}

	// TODO: verify that the CA is indeed a CA (maybe by checking its public key's membership
	// into a trusted pull of CAs' public keys?)
	/*
		// UNCOMMENT IF CAs ARE TO BE USED
		// Validate the signature of the CA
		cert_hash := cert.Hash // the certified config's hash
		err = crypto.VerifySchnorr(network.Suite, cert.Public, cert_hash, *cert.Signature)
		if err != nil {
			log.Lvlf2("CA's signature doesn't verify")
			return errors.New("CA's signature doesn't verify")
		}

		// Verify that the config of the first returned skipblock has been certified
		_, data, _ = network.Unmarshal(sbs[0].Data)
		firstconf, _ := data.(*common_structs.Config)
		conf_hash, _ := firstconf.Hash()

		if !bytes.Equal(cert_hash, conf_hash) {
			return fmt.Errorf("Cert not upon the config of the first returned skipblock")
		}

		// Check that the returned cert is pointing to the requested FQDN
		fqdn := firstconf.FQDN
		if res := strings.Compare(fqdn, name); res != 0 {
			return fmt.Errorf("Returned cert validates another mapping (wrong FQDN) -> Cannot reconnect to the site: %v", name)
		}

		// Check that the returned cert has not yet expired
		expired := firstconf.ExpiredCertConfig()
		if expired {
			return fmt.Errorf("Expired cert -> Cannot connect to the site: %v", name)
		}
	*/

	// Verify the hops starting from the skipblock (sbs[0]) for which the cert was issued
	ok, _ := VerifyHopsLight(sbs)
	if !ok {
		log.Lvlf2("Not valid hops")
		return fmt.Errorf("Not valid hops")
	} else {
		log.Lvlf2("User: %v - Start following the site by setting trust_window: %v", username, website.PinState.Window)
		website.PinState.TimePinAccept = time.Now().Unix() * 1000
		u.WebSites[name] = &website
		u.Follow(name, latestconf, cert)
	}
	log.Print("CONNECTED: user: %v, site: %v", u.UserName, name)
	return nil
}

// Attempt to RE-visit site "name"
func (u *User) ReConnect(name string) error {
	u.sitesMutex.Lock()
	defer u.sitesMutex.Unlock()
	username := u.UserName
	log.Lvlf2("Re-connecting user: %v to site: %v", username, name)
	// Check whether we are trying or not to (re)visit a not yet attached site
	if _, exists := u.WebSites[name]; !exists {
		log.Lvlf2("%v tries to (re)visit a not yet attached site (site's FQDN: %v)", u.UserName, name)
		return fmt.Errorf("Trying to (re)visit a not yet attached site (site's FQDN: %v)", name)
	}

	website := u.WebSites[name]
	// if now > rec + window then the pins have already expired ->  start from scratch (get
	// new pins, certs etc) and check whether we are still following the previously following
	// site skipchain
	if expired := website.ExpiredPins(); expired {

		same_skipchain := false

		log.Lvlf2("Pins have expired!!! (user: %v, site: %v)", u.UserName, name)
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID

		reply := &GetValidSbPathLightReply{}
		u.WSClient.SendProtobuf(serverID, &GetValidSbPathLight{FQDN: name, Hash1: []byte{0}, Hash2: []byte{0}, Challenge: []byte{}}, reply)

		sbs := reply.Configblocks
		firstconf := sbs[0]
		latestconf := sbs[len(sbs)-1]
		cert := reply.Cert
		pof := reply.PoF

		// Check whether the latest config was recently signed by the Cold Key Holders
		// If not, then check if there exists a "good" PoF signed by the Warm Key Holders
		err := latestconf.CheckTimeDiff(maxdiff * 2)
		if err != nil {
			err = pof.Validate(latestconf, maxdiff)
			if err != nil {
				log.Lvlf2("%v", err)
				return err
			}
		}

		first_not_known_index := 0 // Is sbs[0] the first_not_known skipblock?
		latestconfHash, _ := latestconf.Hash()
		firstconfHash, _ := firstconf.Hash()
		for index, sb := range sbs {
			hash, _ := sb.Hash()
			if bytes.Equal(latestconfHash, hash) {
				first_not_known_index = index + 1
				break
			}
		}
		log.Lvlf3("first_not_known_index: %v", first_not_known_index)
		log.Lvlf3("len(sbs): %v", len(sbs))

		if first_not_known_index == 1 && len(sbs) == 1 {
			// Not a single one returned skipblock that is not already known & trusted
			same_skipchain = true
		}

		if first_not_known_index >= 1 {
			// At this point, at least one of the returned skipblocks is already trusted
			// and at least another one is not
			sbs_path_to_be_checked := sbs[first_not_known_index-1:]
			ok2, _ := VerifyHopsLight(sbs_path_to_be_checked)
			if ok2 {
				same_skipchain = true
			}
		} else {
			// Not a single one returned skipblock is already known & trusted
			// -> Ask for a valid path between the latest trusted skipblock till the 'first' returned (==sbs[0])
			log.Lvlf3("Latest known skipblock has hash: %v", website.Latest.Hash)

			reply := &GetValidSbPathLightReply{}
			u.WSClient.SendProtobuf(serverID, &GetValidSbPathLight{FQDN: name, Hash1: latestconfHash, Hash2: firstconfHash, Challenge: []byte{}}, reply)

			sbs2 := reply.Configblocks
			if sbs2 != nil {
				sbs_path_to_be_checked := append(sbs2, sbs[1:]...)
				ok2, _ := VerifyHopsLight(sbs_path_to_be_checked)
				if ok2 {
					same_skipchain = true
				}
			}
		}

		if !same_skipchain {

			// With high probability, it was the first hop that didn't verify -> skipchain-switch
			log.Lvlf2("--------------------------------------------------------------------------------")
			log.Lvlf1("-----------------------------SKIPCHAIN-SWITCH!!! (user: %v, site: %v) --------------------", u.UserName, name)
			log.Lvlf2("--------------------------------------------------------------------------------")
			log.Lvlf3("The first hop is not valid -> Start trusting from scratch once the signature of the CA is verified")

			// Check whether the latest config was recently signed by the Cold Key Holders
			// If not, then check if there exists a "good" PoF signed by the Warm Key Holders
			err := latestconf.CheckTimeDiff(maxdiff * 2)
			if err != nil {
				err = pof.Validate(latestconf, maxdiff)
				if err != nil {
					log.Lvlf2("%v", err)
					return err
				}
			}

			// TODO: verify that the "CA" is indeed a CA (maybe by checking its public key's membership
			// into a trusted pull of CAs' public keys?)
			/*
				// UNCOMMENT IF CAs ARE TO BE USED
				// Verify that the cert is certifying the config of the 'first' skipblock
				_, data, _ = network.Unmarshal(first.Data)
				firstconf, _ := data.(*common_structs.Config)
				firstconf_hash, _ := firstconf.Hash()
				cert_hash := cert.Hash // should contain the certified config's hash
				if !bytes.Equal(cert_hash, firstconf_hash) {
					log.Lvlf2("Received cert does not point to the first returned skipblock's config!")
					return fmt.Errorf("Received cert does not point to the first returned skipblock's config!")
				}

				// Check that the returned cert is pointing to the requested FQDN
				fqdn := firstconf.FQDN
				if res := strings.Compare(fqdn, name); res != 0 {
					return fmt.Errorf("Returned cert validates another mapping (wrong FQDN) -> Cannot reconnect to the site: %v", name)
				}

				// Check that the returned cert has not yet expired
				expired := firstconf.ExpiredCertConfig()
				if expired {
					return fmt.Errorf("Expired cert -> Cannot reconnect to the site: %v", name)
				}

				// Validate the signature of the CA
				err = crypto.VerifySchnorr(network.Suite, cert.Public, cert_hash, *cert.Signature)
				if err != nil {
					log.Lvlf2("CA's signature doesn't verify")
					return errors.New("CA's signature doesn't verify")
				}
			*/

			// Verify the hops starting from the skipblock for which the cert was issued
			ok, _ := VerifyHopsLight(sbs)
			if !ok {
				return errors.New("Got an invalid skipchain -> ABORT without following it")
			}

			website.PinState.Window = int64(1)
			//website.PinState.Window = int64(86400)
			log.Lvlf2("Start trusting the site by setting trust_window: %v", website.PinState.Window)
			website.PinState.TimePinAccept = time.Now().Unix() * 1000
			u.WebSites[name] = website
			u.Follow(name, latestconf, cert)

		}

		if same_skipchain {
			// As we keep following the previously following site skipchain,
			// the trust window should be doubled (we don't have to check the validity of the cert)
			log.Lvlf1("Got the SAME SKIPCHAIN (user: %v, site: %v) -> double trust window", username, name)
			website.PinState.Window = website.PinState.Window * 2
			website.PinState.TimePinAccept = time.Now().Unix() * 1000
			u.WebSites[name] = website
			u.Follow(name, latestconf, cert)
		}

	} else {

		var err error
		log.Lvlf1("Pins are still valid (user: %v, site: %v)", username, name)
		// follow the evolution of the site skipchain to get the latest valid tls keys
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID

		bytess, _ := GenerateRandomBytes(10)
		challenge := []byte(bytess)
		log.Lvlf3("Challenged web server has address: %v", serverID)

		reply := &GetValidSbPathLightReply{}
		err = u.WSClient.SendProtobuf(serverID, &GetValidSbPathLight{FQDN: name, Hash1: website.Latest.Hash, Hash2: []byte{0}, Challenge: challenge}, reply)
		log.ErrFatal(err)
		log.Print("CLIENT After sendprotobuf")
		sbs := reply.Configblocks
		pof := reply.PoF
		latestconf := sbs[len(sbs)-1]
		sig := reply.Signature

		ok, _ := VerifyHopsLight(sbs)
		if !ok {
			log.LLvlf2("Updating the site config was not possible due to corrupted skipblock chain")
			return errors.New("Updating the site config was not possible due to corrupted skipblock chain")
		}
		latestconfHash, _ := latestconf.Hash()
		log.Lvlf2("user %v, Latest returned block has hash: %v", u.UserName, latestconfHash)
		// Check whether the latest config was recently signed by the Cold Key Holders
		// If not, then check if there exists a "good" PoF signed by the Warm Key Holders
		err = latestconf.CheckTimeDiff(maxdiff * 2)
		if err != nil {
			err = pof.Validate(latestconf, maxdiff)
			if err != nil {
				log.LLvlf2("%v", err)
				return err
			}
		}


		key := fmt.Sprintf("tls:%v", serverID)
		ptls := latestconf.Data[key].TLSPublic
		err = crypto.VerifySchnorr(network.Suite, ptls, challenge, *sig)
		if err != nil {
			log.LLvlf2("user %v, Tls public key (%v) should match to the webserver's %v private key but it does not! (latest returned block has hash: %v)", u.UserName, ptls, serverID, latestconfHash)
			return fmt.Errorf("Tls public key (%v) should match to the webserver's private key but it does not!", ptls)
		}
		log.LLvlf2("Tls private key matches")
		u.Follow(name, latestconf, nil)

	}
	log.LLvlf2("RE-CONNECTED: user: %v, site: %v", u.UserName, name)
	return nil
}

func VerifyHopsLight(blocks []*common_structs.Config) (bool, error) {
	// Check the validity of each skipblock hop
	log.Print("Fetched blocks", len(blocks))
	trustedconfig := blocks[0]
	for index, block := range blocks {
		newconfig := block
		if index > 0 {
			prevHash, _ := trustedconfig.Hash()
			nextHash, _ := newconfig.Hash()
			log.Lvlf2("Checking trust delegation: %v -> %v (%v -> %v)", index-1, index, prevHash, nextHash)
			cnt := 0

			for key, newdevice := range newconfig.Device {
				if _, exists := trustedconfig.Device[key]; exists {
					b1, _ := network.Marshal(newdevice.Point)
					b2, _ := network.Marshal(trustedconfig.Device[key].Point)
					if bytes.Equal(b1, b2) {
						if newdevice.Vote != nil {
							err := crypto.VerifySchnorr(network.Suite, newdevice.Point, nextHash, *newdevice.Vote)
							if err != nil {
								log.Lvlf2("Wrong signature")
								return false, errors.New("Wrong signature")
							}
							cnt++
						}
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				log.Lvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				return false, errors.New("No sufficient threshold of trusted devices' votes")
			}
		}
		trustedconfig = newconfig.Copy()
	}
	return true, nil
}


func (website *WebSite) ExpiredPins() bool {
	now := time.Now().Unix() * 1000
	log.Lvlf3("Now: %v, TimePinAccept: %v, Window: %v", now, website.PinState.TimePinAccept, website.PinState.Window)
	if now-website.PinState.TimePinAccept > website.PinState.Window {
		log.Lvl2(now - website.PinState.TimePinAccept)
		log.Lvl2(website.PinState.Window)
		return true
	}
	return false
}

func (u *User) Follow(name string, conf *common_structs.Config, cert *common_structs.Cert) {

	website := u.WebSites[name]
	website.Config = conf

	// updating PinState
	website.PinState.Threshold = conf.Threshold
	pins := make([]abstract.Point, 0)
	for _, dev := range conf.Device {
		pins = append(pins, dev.Point)
	}
	website.PinState.Pins = pins

	if cert != nil {
		website.Cert = cert
	}

	u.WebSites[name] = website

	log.Lvlf2("Follow(): End")
	return
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
