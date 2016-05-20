package identity

import (
	"testing"
	"time"

	"io/ioutil"
	"os"

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
	conf2.Data["two"] = "public2"
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))

	dbg.ErrFatal(c1.ConfigNewCheck())
	al := c1.Proposed
	assert.NotNil(t, al)

	o2, ok := al.Owners["two"]
	assert.True(t, ok)
	assert.True(t, kp2.Public.Equal(o2.Point))
	pub2, ok := al.Data["two"]
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

func TestIdentity_CreateIdentity(t *testing.T) {
	//t.Skip()
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(3, true, true, true)
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
	hosts, el, _ := l.GenTree(3, true, true, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two"] = &Owner{kp2.Public}
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))
	time.Sleep(time.Second)

	for _, s := range services {
		is := s.(*Service)
		id1 := is.getIdentityStorage(c1.ID)
		id1.Lock()
		if id1 == nil {
			t.Fatal("Didn't find")
		}
		assert.NotNil(t, id1.Proposed)
		if len(id1.Proposed.Owners) != 2 {
			t.Fatal("The proposed config should have 2 entries now")
		}
		id1.Unlock()
	}
}

func TestIdentity_ConfigNewVote(t *testing.T) {
	l := sda.NewLocalTest()
	hosts, el, _ := l.GenTree(5, true, true, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		dbg.Lvl3(s.(*Service).identities)
	}

	c1 := NewIdentity(el, 50, "one1", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two2"] = &Owner{kp2.Public}
	conf2.Data["two2"] = "public2"
	dbg.ErrFatal(c1.ConfigNewPropose(conf2))
	dbg.ErrFatal(c1.ConfigNewCheck())
	hash, err := conf2.Hash()
	dbg.ErrFatal(err)
	dbg.ErrFatal(c1.ConfigNewVote(hash, true))

	if len(c1.Config.Owners) != 2 {
		t.Fatal("Should have two owners now")
	}
	if len(c1.Config.Data) != 2 {
		t.Fatal("Should have two data-entries now")
	}

	dbg.ErrFatal(c1.ConfigUpdate())
	if len(c1.Config.Owners) != 2 {
		t.Fatal("Update should have two owners now")
	}
}

func TestIdentity_SaveToStream(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()
	id := NewIdentity(el, 50, "one1", "public1")
	tmpfile, err := ioutil.TempFile("", "example")
	dbg.ErrFatal(err)
	defer os.Remove(tmpfile.Name())
	id.SaveToStream(tmpfile)
	tmpfile.Seek(0, 0)
	id2, err := NewIdentityFromStream(tmpfile)
	assert.NotNil(t, id2)
	dbg.ErrFatal(err)
	tmpfile.Close()

	if id.Config.Threshold != id2.Config.Threshold {
		t.Fatal("Loaded threshold is not the same")
	}
	p, p2 := id.Config.Owners["one1"].Point, id2.Config.Owners["one1"].Point
	if !p.Equal(p2) {
		t.Fatal("Public keys are not the same")
	}
	if id.Config.Data["one1"] != id2.Config.Data["one1"] {
		t.Fatal("Owners are not the same", id.Config.Data, id2.Config.Data)
	}
}
