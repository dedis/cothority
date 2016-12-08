package webserver

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/config"
	//"github.com/dedis/crypto/cosi"
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
		&Update{},
		&UpdateReply{},
		&GetSkipblocks{},
		&GetSkipblocksReply{},
		&GetValidSbPath{},
		&GetValidSbPathReply{},
		&ChallengeReq{},
		&ChallengeReply{},
	} {
		network.RegisterPacketType(s)
	}
}

type User struct {
	// WSClient is included for easy `Send`-methods.
	WSClient *sda.Client
	// Data holds all the data related to this web site user
	// It can be stored and loaded from a config file.
	Data
}

type Data struct {
	UserName string
	*WebSiteMap
}

// WebSiteMap holds the map to the sites so it can be marshaled.
type WebSiteMap struct {
	WebSites map[string]*WebSite // site's FQDN is the key of the map
}

type WebSite struct {
	sync.Mutex
	// Site's FQDN
	FQDN string
	// Site's ID (hash of the genesis block)
	//ID skipchain.SkipBlockID
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
		WSClient: sda.NewClient(ServiceWSName),
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
		log.LLvlf2("NewAttachments(): Trying to attach to site: %v", siteInfo.FQDN)
		err := u.Connect(siteInfo)
		if err != nil {
			log.LLvlf2("%v", err)
		}
	}
	return
}

/*
func (u *User) UserAttachTo(siteInfo *common_structs.SiteInfo) error {
	log.LLvlf2("UserAttachTo(): Start")
	// Check whether we are trying or not to re-attach the user to a site identity
	if _, exists := u.WebSites[string(siteInfo.ID)]; exists {
		log.Lvlf2("Trying to re-attach the user to the site identity: %v", siteInfo.ID)
		return fmt.Errorf("Trying to re-attach the user to the site identity: %v", siteInfo.ID)
	}

	website := WebSite{
		ID:  siteInfo.ID,
		WSs: siteInfo.WSs,
		PinState: &common_structs.PinState{
			//Window: int64(86400), // 86400ms = 1 day * 24 hours/day * 3600 sec/hour
			Window: int64(1),
		},
	}

	wss := website.WSs
	serverID := wss[rand.Int()%len(wss)].ServerID
	msg, err := u.WSClient.Send(serverID, &Connect{ID: siteInfo.ID})
	if err != nil {
		return err
	}
	reply := msg.Msg.(ConnectReply)
	latest := reply.Latest
	certs := reply.Certs

	_, data, _ := network.UnmarshalRegistered(latest.Data)
	latestconf, _ := data.(*common_structs.Config)

	// Check whether the 'latest' skipblock is stale or not
	err = latestconf.CheckTimeDiff(maxdiff)
	if err != nil {
		log.LLvlf2("Stale skipblock can not be accepted")
		return fmt.Errorf("Stale skipblock can not be accepted")
	}

	// Check whether the certs are pointing to the same config or not
	var cert_hash crypto.HashID
	cert_hash, err = CertPointers(certs)
	if err != nil {
		log.LLvlf2("%v", err)
		return err
	}

	// Check whether the certs are pointing to the returned skipblock's config or not
	// If the certs are not pointing to the returned skipblock's config, fetch all the blocks
	// starting with the one for which the certs were issued. Then, validate the hops between
	// each pair of subsequent blocks from the first one returned to the latest
	hash, _ := latestconf.Hash()
	if !bytes.Equal(cert_hash, hash) {
		log.LLvlf2("siteID=%v, LatestID=%v", siteInfo.ID, latest.Hash)
		log.LLvlf2("Received certs are not pointing to the received skipblock, try to fetch the whole skipblock chain")
		msg, _ = u.WSClient.Send(serverID, &GetSkipblocks{ID: siteInfo.ID, Latest: latest})
		reply2, _ := msg.Msg.(GetSkipblocksReply)
		sbs := reply2.Skipblocks
		log.LLvlf2("num of returned blocks: %v", len(sbs))

		// Check that the certs do indeed point to the first returned block's config
		//_, data, _ = network.UnmarshalRegistered(sbs[0].Data)
		//conf, _ := data.(*common_structs.Config)
		//hash, _ = conf.Hash()
		//log.LLvlf2("h1=%v, h2=%v", cert_hash, hash)
		//if !bytes.Equal(cert_hash, hash) {
		//	log.LLvlf2("Certs do not point to the first returned skipblock's config")
		//	return fmt.Errorf("Certs do not point to the first returned skipblock's config")
		//}


		// TODO: check that (a) valid CA(s) has/have signed the cert(s)

		// Verify the hops between each pair of subsequent blocks from the first one returned to the latest
		_, err = VerifyHops(sbs)
		if err != nil {
			log.Lvlf2("%v", err)
			return err
		}
	}

	// Accept skipblock/certs
	website.Latest = reply.Latest
	website.Config = latestconf
	website.Certs = reply.Certs

	// Set the pinstate (at this point, the trust 'Window' is already set to 86400sec <-> 1 day)
	website.PinState.Threshold = latestconf.Threshold
	pins := make([]abstract.Point, 0)
	for _, dev := range latestconf.Device {
		pins = append(pins, dev.Point)
	}
	website.PinState.Pins = pins
	website.PinState.TimePinAccept = time.Now().Unix() * 1000
	u.WebSites[string(siteInfo.ID)] = &website
	log.LLvlf2("%v has been attached to the site identity: %v", u.UserName, siteInfo.ID)
	return nil
}
*/

