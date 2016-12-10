package webserver

import (
	"fmt"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/sidentity"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/random"
	"math/rand"
	"sync"
	"testing"
	"time"
)

//func NewTestIdentity(cothority *sda.Roster, majority int, owner string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, local *sda.LocalTest) *sidentity.Identity {
func NewTestIdentity(cothority *sda.Roster, fqdn string, majority int, owner string, ctype string, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, duration int64, local *sda.LocalTest) *sidentity.Identity {
	id := sidentity.NewIdentity(cothority, fqdn, majority, owner, ctype, cas, data, duration)
	id.CothorityClient = local.NewClient(sidentity.ServiceName)
	return id
}

//func NewTestIdentityMultDevs(cothority *sda.Roster, majority int, owners []string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, local *sda.LocalTest) []*sidentity.Identity {
func NewTestIdentityMultDevs(cothority *sda.Roster, fqdn string, majority int, owners []string, ctype string, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, duration int64, local *sda.LocalTest) []*sidentity.Identity {
	ids, _ := sidentity.NewIdentityMultDevs(cothority, fqdn, majority, owners, ctype, cas, data, duration)
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

// (Using Asymmetric crypto for encryption/decryption of the private tls keys of each of the web servers)
// Upon return of this function, the 'data' field contains only the ServerIdentities of the web servers
func GetWSPublicsPlusServerIDs(num_ws int, el_coth *sda.Roster, l *sda.LocalTest) ([]*sda.Conode, []*WS, []common_structs.WSInfo, map[string]*common_structs.WSconfig) {
	hosts_ws, _, _ := l.GenTree(num_ws, true)
	services := l.GetServices(hosts_ws, WSService)
	wss := make([]common_structs.WSInfo, 0)
	data := make(map[string]*common_structs.WSconfig)
	webservers := make([]*WS, 0)
	for index, ws := range hosts_ws {
		wss = append(wss, common_structs.WSInfo{ServerID: ws.ServerIdentity})

		// push each web server's public key to the cothority (which is going to be used for encryption of
		// its tls private key)
		ws := services[index].(*WS)
		webservers = append(webservers, ws)
		log.ErrFatal(ws.WSPushPublicKey(el_coth))

		key := fmt.Sprintf("tls:%v", ws.ServerIdentity())
		data[key] = &common_structs.WSconfig{
			ServerID: ws.ServerIdentity(),
		}
	}
	return hosts_ws, webservers, wss, data
}

func TestSkipchainSwitch(t *testing.T) {
	log.SetDebugVisible(2)

	l := sda.NewTCPTest()
	hosts_coth, el_coth, _ := l.GenTree(3, true)
	services := l.GetServices(hosts_coth, sidentity.IdentityService)

	proxies := make([]*sidentity.Service, 0)
	for _, s := range services {
		log.Lvl3(s.(*sidentity.Service).Identities)
		proxy := s.(*sidentity.Service)
		proxy.ClearIdentities()
		proxies = append(proxies, proxy)
	}

	//randomID := rand.Int() % len(services)
	randomID := 0
	proxies[randomID].TheRoster = el_coth
	go proxies[randomID].RunLoop(el_coth)

	hosts_ca, _, _ := l.GenTree(2, true)
	services = l.GetServices(hosts_ca, ca.CAService)
	cas := make([]common_structs.CAInfo, 0)
	for index, h := range hosts_ca {
		cas = append(cas, common_structs.CAInfo{Public: services[index].(*ca.CA).Public, ServerID: h.ServerIdentity})
	}

	// site1
	log.Lvlf2("")
	log.Lvlf2("NEW SITE IDENTITY FOR SITE1")
	site1 := "site1"
	thr := 1
	num_ws1 := 2
	duration := int64(15552000000) // == 6 months * 30 days/month * 24 hours/day * 3600 sec/hour * 1000 ms/sec (REALISTIC)
	_, webservers1, wss1, data1 := GetWSPublicsPlusServerIDs(num_ws1, el_coth, l)
	c1 := NewTestIdentity(el_coth, site1, thr, "one", "device", cas[0:1], data1, duration, l)
	log.ErrFatal(c1.CreateIdentity())
	for _, ws := range webservers1 {
		ws.WSAttach(site1, c1.ID, el_coth)
	}

	//site2
	log.Lvlf2("")
	log.Lvlf2("NEW SITE IDENTITY FOR SITE2")
	site2 := "site2"
	thr = 1
	num_ws2 := 1
	_, webservers2, wss2, data2 := GetWSPublicsPlusServerIDs(num_ws2, el_coth, l)
	d1 := NewTestIdentity(el_coth, site2, thr, "site2_one", "device", cas[0:1], data2, duration, l)
	log.ErrFatal(d1.CreateIdentity())
	for _, ws := range webservers2 {
		ws.WSAttach(site2, d1.ID, el_coth)
	}

	//site3
	log.Lvlf2("")
	log.Lvlf2("NEW SITE IDENTITY FOR SITE3")
	site3 := "site3"
	thr = 2
	num_ws3 := 1
	_, webservers3, wss3, data3 := GetWSPublicsPlusServerIDs(num_ws3, el_coth, l)
	e := NewTestIdentityMultDevs(el_coth, site3, thr, []string{"site3_one", "site3_two"}, "device", cas[1:2], data3, duration, l)
	e1 := e[0]
	e2 := e[1]
	log.ErrFatal(e1.CreateIdentityMultDevs(e))
	for _, ws := range webservers3 {
		ws.WSAttach(site3, e1.ID, el_coth)
	}

	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ADDING SECOND DEVICE TO THE SITE IDENTITY: %v", c1.ID)
	c2 := NewTestIdentity(el_coth, "", 0, "two", "device", nil, nil, 0, l)
	c2.AttachToIdentity(c1.ID)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	defer l.CloseAll()

	log.Lvlf2("")
	log.Lvlf2("ATTACHING USER1 TO SITE: %v (SITE IDENTITY: %v)", site1, c1.ID)
	siteInfo := &common_structs.SiteInfo{
		FQDN: site1,
		WSs:  wss1,
	}
	sitestoattach := make([]*common_structs.SiteInfo, 0)
	sitestoattach = append(sitestoattach, siteInfo)
	u1 := NewTestUser("user1", sitestoattach, l)
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}
	log.Lvlf2("%v", c1.Public)
	log.Lvlf2("%v", c2.Public)

	thr = 2
	log.Lvlf2("")
	log.Lvlf2("NEW THRESHOLD VALUE: %v OF THE SITE IDENTITY: %v", thr, c1.ID)
	c1.ProposeConfig(nil, nil, thr, 0, nil)
	c1.ProposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	// site2
	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ATTACHING USER2 TO SITE: %v (SITE IDENTITY: %v)", site2, d1.ID)
	siteInfo = &common_structs.SiteInfo{
		FQDN: site2,
		WSs:  wss2,
	}
	sitestoattach = make([]*common_structs.SiteInfo, 0)
	sitestoattach = append(sitestoattach, siteInfo)
	u2 := NewTestUser("user2", sitestoattach, l)
	log.Lvlf2("user2's pins")
	for _, site := range u2.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}
	log.Lvlf2("trust_window: %v", u2.WebSites[site2].PinState.Window)

	log.Printf("")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}

	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ATTACHING USER2 TO SITE: %v (SITE IDENTITY: %v)", site3, e1.ID)
	siteInfo = &common_structs.SiteInfo{
		FQDN: site3,
		WSs:  wss3,
	}
	sitestoattach = make([]*common_structs.SiteInfo, 0)
	sitestoattach = append(sitestoattach, siteInfo)
	u2.NewAttachments(sitestoattach)
	log.Lvlf2("user2's pins:")
	for _, site := range u2.WebSites {
		log.Lvlf2("Regarding site with FQDN: %v", site.FQDN)
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}
	log.Lvlf2("trust_window: %v", u2.WebSites[site3].PinState.Window)

	log.Printf("")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}

	log.Lvlf2("")
	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("RECONNECTING USER1 TO THE SITE: %v", site1)
	log.ErrFatal(u1.ReConnect(site1))
	log.Lvlf2("user1's pins")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}

	log.Lvlf3("c1's public key: %v", c1.Public)
	log.Lvlf3("c2's public key: %v", c2.Public)

	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ADDING THIRD DEVICE TO THE SITE IDENTITY: %v", c1.ID)
	c3 := NewTestIdentity(el_coth, "", 0, "three", "device", nil, nil, 0, l)
	log.ErrFatal(c3.AttachToIdentity(c2.ID))
	c1.ProposeUpVote()
	c2.ProposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if len(c3.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c3.Config.Device))
	}

	for i := 0; i < 3; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.Lvlf2("")
		log.Lvlf2("%v: RECONNECTING USER1 TO THE SITE: %v", i, site1)
		window := u1.WebSites[site1].PinState.Window
		time_last_pin_acc := u1.WebSites[site1].PinState.TimePinAccept
		log.Lvlf2("User1's window regarding site: %v is: %v", site1, window)
		log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
		log.ErrFatal(u1.ReConnect(site1))
	}

	log.Lvlf2("")
	log.Lvlf2("RECONNECTING USER1 TO THE SITE: %v", site1)
	log.ErrFatal(u1.ReConnect(site1))
	log.Lvlf2("user1's pins")
	for _, site := range u1.WebSites {
		pins := site.PinState.Pins
		for index, pin := range pins {
			log.Lvlf3("Pin: %v is %v", index, pin)
		}
	}

	log.Lvlf3("c1's public key: %v", c1.Public)
	log.Lvlf3("c2's public key: %v", c2.Public)

	// site2
	log.Lvlf2("%v", d1.ID)
	log.Lvlf2("User2's window regarding site: %v is: %v", site2, u2.WebSites[site2].PinState.Window)

	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ADDING SECOND DEVICE TO THE SITE IDENTITY: %v", d1.ID)
	d2 := NewTestIdentity(el_coth, "", 0, "site2_two", "device", nil, nil, 0, l)
	log.ErrFatal(d2.AttachToIdentity(d1.ID))
	d1.ProposeUpVote()
	log.ErrFatal(d2.ConfigUpdate())
	if len(d2.Config.Device) != 2 {

		t.Fatal("Should have two owners by now but got only: ", len(d2.Config.Device))
	}

	for i := 0; i < 15; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.Lvlf2("")
		log.Lvlf2("%v: RECONNECTING USER2 TO THE SITE: %v", i, site2)
		window := u2.WebSites[site2].PinState.Window
		time_last_pin_acc := u2.WebSites[site2].PinState.TimePinAccept
		log.Lvlf2("User2's window regarding site: %v is: %v", site2, window)
		log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
		log.ErrFatal(u2.ReConnect(site2))
	}

	log.Lvlf2("")
	log.Lvlf2("RECONNECTING USER1 TO THE SITE: %v", site1)
	log.ErrFatal(u1.ReConnect(site1))
	log.Lvlf2("user1's pins")

	serverID := wss1[0].ServerID
	key := fmt.Sprintf("tls:%v", serverID)
	log.Lvlf2("Web server with serverID: %v has tls public key: %v", serverID, u1.WebSites[site1].Config.Data[key].TLSPublic)
	log.Lvlf2("DEVICE: %v PROPOSES A MODIFICATION OF THE TLS KEYPAIR OF THE SITE'S: %v WEB SERVER WITH SERVERID %v", c2.DeviceName, site1, serverID)
	serverIDs := make([]*network.ServerIdentity, 0)
	serverIDs = append(serverIDs, serverID)
	thr = 1
	c2.ProposeConfig(nil, nil, thr, 0, serverIDs)
	c2.ProposeUpVote()
	c3.ProposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if c3.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}

	log.Lvlf2("")
	log.Lvlf2("RECONNECTING USER1 TO THE SITE: %v", site1)
	log.ErrFatal(u1.ReConnect(site1))
	log.Lvlf2("user1's pins")

	key = fmt.Sprintf("tls:%v", serverID)
	log.Lvlf2("Web server with serverID: %v has tls public key: %v", serverID, u1.WebSites[site1].Config.Data[key].TLSPublic)

	log.Lvlf2("")
	log.Lvlf2("REVOKING FIRST IDENTITY")
	c3.ConfigUpdate()
	revokelist := make(map[string]abstract.Point)
	n := "one"
	if _, exists := c3.Config.Device[n]; exists {
		revokelist[n] = c3.Config.Device[n].Point
		c3.ProposeConfig(nil, revokelist, thr, 0, nil)
		c3.ProposeUpVote()
		log.ErrFatal(c2.ConfigUpdate())
		if len(c2.Config.Device) != 2 {
			t.Fatal("Should have two owners by now")
		}
		c3.ConfigUpdate()
		if _, exists := c3.Config.Device[n]; exists {
			t.Fatal("Device one should have been revoked by now")
		}
	}

	time.Sleep(1000 * time.Millisecond)
	log.Lvlf2("")
	log.Lvlf2("ADDING THIRD DEVICE TO THE SITE: %v", site3)
	e3 := NewTestIdentity(el_coth, "", 0, "site3_three", "device", nil, nil, 0, l)
	log.ErrFatal(e3.AttachToIdentity(e1.ID))
	e1.ProposeUpVote()
	e2.ProposeUpVote()
	log.ErrFatal(e1.ConfigUpdate())
	if len(e1.Config.Device) != 3 {
		for name, _ := range e1.Config.Device {
			log.Lvlf2("%v", name)
		}
		t.Fatal("Should have three owners by now but got only: ", len(e1.Config.Device))
	}

	for i := 0; i < 3; i++ {
		time.Sleep(1000 * time.Millisecond)
		log.Lvlf2("")
		log.Lvlf2("%v: RECONNECTING USER2 TO THE SITE IDENTITY: %v", i, site3)
		window := u2.WebSites[site3].PinState.Window
		time_last_pin_acc := u2.WebSites[site3].PinState.TimePinAccept
		log.Lvlf2("User1's window regarding site: %v is: %v", site3, window)
		log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
		log.ErrFatal(u2.ReConnect(site3))
	}

	//compromise of site3's web servers
	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	log.Lvlf2("ATTACKER GAINS CONTROL OVER THE WSs OF SITE: %v", site3)
	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	thr = 1
	e_fake := NewTestIdentityMultDevs(el_coth, site3, thr, []string{"site3_fake_one"}, "device", cas[1:2], data3, duration, l)
	e1_fake := e_fake[0]
	log.ErrFatal(e1_fake.CreateIdentityMultDevs(e_fake))
	for _, ws := range webservers3 {
		ws.WSAttach(site3, e1_fake.ID, el_coth)
	}

	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(1000 * time.Millisecond)
			log.Lvlf2("")
			log.Lvlf2("%v: RECONNECTING USER2 TO THE SITE IDENTITY: %v", i, site3)
			window := u2.WebSites[site3].PinState.Window
			time_last_pin_acc := u2.WebSites[site3].PinState.TimePinAccept
			log.Lvlf2("User1's window regarding site: %v is: %v", site3, window)
			log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
			log.ErrFatal(u2.ReConnect(site3))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(1000 * time.Millisecond)
			log.Lvlf2("")
			log.Lvlf2("%v: RECONNECTING USER1 TO THE SITE: %v", i, site1)
			window := u1.WebSites[site1].PinState.Window
			time_last_pin_acc := u1.WebSites[site1].PinState.TimePinAccept
			log.Lvlf2("User1's window regarding site: %v is: %v", site1, window)
			log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
			log.ErrFatal(u1.ReConnect(site1))
		}
	}()

	serverID = wss1[0].ServerID
	key = fmt.Sprintf("tls:%v", serverID)
	log.Lvlf2("Web server with serverID: %v has tls public key: %v", serverID, u1.WebSites[site1].Config.Data[key].TLSPublic)
	log.Lvlf2("DEVICE: %v PROPOSES A MODIFICATION OF THE TLS KEYPAIR OF THE SITE'S: %v WEB SERVER WITH SERVERID %v", c2.DeviceName, site1, serverID)
	serverIDs = make([]*network.ServerIdentity, 0)
	serverIDs = append(serverIDs, serverID)
	thr = 1
	c2.ProposeConfig(nil, nil, thr, 0, serverIDs)
	c2.ProposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if c3.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}

	log.Lvlf2("ADDING FOURTH DEVICE TO THE SITE IDENTITY: %v", c2.ID)
	c4 := NewTestIdentity(el_coth, "", 0, "four", "device", nil, nil, 0, l)
	log.ErrFatal(c4.AttachToIdentity(c2.ID))
	c2.ProposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if len(c3.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c3.Config.Device))
	}

	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			time.Sleep(1000 * time.Millisecond)
			log.Lvlf2("")
			log.Lvlf2("%v: RECONNECTING USER2 TO THE SITE: %v", i, site2)
			window := u2.WebSites[site2].PinState.Window
			time_last_pin_acc := u2.WebSites[site2].PinState.TimePinAccept
			log.Lvlf2("User2's window regarding site: %v is: %v", site2, window)
			log.Lvlf2("Time elapsed until latest pin acceptance: %v", time.Now().Unix()*1000-time_last_pin_acc)
			log.ErrFatal(u2.ReConnect(site2))
		}
	}()

	wg.Wait()

	u := make(map[string]*User)
	sites := []string{site1, site2, site3}
	wsss := [][]common_structs.WSInfo{wss1, wss2, wss3}
	webservers := [][]*WS{webservers1, webservers2, webservers3}
	devices1 := []*sidentity.Identity{c2, c3, c4}
	devices2 := []*sidentity.Identity{d1, d2}
	devices3_fake := []*sidentity.Identity{e1_fake}
	devices := [][]*sidentity.Identity{devices1, devices2, devices3_fake}

	usernames := make([]string, 0)
	for index, site := range sites {
		for userid := 0; userid < 10; userid++ {
			log.Lvlf2("ATTACHING user%v TO SITE: %v", userid, site)
			siteInfo = &common_structs.SiteInfo{
				FQDN: site,
				WSs:  wsss[index],
			}
			sitestoattach = make([]*common_structs.SiteInfo, 0)
			sitestoattach = append(sitestoattach, siteInfo)
			username := fmt.Sprintf("u%v", userid)
			usernames = append(usernames, username)
			if index == 0 {
				u[username] = NewTestUser(username, sitestoattach, l)
			} else {
				u[username].NewAttachments(sitestoattach)
			}
		}
	}
	num_updates := 2
	wg.Add(len(u) + num_updates)

	go func() {
		defer wg.Done()
		site_index := 0
		sitename := sites[site_index]
		wss := wsss[site_index]
		server_index := rand.Int() % len(wss)
		//webserver := webservers[site_index][server_index]
		serverID := wss[server_index].ServerID
		device_index := rand.Int() % len(devices[site_index])
		device := devices[site_index][device_index]
		log.Lvlf2("DEVICE: %v PROPOSES A MODIFICATION OF THE TLS KEYPAIR OF THE SITE'S: %v WEB SERVER WITH SERVERID %v", device.DeviceName, sitename, serverID)
		serverIDs := make([]*network.ServerIdentity, 0)
		serverIDs = append(serverIDs, serverID)
		device.ProposeConfig(nil, nil, 0, 0, serverIDs)
		device.ProposeUpVote()
		log.Lvlf2("______________%v: Web server's %v TLS keypair has changed______________", sitename, serverID)
		log.Lvlf2("___________________________________")
		log.Lvl2("--------------latest skipblock has hash: %v ---------------------", device.LatestID)

	}()

	go func() {
		defer wg.Done()
		site_index := 1
		sitename := sites[site_index]
		wss := wsss[site_index]
		server_index := rand.Int() % len(wss)
		//webserver := webservers[site_index][server_index]
		serverID := wss[server_index].ServerID
		device_index := rand.Int() % len(devices[site_index])
		device := devices[site_index][device_index]
		log.Lvlf2("DEVICE: %v PROPOSES A MODIFICATION OF THE TLS KEYPAIR OF THE SITE'S: %v WEB SERVER WITH SERVERID %v", device.DeviceName, sitename, serverID)
		serverIDs := make([]*network.ServerIdentity, 0)
		serverIDs = append(serverIDs, serverID)
		device.ProposeConfig(nil, nil, 0, 0, serverIDs)
		device.ProposeUpVote()
		log.Lvlf2("______________%v: Web server's %v TLS keypair has changed______________", sitename, serverID)
		log.Lvlf2("___________________________________")
		log.Lvl2("--------------latest skipblock has hash: %v ---------------------", device.LatestID)
	}()

	for index := 0; index < len(u); index++ {
		go func() {
			defer wg.Done()
			username := usernames[rand.Int()%len(usernames)]
			user := u[username]
			for i := 0; i < 10; i++ {
				site := sites[rand.Int()%len(sites)]
				log.Lvlf2("%v: RECONNECTING %v TO THE SITE: %v", i, username, site)
				log.ErrFatal(user.ReConnect(site))
			}
		}()
	}
	wg.Wait()

	site_index := 2
	sitename := sites[site_index]
	wss := wsss[site_index]
	server_index := rand.Int() % len(wss)
	serverID = wss[server_index].ServerID
	webserver := webservers[site_index][server_index]
	prev := webserver.Public
	device_index := rand.Int() % len(devices[site_index])
	device := devices[site_index][device_index]
	log.Lvlf2("DEVICE: %v PROPOSES A MODIFICATION OF THE TLS KEYPAIR OF THE SITE'S: %v WEB SERVER WITH SERVERID %v (public: %v)", device.DeviceName, sitename, serverID, prev)
	serverIDs = make([]*network.ServerIdentity, 0)
	serverIDs = append(serverIDs, serverID)
	device.ProposeConfig(nil, nil, 0, 0, serverIDs)
	device.ProposeUpVote()
	log.Lvlf2("______________%v: Web server's %v TLS keypair has changed______________", sitename, serverID)
	log.Lvlf2("___________________________________")
	log.Lvl2("--------------latest skipblock has hash: %v ---------------------", device.LatestID)

	username := usernames[rand.Int()%len(usernames)]
	user := u[username]

	log.Lvlf2("RECONNECTING %v TO THE SITE: %v", username, sitename)
	log.ErrFatal(user.ReConnect(sitename))
	log.Lvlf2("RECONNECTING %v TO THE SITE: %v", username, sitename)
	log.ErrFatal(user.ReConnect(sitename))

	/*
		for username, user := range u {
			log.LLvl2("user: %v", username)
			for i := 0; i < 100; i++ {
				log.Lvlf2("")
				site := sites[rand.Int()%len(sites)]
				log.Lvlf2("%v: RECONNECTING %v TO THE SITE: %v", i, username, site)
				log.ErrFatal(user.ReConnect(site))
			}
		}
	*/
	log.Lvlf2("THE END")
}

