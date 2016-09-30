package identity

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func NewTestIdentity(cothority *sda.Roster, majority int, owner string, local *sda.LocalTest) *Identity {
	id := NewIdentity(cothority, majority, owner)
	id.Client = local.NewClient(ServiceName)
	return id
}

func TestIdentity_ConfigNewCheck(t *testing.T) {
	l := sda.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one")
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two"] = &Device{kp2.Public}
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
	log.ErrFatal(c2.ConfigUpdate())

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
	conf2.Device["two"] = &Device{kp2.Public}
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

	c1 := NewTestIdentity(el, 50, "one1", l)
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two2"] = &Device{kp2.Public}
	conf2.Data["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(conf2))
	log.ErrFatal(c1.ProposeUpdate())
	log.ErrFatal(c1.ProposeVote(true))

	if len(c1.Config.Device) != 2 {
		t.Fatal("Should have two owners now")
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
	log.ErrFatal(c1.ConfigUpdate())
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

func proposeUpVote(i *Identity) {
	log.ErrFatal(i.ProposeUpdate())
	log.ErrFatal(i.ProposeVote(true))
}
