package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

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
	defer l.CloseAll()
	_, roster, _ := l.GenTree(2, true)

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
