package identity

import (
	"testing"

	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestIdentity_CreateIdentity(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c.CreateIdentity())

	// Check we're in the configuration
	assert.NotNil(t, c.Config)
	assert.NotNil(t, c.data)
	assert.NotNil(t, c.root)
	dbg.ErrFatal(c.data.VerifySignatures())
}

func TestIdentity_ConfigNewPropose(t *testing.T) {
	l := sda.NewLocalTest()
	hosts, el, _ := l.GenTree(5, true, true, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())
	dbg.Print(c1.ID)
	time.Sleep(time.Second)

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two"] = &Owner{kp2.Public}
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))

	for _, s := range services {
		is := s.(*Service)
		id1, ok := is.Identities[string(c1.ID)]
		if !ok {
			t.Fatal("Didn't find")
		}
		assert.NotNil(t, id1.Proposed)
		if len(id1.Proposed.Owners) != 2 {
			t.Fatal("The proposed config should have 2 entries now")
		}
	}
}

func TestIdentity_ConfigNewCheck(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	//services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())
	dbg.Print(c1.ID)

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two"] = &Owner{kp2.Public}
	conf2.Data[kp2.Public] = "public2"
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))

	dbg.ErrFatal(c1.ConfigNewCheck())
	al := c1.Proposed
	assert.NotNil(t, al)

	o2, ok := al.Owners["two"]
	assert.True(t, ok)
	assert.True(t, kp2.Public.Equal(o2.Point))
	pub2, ok := al.Data[o2.Point]
	assert.True(t, ok)
	assert.Equal(t, "public2", pub2)
}

func TestIdentity_ConfigNewVote(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	//services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())
	dbg.Print(c1.ID)

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two"] = &Owner{kp2.Public}
	conf2.Data[kp2.Public] = "public2"
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))
	dbg.ErrFatal(c1.ConfigNewCheck())
	hash, err := conf2.Hash()
	dbg.ErrFatal(err)
	dbg.ErrFatal(c1.ConfigNewVote(hash, true))
	//dbg.ErrFatal(c1.)
}

func TestIdentity_AttachToIdentity(t *testing.T) {
	l := sda.NewLocalTest()
	hosts, el, _ := l.GenTree(5, true, true, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	c2 := NewIdentity(el, 50, "two", "public2")
	dbg.ErrFatal(c2.AttachToIdentity(c1.ID))
	for _, s := range services {
		is := s.(*Service)
		if len(is.Identities) != 1 {
			t.Fatal("The configuration hasn't been proposed in all services")
		}
	}
}

func TestIdentity_ConfigUpdate(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	//services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	c2 := NewIdentity(el, 50, "two", "public2")
	dbg.ErrFatal(c2.ConfigUpdate())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two"] = &Owner{kp2.Public}
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))

	dbg.ErrFatal(c1.ConfigUpdate())
	assert.NotNil(t, c1.Proposed)
	o2 := c1.Proposed.Owners[c2.ManagerStr]
	if !o2.Point.Equal(c2.Entity.Public) {
		t.Fatal("Added owner is not c2")
	}
}
