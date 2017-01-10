package sidentity

import (
	//"fmt"
	"github.com/dedis/onet/log"
	"github.com/dedis/crypto/abstract"
	"time"
	//"github.com/dedis/onet/network"
	"github.com/dedis/onet"
	"github.com/dedis/cothority/dns_id/ca"
	"github.com/dedis/cothority/dns_id/common_structs"
	//"github.com/dedis/crypto/config"
	//"github.com/stretchr/testify/assert"
	//"io/ioutil"
	//"os"
	"testing"
)

func NewTestIdentity(cothority *onet.Roster, majority int, owner string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, local *onet.LocalTest) *Identity {
	id := NewIdentity(cothority, majority, owner, pinstate, cas)
	id.CothorityClient = local.NewClient(ServiceName)
	return id
}

func NewTestIdentityMultDevs(cothority *onet.Roster, majority int, owners []string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, local *onet.LocalTest) []*Identity {
	ids, _ := NewIdentityMultDevs(cothority, majority, owners, pinstate, cas)
	for _, id := range ids {
		id.CothorityClient = local.NewClient(ServiceName)
	}
	return ids
}

func TestGetCert(t *testing.T) {
	l := onet.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, IdentityService)
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	hosts, _, _ = l.GenTree(2, true)
	services = l.GetServices(hosts, ca.CAService)
	defer l.CloseAll()

	cas := make([]common_structs.CAInfo, 0)
	for index, h := range hosts {
		log.LLvlf2("CA: %v has ServerIdentity: %v", index, h.ServerIdentity)
		cas = append(cas, common_structs.CAInfo{Public: services[index].(*ca.CA).Public, ServerID: h.ServerIdentity})
	}

	thr := 1
	log.Print("NEW SITE IDENTITY")
	pinstate := &common_structs.PinState{Ctype: "device"}
	c1 := NewTestIdentity(el, thr, "one", pinstate, cas, l)
	log.ErrFatal(c1.CreateIdentity())

	log.Print("")
	log.Print("Adding second device")
	pinstate = &common_structs.PinState{Ctype: "device"}
	c2 := NewTestIdentity(el, thr, "two", pinstate, nil, l)
	c2.AttachToIdentity(c1.ID)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	thr = 2
	log.Print("")
	log.Lvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	log.Print("")
	log.Print("ADDING THIRD DEVICE")
	c3 := NewTestIdentity(el, thr, "three", pinstate, nil, l)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c2.proposeUpVote()
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
		c3.proposeUpVote()
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

	/*for index, cert := range c3.Certs {
		log.Lvlf2("cert: %v, siteID: %v, hash: %v, sig: %v, public: %v", index, cert.ID, cert.Hash, cert.Signature, cert.Public)
	}*/
	if len(c3.Certs) != len(cas) {
		t.Fatalf("Should have %v certs by now", len(cas))
	}
}

func TestGenesisWithMultipleDevices(t *testing.T) {
	l := onet.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, IdentityService)
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
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
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 3 {
		t.Fatal("Should have three owners by now")
	}

	thr = 2
	log.Lvlf2("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Lvlf2("New threshold: %v", c1.Config.Threshold)

	log.Print("ADDING FOURTH DEVICE")
	c4 := NewTestIdentity(el, thr, "four", pinstate, nil, l)
	log.ErrFatal(c4.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c2.proposeUpVote()
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
		c3.proposeUpVote()
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

func (i *Identity) proposeUpVote() {
	log.ErrFatal(i.ProposeUpdate())
	log.ErrFatal(i.ProposeVote(true))
}
