package webserver

import (
	"fmt"
	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/abstract"
	//"time"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/sidentity"
	"github.com/dedis/crypto/config"
	//"github.com/stretchr/testify/assert"
	//"io/ioutil"
	//"os"
	"testing"
)

func NewTestIdentity(cothority *sda.Roster, majority int, owner string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]abstract.Point, local *sda.LocalTest) *sidentity.Identity {
	id := sidentity.NewIdentity(cothority, majority, owner, pinstate, cas, data)
	id.CothorityClient = local.NewClient(sidentity.ServiceName)
	return id
}

func NewTestIdentityMultDevs(cothority *sda.Roster, majority int, owners []string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]abstract.Point, local *sda.LocalTest) []*sidentity.Identity {
	ids, _ := sidentity.NewIdentityMultDevs(cothority, majority, owners, pinstate, cas, data)
	for _, id := range ids {
		id.CothorityClient = local.NewClient(sidentity.ServiceName)
	}
	return ids
}

func NewTestUser(username string, sitesToAttach []*common_structs.SiteInfo, local *sda.LocalTest) *User {
	u := NewUser(username, sitesToAttach)
	u.WSClient = local.NewClient(ServiceWSName)
	return u
}

func Test1(t *testing.T) {
	l := sda.NewTCPTest()
	hosts_coth, el_coth, _ := l.GenTree(5, true)
	services := l.GetServices(hosts_coth, sidentity.IdentityService)
	for _, s := range services {
		log.Lvl3(s.(*sidentity.Service).Identities)
	}

	hosts_ca, _, _ := l.GenTree(2, true)
	services = l.GetServices(hosts_ca, ca.CAService)
	cas := make([]common_structs.CAInfo, 0)
	for index, h := range hosts_ca {
		cas = append(cas, common_structs.CAInfo{Public: services[index].(*ca.CA).Public, ServerID: h.ServerIdentity})
	}

	thr := 1
	log.Print("NEW SITE IDENTITY")
	pinstate := &common_structs.PinState{Ctype: "device"}
	// include into the config the tls public key of one web server
	hosts_ws, _, _ := l.GenTree(1, true)
	wss := make([]common_structs.WSInfo, 0)
	data := make(map[string]abstract.Point)
	keypairs := make([]*config.KeyPair, 0)
	for _, h := range hosts_ws {
		wss = append(wss, common_structs.WSInfo{ServerID: h.ServerIdentity})
		keypair := config.NewKeyPair(network.Suite)
		keypairs = append(keypairs, keypair)
		key := fmt.Sprintf("tls:%v", h.ServerIdentity)
		log.LLvlf2("%v", key)
		data[key] = keypair.Public
	}

	c1 := NewTestIdentity(el_coth, thr, "one", pinstate, cas, data, l)
	log.ErrFatal(c1.CreateIdentity())

	log.Print("")
	log.Print("ADDING SECOND DEVICE")
	pinstate = &common_structs.PinState{Ctype: "device"}
	c2 := NewTestIdentity(el_coth, thr, "two", pinstate, nil, nil, l)
	c2.AttachToIdentity(c1.ID)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	log.LLvlf2("ATTACHING ONE WEB SERVER TO THE SITE IDENTITY: %v", c1.ID)
	services = l.GetServices(hosts_ws, WSService)
	for _, s := range services {
		ws := s.(*WS)
		log.ErrFatal(ws.WSAttach(el_coth, c1.ID, keypairs[0].Public, keypairs[0].Secret))
		site := ws.getSiteStorage(c1.ID)
		log.LLvlf2("WS has private: %v, public: %v for site id: %v", site.Private, site.Public, c1.ID)
	}

	defer l.CloseAll()

	log.LLvlf2("ATTACHING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	siteInfo := &common_structs.SiteInfo{
		ID:  c1.ID,
		WSs: wss,
	}
	sitestoattach := make([]*common_structs.SiteInfo, 0)
	sitestoattach = append(sitestoattach, siteInfo)
	u1 := NewTestUser("user1", sitestoattach, l)
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}
	log.LLvlf2("%v", c1.Public)
	log.LLvlf2("%v", c2.Public)

	thr = 2
	log.Print("")
	log.Lvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	log.Print("")
	log.Print("ADDING THIRD DEVICE")
	c3 := NewTestIdentity(el_coth, thr, "three", pinstate, nil, nil, l)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	c1.ProposeUpVote()
	c2.ProposeUpVote()
	log.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c1.Config.Device))
	}

	log.Print("")
	log.Print("REVOKING FIRST IDENTITY")
	c3.ConfigUpdate()
	add := make(map[string]abstract.Point)
	revoke := make(map[string]abstract.Point)
	n := "one"
	if _, exists := c3.Config.Device[n]; exists {
		revoke[n] = c3.Config.Device[n].Point
		c3.ProposeConfig(add, revoke, thr)
		c3.ProposeUpVote()
		c1.ProposeUpdate()
		c1.ProposeVote(false)
		c2.ProposeUpdate()
		c2.ProposeVote(true)
		log.ErrFatal(c2.ConfigUpdate())
		if len(c2.Config.Device) != 2 {
			t.Fatal("Should have two owners by now")
		}
		c3.ConfigUpdate()
		if _, exists := c3.Config.Device[n]; exists {
			t.Fatal("Device one should have been revoked by now")
		}
	}

	if len(c3.Certs) != len(cas) {
		t.Fatalf("Should have %v certs by now", len(cas))
	}

	log.LLvlf2("ATTACHING USER2 TO THE SITE IDENTITY: %v", c1.ID)
	siteInfo = &common_structs.SiteInfo{
		ID:  c1.ID,
		WSs: wss,
	}
	sitestoattach = make([]*common_structs.SiteInfo, 0)
	sitestoattach = append(sitestoattach, siteInfo)
	u2 := NewTestUser("user2", sitestoattach, l)
	log.LLvlf2("user2's pins")
	for _, site := range u2.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	u1.ReConnect(c1.ID)
	log.LLvlf2("user1's pins")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("c1's public key: %v", c1.Public)
	log.LLvlf2("c2's public key: %v", c2.Public)
	log.LLvlf2("c3's public key: %v", c3.Public)

}