func (u *User) Connect(siteInfo *common_structs.SiteInfo) error {
	log.LLvlf2("Connect(): Start")
	//id := siteInfo.ID
	name := siteInfo.FQDN
	// Check whether we are trying or not to re-attach the user to a site identity
	//if _, exists := u.WebSites[string(id)]; exists {
	if _, exists := u.WebSites[name]; exists {
		log.LLvlf2("Trying to re-attach the user to site: %v", name)
		return fmt.Errorf("Trying to re-attach the user to site: %v", name)
	}

	website := WebSite{
		//ID:  id,
		FQDN: name,
		WSs:  siteInfo.WSs,
		PinState: &common_structs.PinState{
			//Window: int64(86400), // 86400ms = 1 day * 24 hours/day * 3600 sec/hour
			Window: int64(1),
		},
	}

	wss := website.WSs
	serverID := wss[rand.Int()%len(wss)].ServerID

	log.LLvlf2("Connect(): Before fetching skipblocks: serverID=%v, FQDN=%v", serverID, name)
	msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{FQDN: name, Hash1: []byte{0}, Hash2: []byte{0}})
	reply, _ := msg.Msg.(GetValidSbPathReply)
	sbs := reply.Skipblocks
	latest := sbs[len(sbs)-1]
	cert := reply.Cert
	log.LLvlf2("Connect(): Skipblocks fetched")

	// Check whether the 'latest' skipblock is stale or not
	_, data, _ := network.UnmarshalRegistered(latest.Data)
	latestconf, _ := data.(*common_structs.Config)
	err := latestconf.CheckTimeDiff(maxdiff)
	if err != nil {
		log.LLvlf2("Stale skipblock can not be accepted")
		return fmt.Errorf("Stale skipblock can not be accepted")
	}

	// TODO: verify that the CA is indeed a CA (maybe by checking its public key's membership
	// into a trusted pull of CAs' public keys?)

	// Validate the signature of the CA
	cert_hash := cert.Hash // the certified config's hash
	err = crypto.VerifySchnorr(network.Suite, cert.Public, cert_hash, *cert.Signature)
	if err != nil {
		log.LLvlf2("CA's signature doesn't verify")
		return errors.New("CA's signature doesn't verify")
	}

	// Verify that the config of the first returned skipblock has been certified
	_, data, _ = network.UnmarshalRegistered(sbs[0].Data)
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

	// Verify the hops starting from the skipblock (sbs[0]) for which the cert was issued
	ok, _ := VerifyHops(sbs)
	if !ok {
		log.LLvlf2("Not valid hops")
		return fmt.Errorf("Not valid hops")
	} else {
		//website.PinState.Window = int64(86400) // (REALISTIC)
		website.PinState.Window = int64(1)
		log.LLvlf2("Start following the site by setting trust_window: %v", website.PinState.Window)
		website.PinState.TimePinAccept = time.Now().Unix() * 1000
		u.WebSites[name] = &website
		u.Follow(name, latest, cert)
	}
	log.LLvlf2("Connect(): End")
	return nil
}

