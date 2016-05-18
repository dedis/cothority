package identity

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/stretchr/testify/assert"
	"time"
	"github.com/dedis/crypto/config"
)

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
	for _, s := range services{
		dbg.Lvl3(s.(*Service).identities)
	}

	c1 := NewIdentity(el, 50, "one1", "public1")
	dbg.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Owners["two2"] = &Owner{kp2.Public}
	conf2.Data[kp2.Public] = "public2"
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
