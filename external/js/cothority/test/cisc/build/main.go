package main

import (
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/identity"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/kyber/util/key"
	"strconv"
)

func main() {

	n := 5
	l := onet.NewTCPTest(cothority.Suite)
	hosts, ro, _ := l.GenTree(n, true)
	services := l.GetServices(hosts, identity.IdentityService())
	defer l.CloseAll()

	// creates the clients
	clients := make(map[int]*identity.Identity)
	for i := range [8]byte{} {
		clients[i] = createIdentity(l, services, ro, "client-"+strconv.Itoa(i))
	}

	// saves the roster (of clients) into public.toml
	serversToml := make([]*app.ServerToml, n)
	for i, si := range ro.List {
		serversToml[i] = app.NewServerToml(
			cothority.Suite,
			si.Public,
			si.Address,
			si.Description,
		)
	}
	group := app.NewGroupToml(serversToml...)
	log.ErrFatal(group.Save("public.toml"))

	c0 := clients[0] // = newTestIdentity(localTCPTest, roster)
	log.Lvlf1("Initial data %+v, %+v\n", c0.Data.Storage, c0.Data.Votes) // map[], map[]

	newData := c0.Data.Copy()
	newData.Storage["ninja"] = "test"
	log.ErrFatal(c0.ProposeSend(newData))
	log.ErrFatal(c0.ProposeUpdate())
	log.ErrFatal(c0.ProposeVote(true))

	log.Lvlf1("After propose %+v, %+v\n", c0.Data.Storage, c0.Data.Votes) // map[], map[]

	// creates a CISC
	//_, inter, err := clients[0].CreateRootControl(ro, ro, nil, 1, 1, 1)
	//log.ErrFatal(err)
	//
	//_, err = clients[2%8].GetUpdateChain(inter.Roster, inter.Hash)
	//log.ErrFatal(err)
	//
	//id := hex.EncodeToString(inter.Hash)
	//fd, err := os.Create("genesis.txt")
	//log.ErrFatal(err)
	//fd.WriteString(id)
	//fd.Close()
	//fmt.Println("OK")
	//time.Sleep(3600 * time.Second)

}


func createIdentity(l *onet.LocalTest, services []onet.Service, el *onet.Roster, name string) *identity.Identity {
	kp1 := key.NewKeyPair(cothority.Suite)
	//kp2 := key.NewKeyPair(cothority.Suite)
	//set := anon.Set([]kyber.Point{kp1.Public, kp2.Public})
	//for _, srvc := range services {
	//	s := srvc.(*identity.Service)
	//	s.Storage.Auth.sets = append(s.Storage.Auth.sets, set)
	//}

	c := NewTestIdentity(el, 1, name, l, kp1)
	log.ErrFatal(c.CreateIdentity(identity.PublicAuth, nil, kp1.Private))
	return c
}
func NewTestIdentity(cothority *onet.Roster, majority int, owner string, local *onet.LocalTest, kp *key.Pair) *identity.Identity {
	id := identity.NewIdentity(cothority, majority, owner, kp)
	id.Client = local.NewClient("Identity")
	return id
}