// Attempt to RE-visit site "name"
func (u *User) ReConnect(name string) error {
	log.LLvlf2("Reconnecting user: %v to site: %v", u.UserName, name)
	// Check whether we are trying or not to (re)visit a not yet attached site
	if _, exists := u.WebSites[name]; !exists {
		log.LLvlf2("Trying to (re)visit a not yet attached site (site's FQDN: %v)", name)
		return fmt.Errorf("Trying to (re)visit a not yet attached site (site's FQDN: %v)", name)
	}

	website := u.WebSites[name]

	// if now > rec + window then the pins have already expired ->  start from scratch (get
	// new pins, certs etc) and check whether we are still following the previously following
	// site skipchain
	if expired := website.ExpiredPins(); expired {

		same_skipchain := false

		log.LLvlf2("Pins have expired!!!")
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID

		//msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{FQDN: name, Hash1: website.Latest.Hash, Hash2: []byte{0}})
		msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{FQDN: name, Hash1: []byte{0}, Hash2: []byte{0}})
		reply, _ := msg.Msg.(GetValidSbPathReply)
		sbs := reply.Skipblocks
		first := sbs[0]
		latest := sbs[len(sbs)-1]
		cert := reply.Cert

		first_not_known_index := 0 // Is sbs[0] the first_not_known skipblock?
		for index, sb := range sbs {
			if bytes.Equal(website.Latest.Hash, sb.Hash) {
				first_not_known_index = index + 1
				break
			}
		}
		log.LLvlf2("first_not_known_index: %v", first_not_known_index)
		log.LLvlf2("len(sbs): %v", len(sbs))

		if first_not_known_index == 1 && len(sbs) == 1 {
			// Not a single one returned skipblock that is not already known & trusted
			same_skipchain = true
		}

		if first_not_known_index >= 1 {
			// At this point, at least one of the returned skipblocks is already trusted
			// and at least another one is not
			sbs_path_to_be_checked := sbs[first_not_known_index-1:]
			ok2, _ := VerifyHops(sbs_path_to_be_checked)
			if ok2 {
				same_skipchain = true
			}
		} else {
			// Not a single one returned skipblock is already known & trusted
			// -> Ask for a valid path between the latest trusted skipblock till the 'first' returned (==sbs[0])
			log.LLvlf2("Latest known skipblock has hash: %v", website.Latest.Hash)
			msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{FQDN: name, Hash1: website.Latest.Hash, Hash2: first.Hash})
			reply, _ := msg.Msg.(GetValidSbPathReply)
			sbs2 := reply.Skipblocks
			if sbs2 != nil {
				sbs_path_to_be_checked := append(sbs2, sbs[1:]...)
				ok2, _ := VerifyHops(sbs_path_to_be_checked)
				if ok2 {
					same_skipchain = true
				}
			}
		}

		if !same_skipchain {

			// With high probability, it was the first hop that didn't verify -> skipchain-switch
			log.LLvlf2("--------------------------------------------------------------------------------")
			log.LLvlf2("-----------------------------SKIPCHAIN-SWITCH!!!--------------------")
			log.LLvlf2("--------------------------------------------------------------------------------")
			log.LLvlf2("The first hop is not valid -> Start trusting from scratch once the signature of the CA is verified")

			// Check whether the 'latest' skipblock is stale or not
			_, data, _ := network.UnmarshalRegistered(latest.Data)
			latestconf, _ := data.(*common_structs.Config)
			err := latestconf.CheckTimeDiff(maxdiff)
			if err != nil {
				log.LLvlf2("Stale skipblock can not be accepted")
				return fmt.Errorf("Stale skipblock can not be accepted")
			}

			// TODO: verify that the "CA" is indeed a CA (maybe by checking its public key's membership
			// into a trusted pull of CAs' public keys?)

			// Verify that the cert is certifying the config of the 'first' skipblock
			_, data, _ = network.UnmarshalRegistered(first.Data)
			firstconf, _ := data.(*common_structs.Config)
			firstconf_hash, _ := firstconf.Hash()
			cert_hash := cert.Hash // should contain the certified config's hash
			if !bytes.Equal(cert_hash, firstconf_hash) {
				log.LLvlf2("Received cert does not point to the first returned skipblock's config!")
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
				log.LLvlf2("CA's signature doesn't verify")
				return errors.New("CA's signature doesn't verify")
			}

			// Verify the hops starting from the skipblock for which the cert was issued
			ok, _ := VerifyHops(sbs)
			if !ok {
				return errors.New("Got an invalid skipchain -> ABORT without following it")
			}

			website.PinState.Window = int64(1)
			//website.PinState.Window = int64(86400)
			log.LLvlf2("Start trusting the site by setting trust_window: %v", website.PinState.Window)
			website.PinState.TimePinAccept = time.Now().Unix() * 1000
			u.WebSites[name] = website
			u.Follow(name, latest, cert)

			/*
					// Find the chain of blocks ('sbs_cert') starting from the one for which the cert was issued
					var start_sb *skipchain.SkipBlock
					start_sb = nil
					var start_index int
					for index, sb := range sbs {
						_, data, _ := network.UnmarshalRegistered(sb.Data)
						conf, _ := data.(*common_structs.Config)
						conf_hash, _ := conf.Hash()

						if bytes.Equal(cert_hash, conf_hash) {
							start_sb = sb
							start_index = index
							break
						}
					}
					if start_sb == nil {
						return fmt.Errorf("Didn't find skipblock that matches the cert")
					}

				// Verify the hops starting from the skipblock for which the cert was issued
				sbs_cert := sbs[start_index:len(sbs)]
				ok, _ := VerifyHops(sbs_cert)
				if !ok {
					log.LLvlf2("Not valid hops")
					return fmt.Errorf("Not valid hops")
				} else {
					website.PinState.Window = int64(1)
					//website.PinState.Window = int64(86400)
					log.LLvlf2("Start trusting the site by setting trust_window: %v", website.PinState.Window)
					website.PinState.TimePinAccept = time.Now().Unix() * 1000
					u.WebSites[name] = website
					u.Follow(name, latest, cert)
				}
			*/

		}

		if same_skipchain {
			// As we keep following the previously following site skipchain,
			// the trust window should be doubled (we don't have to check the validity of the cert)
			log.LLvlf2("Got the SAME SKIPCHAIN -> double trust window")
			website.PinState.Window = website.PinState.Window * 2
			website.PinState.TimePinAccept = time.Now().Unix() * 1000
			u.WebSites[name] = website
			u.Follow(name, latest, cert)
		}

		/*
			msg, err := u.WSClient.Send(serverID, &Connect{ID: id})
			if err != nil {
				return err
			}
			reply, _ := msg.Msg.(ConnectReply)

			latest := reply.Latest
			certs := reply.Certs

			_, data, _ := network.UnmarshalRegistered(latest.Data)
			latestconf, _ := data.(*common_structs.Config)


			// Check whether the certs are pointing to the same config or not
			_, err = CertPointers(certs)
			if err != nil {
				return err
			}

			hash, _ := latestconf.Hash()
			if !bytes.Equal(certs[0].Hash, hash) {
				log.Lvlf2("Received certs are not pointing to the received skipblock, try to fetch the whole skipblock chain")
				msg, _ = u.WSClient.Send(serverID, &GetSkipblocks{ID: id, Latest: latest})
				reply2, _ := msg.Msg.(GetSkipblocksReply)
				sbs := reply2.Skipblocks

				// Check that the certs do indeed point to the first returned block's config
				//	_, data, _ = network.UnmarshalRegistered(sbs[0].Data)
				//	conf, _ := data.(*common_structs.Config)
				//	hash, _ = conf.Hash()
				//	if !bytes.Equal(certs[0].Hash, hash) {
				//		log.Lvlf2("Certs do not point to the first returned skipblock's config")
				//		return fmt.Errorf("Certs do not point to the first returned skipblock's config")
				//	}

				// Verify the hops between each pair of subsequent blocks from the first one returned to the latest
				log.LLvlf2("Verify the hops")
				_, err = VerifyHops(sbs)
				if err != nil {
					log.LLvlf2("%v", err)
					return err
				}

				// check whether we are still following the previously following site skipchain
				log.LLvlf2("Latest trusted block has hash: %v, the current head has hash: %v", website.Latest.Hash, sbs[len(sbs)-1].Hash)
				msg, _ = u.WSClient.Send(serverID, &GetValidSbPath{ID: id, Hash1: website.Latest.Hash, Hash2: sbs[len(sbs)-1].Hash})
				reply3, _ := msg.Msg.(GetValidSbPathReply)
				sbs = reply3.Skipblocks

				log.LLvlf2("Verify the hops2")
				ok, _ := VerifyHops(sbs)
				if !ok {
					// start trusting from scratch
					log.LLvlf2("Start trusting from scratch")
					website.PinState.Window = int64(86400)
				} else {
					// as we keep following the previously following site skipchain,
					// the trust window should be doubled
					log.LLvlf2("Doubled trust window")
					website.PinState.Window = website.PinState.Window * 2
				}
				website.PinState.TimePinAccept = time.Now().Unix() * 1000
				u.WebSites[string(id)] = website

				u.Follow(id, reply.Latest, reply.Certs)
			}
		*/

	} else {
		var err error
		log.LLvlf2("Pins are still valid")
		// follow the evolution of the site skipchain to get the latest valid tls keys
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID
		log.LLvlf2("Challenged web server has address: %v", serverID)
		msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{FQDN: name, Hash1: website.Latest.Hash, Hash2: []byte{0}})
		reply, _ := msg.Msg.(GetValidSbPathReply)
		sbs := reply.Skipblocks
		ok, _ := VerifyHops(sbs)
		if !ok {
			log.Lvlf2("Updating the site config was not possible due to corrupted skipblock chain")
			return errors.New("Updating the site config was not possible due to corrupted skipblock chain")
		}
		u.Follow(name, sbs[len(sbs)-1], nil)

		// Pins still valid, try to use the existent tls public key in order to communicate
		// with the webserver
		key := fmt.Sprintf("tls:%v", serverID)
		ptls := website.Config.Data[key].TLSPublic
		bytess, _ := GenerateRandomBytes(10)
		challenge := crypto.HashID(bytess)
		msg, err = u.WSClient.Send(serverID, &ChallengeReq{FQDN: name, Challenge: challenge})
		if err != nil {
			return err
		}
		reply2, _ := msg.Msg.(ChallengeReply)
		sig := reply2.Signature
		err = crypto.VerifySchnorr(network.Suite, ptls, challenge, *sig)
		if err != nil {
			log.LLvlf2("Tls public key (%v) should match to the webserver's private key but it does not!", ptls)
			return fmt.Errorf("Tls public key (%v) should match to the webserver's private key but it does not!", ptls)
		}
		log.LLvlf2("Tls private key matches")

	}
	return nil
}

