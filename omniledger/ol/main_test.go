package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dedis/cothority"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

// This is required; without it onet/log/testuitl.go:interestingGoroutines will
// call main.main() interesting.
func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestCli(t *testing.T) {
	dir, err := ioutil.TempDir("", "ol-test")
	if err != nil {
		t.Fatal(err)
	}
	getDataPath = func(in string) string {
		return dir
	}
	defer os.RemoveAll(dir)

	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(2, true)

	defer l.CloseAll()
	defer func() {
		// Walk the service lists, looking for Omniledgers that we can shut down.
		for _, x := range l.Services {
			for _, y := range x {
				if z, ok := y.(*omniledger.Service); ok {
					z.ClosePolling()
				}
			}
		}
	}()

	// All this mess is to take the roster we have from onet.NewTCPTest
	// and get it into a file that create can read.
	g := &app.GroupToml{
		Servers: make([]*app.ServerToml, len(roster.List)),
	}
	for i, si := range roster.List {
		g.Servers[i] = &app.ServerToml{
			Address: si.Address,
			Public:  si.Public.String(),
		}
	}
	rf := path.Join(dir, "roster.toml")
	err = g.Save(rf)
	require.NoError(t, err)

	interval := 100 * time.Millisecond

	log.Lvl1("create: ")
	b := &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args := []string{"ol", "create", "-roster", rf, "--interval", interval.String()}
	err = cliApp.Run(args)
	require.NoError(t, err)
	require.Contains(t, string(b.Bytes()), "Created")

	// Collect the OL config filename that create() left for us,
	// and make it available for the next tests.
	ol := cliApp.Metadata["OL"]
	require.IsType(t, "", ol)
	os.Setenv("OL", ol.(string))

	log.Lvl1("show: ")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"ol", "show"}
	err = cliApp.Run(args)
	require.NoError(t, err)
	require.Contains(t, string(b.Bytes()), "Roster: 127.0.0.1")
	require.Contains(t, string(b.Bytes()), "spawn:darc")

	log.Lvl1("add: ")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"ol", "add", "--identity", "ed25519:XXX", "spawn:xxx"}
	err = cliApp.Run(args)
	require.NoError(t, err)

	time.Sleep(2 * interval)

	log.Lvl1("show after add: ")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"ol", "show"}
	err = cliApp.Run(args)
	require.NoError(t, err)
	require.Contains(t, string(b.Bytes()), "Roster: 127.0.0.1")
	require.Contains(t, string(b.Bytes()), "spawn:xxx - \"ed25519:XXX\"")
}
