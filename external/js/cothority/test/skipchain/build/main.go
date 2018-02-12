package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
)

func main() {
	n := 5
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(n, true)
	defer l.CloseAll()

	clients := make(map[int]*skipchain.Client)
	for i := range [8]byte{} {
		clients[i] = newTestClient(l)
	}
	_, inter, err := clients[0].CreateRootControl(ro, ro, nil, 1, 1, 1)
	log.ErrFatal(err)

	_, err = clients[2%8].GetUpdateChain(inter.Roster, inter.Hash)
	log.ErrFatal(err)

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

	id := hex.EncodeToString(inter.Hash)
	fd, err := os.Create("genesis.txt")
	log.ErrFatal(err)
	fd.WriteString(id)
	fd.Close()
	fmt.Println("OK")
	time.Sleep(3600 * time.Second)

}

func write(name string, i interface{}) {
	fd, err := os.Create(name)
	log.ErrFatal(err)
	defer fd.Close()
	log.ErrFatal(toml.NewEncoder(fd).Encode(i))
}

func newTestClient(l *onet.LocalTest) *skipchain.Client {
	c := skipchain.NewClient()
	c.Client = l.NewClient("Skipchain")
	return c
}
