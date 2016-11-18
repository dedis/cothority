package sidentity

import (
	//"fmt"
	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/abstract"
	//"time"
	//"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	//"github.com/dedis/crypto/config"
	//"github.com/stretchr/testify/assert"
	//"io/ioutil"
	//"os"
	"testing"
)

func NewTestIdentity(cothority *sda.Roster, majority int, owner string, pinstate *PinState, cas []common_structs.CAInfo, local *sda.LocalTest) *Identity {
	id := NewIdentity(cothority, majority, owner, pinstate, cas)
	id.CothorityClient = local.NewClient(ServiceName)
	return id
}

/*
func TestIdentity_ConfigNewCheck(t *testing.T) {
	l := sda.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one")
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two"] = &Device{Point: kp2.Public}
	conf2.Data["two"] = "public2"
	log.ErrFatal(c1.ProposeSend(conf2))

	log.ErrFatal(c1.ProposeUpdate())
	al := c1.Proposed
	assert.NotNil(t, al)

	o2, ok := al.Device["two"]
	assert.True(t, ok)
	assert.True(t, kp2.Public.Equal(o2.Point))
	pub2, ok := al.Data["two"]
	assert.True(t, ok)
	assert.Equal(t, "public2", pub2)
	l.CloseAll()
}

func TestIdentity_AttachToIdentity(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	for _, s := range services {
		s.(*Service).clearIdentities()
	}
	defer l.CloseAll()

	c1 := NewTestIdentity(el, 50, "one", l)
	log.ErrFatal(c1.CreateIdentity())

	c2 := NewTestIdentity(el, 50, "two", l)
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	for _, s := range services {
		is := s.(*Service)
		is.identitiesMutex.Lock()
		if len(is.Identities) != 1 {
			t.Fatal("The configuration hasn't been proposed in all services")
		}
		is.identitiesMutex.Unlock()
	}
}

func TestIdentity_ConfigUpdate(t *testing.T) {
	l := sda.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c1 := NewTestIdentity(el, 50, "one", l)
	log.ErrFatal(c1.CreateIdentity())

	c2 := NewTestIdentity(el, 50, "two", l)
	c2.ID = c1.ID
	log.ErrFatal(c2.ConfigUpdateNew())

	assert.NotNil(t, c2.Config)
	o1 := c2.Config.Device[c1.DeviceName]
	if !o1.Point.Equal(c1.Public) {
		t.Fatal("Owner is not c1")
	}
}

func TestIdentity_CreateIdentity(t *testing.T) {
	l := sda.NewTCPTest()
	_, el, _ := l.GenTree(3, true)
	defer l.CloseAll()

	c := NewTestIdentity(el, 50, "one", l)
	log.ErrFatal(c.CreateIdentity())

	// Check we're in the configuration
	assert.NotNil(t, c.Config)
}

func TestIdentity_ConfigNewPropose(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(3, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewTestIdentity(el, 50, "one", l)
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two"] = &Device{Point: kp2.Public}
	log.ErrFatal(c1.ProposeSend(conf2))

	for _, s := range services {
		is := s.(*Service)
		id1 := is.getIdentityStorage(c1.ID)
		id1.Lock()
		if id1 == nil {
			t.Fatal("Didn't find")
		}
		assert.NotNil(t, id1.Proposed)
		if len(id1.Proposed.Device) != 2 {
			t.Fatal("The proposed config should have 2 entries now")
		}
		id1.Unlock()
	}
}

func TestIdentity_ProposeVote(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	c1 := NewTestIdentity(el, 1, "one1", l)
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two2"] = &Device{Point: kp2.Public}
	conf2.Data["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(conf2))
	log.ErrFatal(c1.ProposeUpdate())
	fmt.Println("before voting")
	log.ErrFatal(c1.ProposeVote(true))

	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners now")
	}
}

func TestIdentity_ProposeVoteExtended(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	thr := 1
	c1 := NewTestIdentity(el, thr, "one", l)
	log.ErrFatal(c1.CreateIdentity())

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding second identity")
	c2 := NewTestIdentity(el, thr, "two", l)
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	thr = 2
	fmt.Println()
	fmt.Println()
	fmt.Println("Proposing a new threshold value: ", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding third identity")
	c3 := NewTestIdentity(el, thr, "three", l)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c2.proposeUpVote()
	fmt.Println("HELLO")
	log.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c1.Config.Device))
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding fourth identity")
	c4 := NewTestIdentity(el, thr, "four", l)
	log.ErrFatal(c4.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	//c3.ConfigUpdate()
	c3.proposeUpVote()
	log.ErrFatal(c4.ConfigUpdate())
	if len(c4.Config.Device) != 4 {
		t.Fatal("Should have four owners by now")
	}

	thr = 3
	fmt.Println()
	fmt.Println()
	fmt.Println("Proposing a new threshold value: ", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c4.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding fifth identity")
	c5 := NewTestIdentity(el, thr, "five", l)
	log.ErrFatal(c5.AttachToIdentity(c3.ID))
	c1.proposeUpVote()
	c2.proposeUpVote()
	c4.proposeUpVote()
	log.ErrFatal(c3.ConfigUpdate())
	if len(c3.Config.Device) != 5 {
		t.Fatal("Should have five owners by now")
	}
}

func TestIdentity_ProposeVoteExtended_Del(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	thr := 1
	pinstate := &PinState{Ctype: "device"}

	c1 := NewTestIdentity(el, thr, "one", pinstate, l)
	log.ErrFatal(c1.CreateIdentity())

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding second identity")
	c2 := NewTestIdentity(el, thr, "two", pinstate, l)
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	thr = 2
	fmt.Println()
	fmt.Println()
	fmt.Println("Proposing a new threshold value: ", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}

	fmt.Println()
	fmt.Println()
	fmt.Println("Adding third identity")
	c3 := NewTestIdentity(el, thr, "three", pinstate, l)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	c1.proposeUpVote()
	c2.proposeUpVote()
	fmt.Println("HELLO")
	log.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Device) != 3 {

		t.Fatal("Should have three owners by now but got only: ", len(c1.Config.Device))
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("Revoking first identity")
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

}

func TestIdentity_SaveToStream(t *testing.T) {
	l := sda.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()
	id := NewIdentity(el, 50, "one1")
	tmpfile, err := ioutil.TempFile("", "example")
	log.ErrFatal(err)
	defer os.Remove(tmpfile.Name())
	log.ErrFatal(id.SaveToStream(tmpfile))
	tmpfile.Close()
	tmpfile, err = os.Open(tmpfile.Name())
	log.ErrFatal(err)
	id2, err := NewIdentityFromStream(tmpfile)
	assert.NotNil(t, id2)
	log.ErrFatal(err)
	tmpfile.Close()

	if id.Config.Threshold != id2.Config.Threshold {
		t.Fatal("Loaded threshold is not the same")
	}
	p, p2 := id.Config.Device["one1"].Point, id2.Config.Device["one1"].Point
	if !p.Equal(p2) {
		t.Fatal("Public keys are not the same")
	}
	if id.Config.Data["one1"] != id2.Config.Data["one1"] {
		t.Fatal("Owners are not the same", id.Config.Data, id2.Config.Data)
	}
}

func TestCrashAfterRevocation(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	c1 := NewIdentity(el, 2, "one")
	c2 := NewIdentity(el, 2, "two")
	c3 := NewIdentity(el, 2, "three")
	log.ErrFatal(c1.CreateIdentity())
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	proposeUpVote(c1)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	proposeUpVote(c1)
	proposeUpVote(c2)
	//log.ErrFatal(c1.ConfigUpdate())
	log.Lvl2(c1.Config)

	conf := c1.GetProposed()
	delete(conf.Device, "three")
	log.Lvl2(conf)
	log.ErrFatal(c1.ProposeSend(conf))
	proposeUpVote(c1)
	proposeUpVote(c2)
	log.ErrFatal(c1.ConfigUpdate())
	log.Lvl2(c1.Config)

	log.Lvl1("C3 trying to send anyway")
	conf = c3.GetProposed()
	c3.ProposeSend(conf)
	if c3.ProposeVote(true) == nil {
		t.Fatal("Should not be able to vote")
	}
	log.ErrFatal(c1.ProposeUpdate())
}
*/
func TestGetCert(t *testing.T) {
	l := sda.NewTCPTest()
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
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
	pinstate := &PinState{Ctype: "device"}
	c1 := NewTestIdentity(el, thr, "one", pinstate, cas, l)
	log.ErrFatal(c1.CreateIdentity())

	log.Print("")
	log.Print("Adding second device")
	pinstate = &PinState{Ctype: "device"}
	c2 := NewTestIdentity(el, thr, "two", pinstate, nil, l)
	c2.AttachToIdentity(c1.ID)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners by now")
	}

	thr = 2
	log.Print("")
	log.Printf("NEW THRESHOLD VALUE: %v", thr)
	c1.UpdateIdentityThreshold(thr)
	c1.proposeUpVote()
	c1.ConfigUpdate()
	if c1.Config.Threshold != thr {
		t.Fatal("Wrong threshold")

	}
	log.Printf("New threshold: %v", c1.Config.Threshold)

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
		log.Printf("cert: %v, siteID: %v, hash: %v, sig: %v, public: %v", index, cert.ID, cert.Hash, cert.Signature, cert.Public)
	}*/
	if len(c3.Certs) != len(cas) {
		t.Fatalf("Should have %v certs by now", len(cas))
	}
}

func (i *Identity) proposeUpVote() {
	log.ErrFatal(i.ProposeUpdate())
	log.ErrFatal(i.ProposeVote(true))
}
