package main

import (
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/identity"
	"github.com/dedis/onet"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/onet/log"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/kyber"
)

func main() {
	l := onet.NewTCPTest(cothority.Suite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identity.IdentityService())
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*identity.Service).Storage.Identities)
	}

	c1 := createIdentity(l, services, el, "one1")
	data2 := c1.Data.Copy()
	kp2 := key.NewKeyPair(cothority.Suite)
	data2.Device["two2"] = &identity.Device{kp2.Public}
	data2.Storage["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(data2))
	log.ErrFatal(c1.ProposeUpdate())
	log.ErrFatal(c1.ProposeVote(true))

	if len(c1.Data.Device) != 2 {
		log.Fatal("Should have two owners now")
	}
}

func createIdentity(l *onet.LocalTest, services []onet.Service, el *onet.Roster, name string) *identity.Identity {
	kp1 := key.NewKeyPair(cothority.Suite)
	kp2 := key.NewKeyPair(cothority.Suite)
	set := anon.Set([]kyber.Point{kp1.Public, kp2.Public})
	for _, srvc := range services {
		s := srvc.(*identity.Service)
		s.Storage.Auth.AddToSet(set)
	}

	c := NewTestIdentity(el, 50, name, l, kp1)
	log.ErrFatal(c.CreateIdentity(identity.PoPAuth, set, kp1.Private))
	return c
}

func NewTestIdentity(cothority *onet.Roster, majority int, owner string, local *onet.LocalTest, kp *key.Pair) *identity.Identity {
	id := identity.NewIdentity(cothority, majority, owner, kp)
	id.Client = local.NewClient(identity.ServiceName)
	return id
}