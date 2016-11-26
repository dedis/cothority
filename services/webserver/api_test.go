package webserver

import (
	"fmt"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/sidentity"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"time"
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

func AttachWebServersToSite(id skipchain.SkipBlockID, hosts_ws []*sda.Conode, el_coth *sda.Roster, keypairs []*config.KeyPair, l *sda.LocalTest) {
	log.LLvlf2("")
	log.LLvlf2("ATTACHING WEB SERVERS TO THE SITE IDENTITY: %v", id)
	services := l.GetServices(hosts_ws, WSService)
	for index, s := range services {
		ws := s.(*WS)
		log.ErrFatal(ws.WSAttach(el_coth, id, keypairs[index].Public, keypairs[index].Secret))
		site := ws.getSiteStorage(id)
		log.LLvlf2("WS has private: %v, public: %v for site id: %v", site.Private, site.Public, id)
	}
	return
}

func GenerateWSPublicKeys(num_ws int, l *sda.LocalTest) ([]*sda.Conode, []common_structs.WSInfo, []*config.KeyPair, map[string]abstract.Point) {
	hosts_ws, _, _ := l.GenTree(num_ws, true)
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
	return hosts_ws, wss, keypairs, data
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

func Test2(t *testing.T) {
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
	hosts_ws, _, _ := l.GenTree(5, true)
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

	time.Sleep(1000 * time.Millisecond)
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

	log.LLvlf2("ATTACHING WEB SERVERS TO THE SITE IDENTITY: %v", c1.ID)
	services = l.GetServices(hosts_ws, WSService)
	for index, s := range services {
		ws := s.(*WS)
		log.ErrFatal(ws.WSAttach(el_coth, c1.ID, keypairs[index].Public, keypairs[index].Secret))
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
	log.LLvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	time.Sleep(1000 * time.Millisecond)
	log.Print("")
	log.LLvlf2("ADDING THIRD DEVICE")
	c3 := NewTestIdentity(el_coth, thr, "three", pinstate, nil, nil, l)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	c1.ProposeUpVote()
	c2.ProposeUpVote()
	log.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c1.Config.Device))
	}

	time.Sleep(1000 * time.Millisecond)
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

	time.Sleep(1000 * time.Millisecond)
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

	log.Printf("")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
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

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("ADDING FOURTH DEVICE")
	c4 := NewTestIdentity(el_coth, thr, "four", pinstate, nil, nil, l)
	log.ErrFatal(c4.AttachToIdentity(c2.ID))
	c2.ProposeUpVote()
	c3.ProposeUpVote()
	log.ErrFatal(c4.ConfigUpdate())
	if len(c4.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c4.Config.Device))
	}

	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
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
	log.LLvlf2("c4's public key: %v", c4.Public)

	log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)

	log.LLvlf2("RECONNECTING USER2 TO THE SITE IDENTITY: %v", c2.ID)
	log.ErrFatal(u2.ReConnect(c2.ID))
	log.LLvlf2("user2's pins")
	for _, site := range u2.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}
	log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)

	for i := 0; i < 10; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.LLvlf2("RECONNECTING USER2 TO THE SITE IDENTITY: %v", c2.ID)
		log.ErrFatal(u2.ReConnect(c2.ID))
		log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)
	}
}

func Test2MultDevs(t *testing.T) {
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
	hosts_ws, _, _ := l.GenTree(5, true)
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

	c := NewTestIdentityMultDevs(el_coth, thr, []string{"one", "two"}, pinstate, cas, data, l)
	c1 := c[0]
	c2 := c[1]
	log.ErrFatal(c1.CreateIdentityMultDevs(c))

	log.ErrFatal(c1.ConfigUpdate())
	for name, _ := range c1.Config.Device {
		log.LLvlf2("device: %v", name)
	}

	time.Sleep(1000 * time.Millisecond)
	log.Print("")
	log.LLvlf2("ADDING THIRD DEVICE")
	pinstate = &common_structs.PinState{Ctype: "device"}
	c3 := NewTestIdentity(el_coth, thr, "threeEEEEE", pinstate, nil, nil, l)
	log.ErrFatal(c1.ConfigUpdate())
	for name, _ := range c1.Config.Device {
		log.LLvlf2("device: %v", name)
	}

	log.LLvlf2("Before proposing")
	c3.AttachToIdentity(c1.ID)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 3 {
		t.Fatal("Should have three owners by now")
	}

	log.LLvlf2("ATTACHING WEB SERVERS TO THE SITE IDENTITY: %v", c1.ID)
	services = l.GetServices(hosts_ws, WSService)
	for index, s := range services {
		ws := s.(*WS)
		log.ErrFatal(ws.WSAttach(el_coth, c1.ID, keypairs[index].Public, keypairs[index].Secret))
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
	log.LLvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	time.Sleep(1000 * time.Millisecond)
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

	time.Sleep(1000 * time.Millisecond)
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

	log.Printf("")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
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

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("ADDING FOURTH DEVICE")
	c4 := NewTestIdentity(el_coth, thr, "four", pinstate, nil, nil, l)
	log.ErrFatal(c4.AttachToIdentity(c2.ID))
	c2.ProposeUpVote()
	c3.ProposeUpVote()
	log.ErrFatal(c4.ConfigUpdate())
	if len(c4.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c4.Config.Device))
	}

	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
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
	log.LLvlf2("c4's public key: %v", c4.Public)

	log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)

	log.LLvlf2("RECONNECTING USER2 TO THE SITE IDENTITY: %v", c2.ID)
	log.ErrFatal(u2.ReConnect(c2.ID))
	log.LLvlf2("user2's pins")
	for _, site := range u2.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}
	log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)

	for i := 0; i < 10; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.LLvlf2("RECONNECTING USER2 TO THE SITE IDENTITY: %v", c2.ID)
		log.ErrFatal(u2.ReConnect(c2.ID))
		log.LLvlf2("User2's window regarding site: %v is: %v", c2.ID, u2.WebSites[string(c2.ID)].PinState.Window)
	}
}