func VerifyHops(blocks []*skipchain.SkipBlock) (bool, error) {
	// Check the validity of each skipblock hop
	prev := blocks[0]
	_, data, _ := network.UnmarshalRegistered(prev.Data)
	trustedconfig := data.(*common_structs.Config)
	for index, block := range blocks {
		next := block
		if index > 0 {
			log.LLvlf2("Checking trust delegation: %v -> %v (%v -> %v)", index-1, index, prev.Hash, next.Hash)
			cnt := 0
			_, data, err2 := network.UnmarshalRegistered(next.Data)
			if err2 != nil {
				return false, errors.New("Couldn't unmarshal subsequent skipblock's SkipBlockFix field")
			}
			newconfig, ok := data.(*common_structs.Config)
			if !ok {
				return false, errors.New("Couldn't get type '*Config'")
			}

			for key, newdevice := range newconfig.Device {
				if _, exists := trustedconfig.Device[key]; exists {
					b1, _ := network.MarshalRegisteredType(newdevice.Point)
					b2, _ := network.MarshalRegisteredType(trustedconfig.Device[key].Point)
					if bytes.Equal(b1, b2) {
						//fmt.Println("Check whether there is a non-nil signature")
						if newdevice.Vote != nil {
							var hash crypto.HashID
							hash, err := newconfig.Hash()
							if err != nil {
								log.LLvlf2("Couldn't get hash")
								return false, errors.New("Couldn't get hash")
							}
							//log.LLvlf2("Verify signature of device: %v", key)
							err = crypto.VerifySchnorr(network.Suite, newdevice.Point, hash, *newdevice.Vote)
							if err != nil {
								log.LLvlf2("Wrong signature")
								return false, errors.New("Wrong signature")
							}
							cnt++
							//fmt.Println(cnt)
						}
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				log.LLvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				return false, errors.New("No sufficient threshold of trusted devices' votes")
			}
		}
		prev = next
		_, data, _ := network.UnmarshalRegistered(prev.Data)
		trustedconfig = data.(*common_structs.Config)
	}
	return true, nil
}
func (website *WebSite) ExpiredPins() bool {
	now := time.Now().Unix() * 1000
	log.LLvlf2("Now: %v, TimePinAccept: %v, Window: %v", now, website.PinState.TimePinAccept, website.PinState.Window)
	if now-website.PinState.TimePinAccept > website.PinState.Window {
		log.LLvl2(now - website.PinState.TimePinAccept)
		log.LLvl2(website.PinState.Window)
		return true
	}
	return false
}

func (u *User) Follow(name string, block *skipchain.SkipBlock, cert *common_structs.Cert) {
	_, data, _ := network.UnmarshalRegistered(block.Data)
	conf, _ := data.(*common_structs.Config)

	website := u.WebSites[name]
	website.Config = conf
	website.Latest = block

	// updating PinState
	website.PinState.Threshold = conf.Threshold
	pins := make([]abstract.Point, 0)
	for _, dev := range conf.Device {
		pins = append(pins, dev.Point)
	}
	website.PinState.Pins = pins
	//website.PinState.TimePinAccept = time.Now().Unix() * 1000

	if cert != nil {
		website.Cert = cert
	}

	// TODO: what happens with webserver insertions/deletions
	// & what happens with fresh certs? (the associated fields should
	// be updated)

	u.WebSites[name] = website
	log.LLvlf2("Follow(): End")
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

/*
func CertPointers(certs []*ca.Cert) (crypto.HashID, error) {
	//log.LLvlf2("CertPointers(): Start")
	// TODO: what happens when returned certs are pointing to different configs?????
	// Check that the certs are pointing to the same config
	ok := true
	prev := certs[0]
	for index, cert := range certs {
		log.LLvlf2("index: %v", index)
		curr := cert
		if index > 0 {
			if !bytes.Equal(prev.Hash, curr.Hash) {
				log.LLvlf2("Certs are not pointing to the same config")
				log.LLvlf2("hash%v: %v, hash%v: %v", index-1, prev.Hash, index, curr.Hash)
				ok = false
				break
			}
		}
		prev = curr
	}
	if !ok {
		return nil, fmt.Errorf("Certs are not pointing to the same config")
	}
	hash := prev.Hash
	//log.LLvlf2("CertPointers(): End")
	return hash, nil
}
*/
