package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/dedis/cothority"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

// This is required; without it onet/log/testuitl.go:interestingGoroutines will
// call main.main() interesting.
func TestMain(m *testing.M) {
	log.MainTest(m)
}

const blockInterval = 1 * time.Second

func Test(t *testing.T) {
	dir, err := ioutil.TempDir("", "el-test")
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
					close(z.CloseQueues)
				}
			}
		}
	}()

	_, err = doCreate("test", roster, blockInterval)
	require.Nil(t, err)
	_, err = doCreate("test2", roster, blockInterval)
	require.Nil(t, err)

	c, err := loadConfigs(getDataPath("el"))
	require.Nil(t, err)
	require.Equal(t, 2, len(c))
	// No need to check the order here, because iotuil.ReadDir returns them
	// sorted by filename = sorted by ID. We don't know which ID will be lower,
	// but for this test we don't care.
	require.True(t, c[0].Name == "test" || c[1].Name == "test")
	if c[0].Name == "test" {
		require.True(t, c[1].Name == "test2")
	}

	args := []string{"el", "log", "-topic", "testTopic1", "-content", "Test Message"}
	err = cliApp.Run(args)
	require.Nil(t, err)

	// Make sure they are commtted to the log (in separate tx, so their
	// order is repeatable.)
	time.Sleep(2 * blockInterval)

	args = []string{"el", "log", "-topic", "testTopic2", "-content", "ldjkf"}
	err = cliApp.Run(args)
	require.Nil(t, err)

	// Make sure they are commtted to the log (in separate tx, so their
	// order is repeatable.)
	time.Sleep(2 * blockInterval)

	t.Log("search: get all")
	b := &bytes.Buffer{}
	cliApp.Metadata["stdout"] = b
	args = []string{"el", "search"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.Contains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by topic")
	b = &bytes.Buffer{}
	cliApp.Metadata["stdout"] = b
	args = []string{"el", "search", "-topic", "testTopic1"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.NotContains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by count")
	b = &bytes.Buffer{}
	cliApp.Metadata["stdout"] = b
	args = []string{"el", "search", "-count", "1"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.NotContains(t, string(b.Bytes()), "ldjkf")

	// It would be interesting to try to write a test for
	// -from/-to, but it would make this test too fragile.
	// TODO:
	// Write a separate test, gated by testing.Slow() that adds
	// one event per second, and then selects some of them using
	// "-from 10s ago -for 5s".
}