/*
func TestGenesisWithMultipleDevices(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, sidentity.IdentityService)
	for _, s := range services {
		log.Lvl3(s.(*sidentity.Service).Identities)
	}

	hosts, _, _ = l.GenTree(2, true)
	services = l.GetServices(hosts, ca.CAService)
	defer l.CloseAll()

	cas := make([]common_structs.CAInfo, 0)
	for index, h := range hosts {
		cas = append(cas, common_structs.CAInfo{Public: services[index].(*ca.CA).Public, ServerID: h.ServerIdentity})
	}

	thr := 1
	log.Print("NEW SITE IDENTITY")
	pinstate := &common_structs.PinState{Ctype: "device"}
	c := NewTestIdentityMultDevs(el, thr, []string{"one", "two"}, pinstate, cas, l)
	c1 := c[0]
	c2 := c[1]
	log.ErrFatal(c1.CreateIdentityMultDevs(c))

	log.Print("ADDING THIRD DEVICE")
	pinstate = &common_structs.PinState{Ctype: "device"}
	c3 := NewTestIdentity(el, thr, "three", pinstate, nil, l)
	c3.AttachToIdentity(c1.ID)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 3 {
		t.Fatal("Should have three owners by now")
	}

	thr = 2
	log.Lvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	log.Print("ADDING FOURTH DEVICE")
	c4 := NewTestIdentity(el, thr, "four", pinstate, nil, l)
	log.ErrFatal(c4.AttachToIdentity(c1.ID))
	c1.ProposeUpVote()
	c2.ProposeUpVote()
	log.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Device) != 4 {

		t.Fatal("Should have four owners by now but got only: ", len(c1.Config.Device))
	}

	log.Print("REVOKING FIRST IDENTITY")
	c3.ConfigUpdate()
	add := make(map[string]abstract.Point)
	revoke := make(map[string]abstract.Point)
	n := "one"
	if _, exists := c3.Config.Device[n]; exists {
		revoke[n] = c3.Config.Device[n].Point
		c3.ProposeConfig(add, revoke, thr)
		c3.ProposeUpVote()
		c1.ProposeUpdate()
		c1.ProposeVote(false)
		c4.ProposeUpdate()
		c4.ProposeVote(true)
		log.ErrFatal(c2.ConfigUpdate())
		if len(c2.Config.Device) != 3 {
			t.Fatal("Should have three owners by now")
		}
		c3.ConfigUpdate()
		if _, exists := c3.Config.Device[n]; exists {
			t.Fatal("Device one should have been revoked by now")
		}

	}

	if len(c3.Certs) != len(cas) {
		t.Fatalf("Should have %v certs by now", len(cas))
	}

	timestamp := c3.Config.Timestamp
	diff := time.Since(time.Unix(timestamp, 0))
	log.Lvlf2("Time elapsed since latest skipblock's timestamp: %v", diff)

}
*/