func TestDecryption(t *testing.T) {

	suite := ed25519.NewAES128SHA256Ed25519(false)
	// Create a public/private keypair
	a := suite.Scalar().Pick(random.Stream) // Alice's private key
	A := suite.Point().Mul(nil, a)          // Alice's public key

	// ElGamal-encrypt a message using the public key.
	//m := []byte("The quick brown fox")
	m := []byte("The quick brown foxThe quick brown fox123456789123")
	log.Lvlf2("%va", string(m[0:29]))
	log.Lvlf2("%va", string(m[29:len(m)]))
	K1, C1, _ := common_structs.ElGamalEncrypt(suite, A, m[0:29])
	K2, C2, _ := common_structs.ElGamalEncrypt(suite, A, m[29:len(m)])

	// Decrypt it using the corresponding private key.
	mm1, err1 := common_structs.ElGamalDecrypt(suite, a, K1, C1)
	mm2, err2 := common_structs.ElGamalDecrypt(suite, a, K2, C2)

	// Make sure it worked!
	if err1 != nil || err2 != nil {
		panic("decryption failed: " + err1.Error() + err2.Error())
	}
	if string(mm1) != string(m[0:29]) || string(mm2) != string(m[29:len(m)]) {
		panic("decryption produced wrong output: " + string(m[0:29]) + string(m[29:len(m)]))
	}
	println("Decryption succeeded: " + string(m[0:29]) + string(m[29:len(m)]))

}
