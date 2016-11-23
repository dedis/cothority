package webserver

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/config"
	//"github.com/dedis/crypto/cosi"
)

/*
This is the external API to access the identity-service. It shows the methods
used to create a new identity-skipchain, propose new configurations and how
to vote on these configurations.
*/

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
	WebSites map[string]*WebSite
}

type WebSite struct {
	sync.Mutex
	// Site's ID (hash of the genesis block)
	ID skipchain.SkipBlockID
	// Config is the latest, valid configuration of the site.
	Config *common_structs.Config
	// Latest known skipblock
	Latest *skipchain.SkipBlock
	// Pin-state
	PinState *common_structs.PinState
	Certs    []*ca.Cert
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
		log.LLvlf2("NewAttachments(): Trying to attach to site: %v", siteInfo.ID)
		err := u.UserAttachTo(siteInfo)
		if err != nil {
			log.Lvlf2("%v", err)
		}
	}
	return
}

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
			Window: int64(86400), // 86400ms = 1 day * 24 hours/day * 3600 sec/hour
			//Window: int64(0),
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
		/*_, data, _ = network.UnmarshalRegistered(sbs[0].Data)
		conf, _ := data.(*common_structs.Config)
		hash, _ = conf.Hash()
		log.LLvlf2("h1=%v, h2=%v", cert_hash, hash)
		if !bytes.Equal(cert_hash, hash) {
			log.LLvlf2("Certs do not point to the first returned skipblock's config")
			return fmt.Errorf("Certs do not point to the first returned skipblock's config")
		}
		*/

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
	website.PinState.TimeSbRec = time.Now().Unix()
	u.WebSites[string(siteInfo.ID)] = &website
	log.LLvlf2("%v has been attached to the site identity: %v", u.UserName, siteInfo.ID)
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
			log.Lvlf2("Checking trust delegation: %v -> %v", index-1, index)
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
								log.Lvlf2("Couldn't get hash")
								return false, errors.New("Couldn't get hash")
							}
							fmt.Println("Verify signature of device: ", key)
							err = crypto.VerifySchnorr(network.Suite, newdevice.Point, hash, *newdevice.Vote)
							if err != nil {
								log.Lvlf2("Wrong signature")
								return false, errors.New("Wrong signature")
							}
							cnt++
							//fmt.Println(cnt)
						}
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				log.Lvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				return false, errors.New("No sufficient threshold of trusted devices' votes")
			}
		}
		prev = next
		_, data, _ := network.UnmarshalRegistered(prev.Data)
		trustedconfig = data.(*common_structs.Config)
	}
	return true, nil
}

