package main

import (
	"io/ioutil"
	"os"
	"testing"

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

	_, err = doCreate("test", roster)
	require.Nil(t, err)
	_, err = doCreate("test2", roster)
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
}
