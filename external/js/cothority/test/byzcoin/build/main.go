// This is a part of the JavaScript integration test.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	_ "github.com/dedis/cothority/status/service"
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

	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(n, true)
	defer l.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, ro, []string{"spawn:dummy"}, signer.Identity())
	log.ErrFatal(err)
	msg.BlockInterval = 100 * time.Millisecond

	_, resp, err := byzcoin.NewLedger(msg, false)
	log.ErrFatal(err)

	group := &app.Group{Roster: ro}
	log.ErrFatal(group.Save(cothority.Suite, "public.toml"))

	id := hex.EncodeToString(resp.Skipblock.SkipChainID())
	fd, err := os.Create("genesis.txt")
	log.ErrFatal(err)
	fd.WriteString(id)
	fd.Close()
	fmt.Println("OK")
	time.Sleep(3600 * time.Second)
}