// Attempt to RE-visit a site with site identity 'ID' (hash of the genesis skipblock)
func (u *User) ReConnect(id skipchain.SkipBlockID) error {
	log.LLvlf2("Reconnecting user: %v to the site id: %v", u.UserName, id)
	// Check whether we are trying or not to (re)visit a not yet attached site
	if _, exists := u.WebSites[string(id)]; !exists {
		log.LLvlf2("Trying to (re)visit a not yet attached site (site identity: %v)", id)
		return fmt.Errorf("Trying to (re)visit a not yet attached site (site identity: %v)", id)
	}

	website := u.WebSites[string(id)]

	// if now > rec + window then the pins have already expired ->  start from scratch (get
	// new pins, certs etc) and check whether we are still following the previously following
	// site skipchain
	if expired := website.ExpiredPins(); expired {
		// TODO: chech whether the certs are signed or not by keys that are known to belong
		// to existing CAs
		log.LLvlf2("Pins have expired")
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID
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
			/*	_, data, _ = network.UnmarshalRegistered(sbs[0].Data)
				conf, _ := data.(*common_structs.Config)
				hash, _ = conf.Hash()
				if !bytes.Equal(certs[0].Hash, hash) {
					log.Lvlf2("Certs do not point to the first returned skipblock's config")
					return fmt.Errorf("Certs do not point to the first returned skipblock's config")
				}
			*/
			// Verify the hops between each pair of subsequent blocks from the first one returned to the latest
			_, err = VerifyHops(sbs)
			if err != nil {
				log.Lvlf2("%v", err)
				return err
			}

			// check whether we are still following the previously following site skipchain
			msg, _ = u.WSClient.Send(serverID, &GetValidSbPath{ID: id, Sb1: website.Latest, Sb2: sbs[len(sbs)-1]})
			reply3, _ := msg.Msg.(GetValidSbPathReply)
			sbs = reply3.Skipblocks

			ok, _ := VerifyHops(sbs)
			if !ok {
				// start trusting from scratch
				log.Lvlf2("Start trusting from scratch")
				website.PinState.Window = int64(86400)
			} else {
				// as we keep following the previously following site skipchain,
				// the trust window should be doubled
				log.Lvlf2("Doubled trust window")
				website.PinState.Window = website.PinState.Window * 2
			}

			u.Follow(id, reply.Latest, reply.Certs)

			/*
				website.Latest = reply.Latest
				website.Config = latestconf
				website.Certs = reply.Certs

				website.PinState.Threshold = latestconf.Threshold
				pins := make([]abstract.Point, 0)
				for _, dev := range latestconf.Device {
					pins = append(pins, dev.Point)
				}
				website.PinState.Pins = pins
				website.PinState.TimeSbRec = time.Now().Unix()
				u.WebSites[string(id)] = website
			*/
		}

	} else {
		var err error
		log.LLvlf2("Pins still valid")
		// follow the evolution of the site skipchain to get the latest valid tls keys
		wss := website.WSs
		serverID := wss[rand.Int()%len(wss)].ServerID
		msg, _ := u.WSClient.Send(serverID, &GetValidSbPath{ID: id, Sb1: website.Latest, Sb2: nil})
		reply, _ := msg.Msg.(GetValidSbPathReply)
		sbs := reply.Skipblocks

		ok, _ := VerifyHops(sbs)
		if !ok {
			log.Lvlf2("Updating the site config was not possible due to corrupted skipblock chain")
			return errors.New("Updating the site config was not possible due to corrupted skipblock chain")
		}

		u.Follow(id, sbs[len(sbs)-1], nil)

		// pins still valid, try to use the existent tls public key in order to communicate
		// with the webserver
		key := fmt.Sprintf("tls:%v", serverID)
		ptls := website.Config.Data[key]
		bytess, _ := GenerateRandomBytes(10)
		challenge := crypto.HashID(bytess)
		log.LLvlf2("Challenged web server has address: %v", serverID)
		msg, err = u.WSClient.Send(serverID, &ChallengeReq{ID: id, Challenge: challenge})
		if err != nil {
			return err
		}
		reply2, _ := msg.Msg.(ChallengeReply)
		sig := reply2.Signature
		err = crypto.VerifySchnorr(network.Suite, ptls, challenge, *sig)
		if err != nil {
			log.LLvlf2("Tls public key should match to the webserver's private key but it does not!")
			return errors.New("Tls public key should match to the webserver's private key but it does not!")
		}
		log.LLvlf2("Tls private key matches")

	}
	return nil
}

func (website *WebSite) ExpiredPins() bool {
	now := time.Now().Unix()
	if now-website.PinState.TimeSbRec > website.PinState.Window {
		log.LLvl2(now - website.PinState.TimeSbRec)
		log.LLvl2(website.PinState.Window)
		return true
	}
	return false
}

func CertPointers(certs []*ca.Cert) (crypto.HashID, error) {
	//log.LLvlf2("CertPointers(): Start")
	// TODO: what happens when returned certs are pointing to different configs?????
	// Check that the certs are pointing to the same config
	ok := true
	prev := certs[0]
	for index, cert := range certs {
		curr := cert
		if index > 0 {
			if !bytes.Equal(prev.Hash, curr.Hash) {
				log.Lvlf2("Certs are not pointing to the same config")
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

func (u *User) Follow(id skipchain.SkipBlockID, block *skipchain.SkipBlock, certs []*ca.Cert) {
	_, data, _ := network.UnmarshalRegistered(block.Data)
	conf, _ := data.(*common_structs.Config)

	website := u.WebSites[string(id)]
	website.Config = conf
	website.Latest = block

	// updating PinState
	website.PinState.Threshold = conf.Threshold
	pins := make([]abstract.Point, 0)
	for _, dev := range conf.Device {
		pins = append(pins, dev.Point)
	}
	website.PinState.Pins = pins
	website.PinState.TimeSbRec = time.Now().Unix()

	if certs != nil {
		website.Certs = certs
	}

	// TODO: what happens with webserver insertions/deletions
	// & what happens with fresh certs? (the associated fields should
	// be updated)

	u.WebSites[string(id)] = website
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
