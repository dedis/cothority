package identity

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestIdentity_ConfigNewCheck(t *testing.T) {
	t.Skip()
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	//services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

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
		is.identitiesMutex.Lock()
		if len(is.identities) != 1 {
			t.Fatal("The configuration hasn't been proposed in all services")
		}
		is.identitiesMutex.Unlock()
	}
}

func TestIdentity_ConfigUpdate(t *testing.T) {
	t.Skip()
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	//services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	c2 := NewIdentity(el, 50, "two", "public2")
	c2.ID = c1.ID
	dbg.ErrFatal(c2.ConfigUpdate())

	assert.NotNil(t, c2.Config)
	o1 := c2.Config.Owners[c1.ManagerStr]
	if !o1.Point.Equal(c1.Entity.Public) {
		t.Fatal("Owner is not c1")
	}
}