func TestMultSites(t *testing.T) {
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
	log.LLvlf2("")
	log.LLvlf2("NEW SITE IDENTITY FOR SITE1")
	pinstate := &common_structs.PinState{Ctype: "device"}
	// include into the config the tls public key of one web server
	hosts_ws, wss, keypairs, data := GenerateWSPublicKeys(1, l)
	c1 := NewTestIdentity(el_coth, thr, "one", pinstate, cas, data, l)
	log.ErrFatal(c1.CreateIdentity())

	log.LLvlf2("")
	log.LLvlf2("NEW SITE IDENTITY FOR SITE2")
	pinstate = &common_structs.PinState{Ctype: "device"}
	// include into the config the tls public key of one web server
	hosts_ws2, wss2, keypairs2, data2 := GenerateWSPublicKeys(1, l)
	d1 := NewTestIdentity(el_coth, thr, "site2_one", pinstate, cas, data2, l)
	log.ErrFatal(d1.CreateIdentity())

	AttachWebServersToSite(c1.ID, hosts_ws, el_coth, keypairs, l)

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("")
	log.LLvlf2("ADDING SECOND DEVICE TO THE SITE IDENTITY: %v", c1.ID)
	pinstate = &common_structs.PinState{Ctype: "device"}
	c2 := NewTestIdentity(el_coth, thr, "two", pinstate, nil, nil, l)
	c2.AttachToIdentity(c1.ID)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	AttachWebServersToSite(d1.ID, hosts_ws2, el_coth, keypairs2, l)

	defer l.CloseAll()

	log.LLvlf2("")
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
	log.LLvlf2("")
	log.LLvlf2("NEW THRESHOLD VALUE: %v OF THE SITE IDENTITY: %v", thr, c1.ID)
	c1.UpdateIdentityThreshold(thr)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("")
	log.LLvlf2("ATTACHING USER2 TO THE SITE IDENTITY: %v", d1.ID)
	siteInfo = &common_structs.SiteInfo{
		ID:  d1.ID,
		WSs: wss2,
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

	log.Printf("")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("")
	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
	log.LLvlf2("user1's pins")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("c1's public key: %v", c1.Public)
	log.LLvlf2("c2's public key: %v", c2.Public)

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("")
	log.LLvlf2("ADDING THIRD DEVICE TO THE SITE IDENTITY: %v", c1.ID)
	c3 := NewTestIdentity(el_coth, thr, "three", pinstate, nil, nil, l)
	log.ErrFatal(c3.AttachToIdentity(c2.ID))
	c1.ProposeUpVote()
	c2.ProposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if len(c3.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c3.Config.Device))
	}

	log.LLvlf2("")
	log.LLvlf2("RECONNECTING USER1 TO THE SITE IDENTITY: %v", c1.ID)
	log.ErrFatal(u1.ReConnect(c1.ID))
	log.LLvlf2("user1's pins")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.LLvlf2("Pin: %v is %v", index, pin)
		}
	}

	log.LLvlf2("c1's public key: %v", c1.Public)
	log.LLvlf2("c2's public key: %v", c2.Public)

	log.LLvlf2("User2's window regarding site: %v is: %v", d1.ID, u2.WebSites[string(d1.ID)].PinState.Window)

	time.Sleep(1000 * time.Millisecond)
	log.LLvlf2("")
	log.LLvlf2("ADDING SECOND DEVICE TO THE SITE IDENTITY: %v", d1.ID)
	d2 := NewTestIdentity(el_coth, thr, "site2_two", pinstate, nil, nil, l)
	log.ErrFatal(d2.AttachToIdentity(d1.ID))
	d1.ProposeUpVote()
	log.ErrFatal(d2.ConfigUpdate())
	if len(d2.Config.Device) != 2 {

		t.Fatal("Should have two owners by now but got only: ", len(d2.Config.Device))
	}

	for i := 0; i < 15; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.LLvlf2("")
		log.LLvlf2("%v: RECONNECTING USER2 TO THE SITE IDENTITY: %v", i, d1.ID)
		window := u2.WebSites[string(d1.ID)].PinState.Window
		time_last_pin_acc := u2.WebSites[string(d1.ID)].PinState.TimePinAccept
		log.LLvlf2("User2's window regarding site: %v is: %v", d1.ID, window)
		log.LLvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
		log.ErrFatal(u2.ReConnect(d1.ID))
	}

}
