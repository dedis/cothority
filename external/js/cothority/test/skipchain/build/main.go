// This is a part of the JavaScript integration test.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/cothority/v3/skipchain"
	_ "go.dedis.ch/cothority/v3/status/service"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/cosi"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
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

	reply, err := client.GetUpdateChain(ro, inter.Hash)
	log.ErrFatal(err)
	block := reply.Update[0]
	link := block.ForwardLink[len(block.ForwardLink)-1]
	fmt.Println("Link signature: ", len(link.Signature.Sig))
	policy := cosi.NewThresholdPolicy(byzcoinx.Threshold(len(ro.List)))
	publics := make([]kyber.Point, len(ro.List))
	for i := range ro.List {
		publics[i] = ro.List[i].Public
	}
	err = cosi.Verify(cothority.Suite, publics, link.Signature.Msg, link.Signature.Sig, policy)
	if err != nil {
		panic(err)
	}
	group := &app.Group{Roster: ro}
	log.ErrFatal(group.Save(cothority.Suite, "public.toml"))

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
