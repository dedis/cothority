// This is a part of the JavaScript integration test.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
)

const defaultN = 5
const defaultBlocks = 5

func main() {
	n := defaultN
	var err error
	if len(os.Args) > 1 {
		n, err = strconv.Atoi(os.Args[1])
		log.ErrFatal(err)
	}
	blocks := defaultBlocks
	if len(os.Args) > 2 {
		blocks, err = strconv.Atoi(os.Args[2])
		log.ErrFatal(err)
	}
	var message = []byte("Hello World")
	if len(os.Args) > 3 {
		message = []byte(os.Args[3])
	}

	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(n, true)
	defer l.CloseAll()

	client := newTestClient(l)
	_, inter, err := client.CreateRootControl(ro, ro, nil, 1, 1, 1)
	log.ErrFatal(err)

	var latest = inter
	for i := 0; i < blocks; i++ {
		sb, err := client.StoreSkipBlock(latest, ro, message)
		log.ErrFatal(err)
		latest = sb.Latest
	}

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
