package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/identity"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
)

func main() {
	n := 5
	l := onet.NewTCPTest(cothority.Suite)
	hosts, el, _ := l.GenTree(n, true)
	services := l.GetServices(hosts, identity.IdentityService())
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*identity.Service).Storage.Identities)
	}
	c1 := createIdentity(l, services, el, "one1")

	serversToml := make([]*app.ServerToml, n)
	for i, si := range el.List {
		serversToml[i] = app.NewServerToml(
			cothority.Suite,
			si.Public,
			si.Address,
			si.Description,
		)
	}
	group := app.NewGroupToml(serversToml...)
	log.ErrFatal(group.Save("public.toml"))

	log.Lvl3("Before data add", c1.Data.Storage)
	data2 := c1.Data.Copy()
	data2.Storage["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(data2))
	log.ErrFatal(c1.ProposeUpdate())
	log.ErrFatal(c1.ProposeVote(true))
	log.ErrFatal(c1.DataUpdate())

	log.Lvl3("After data add", c1.Data.Storage)

	id := hex.EncodeToString(c1.ID)
	fd, err := os.Create("genesis.txt")
	log.ErrFatal(err)
	fd.WriteString(id)
	fd.Close()
	fmt.Println("OK")
	time.Sleep(3600 * time.Second)
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

// NewTestIdentity returns a identity.Identity client
func NewTestIdentity(cothority *onet.Roster, majority int, owner string, local *onet.LocalTest, kp *key.Pair) *identity.Identity {
	id := identity.NewIdentity(cothority, majority, owner, kp)
	id.Client = local.NewClient(identity.ServiceName)
	return id
}
