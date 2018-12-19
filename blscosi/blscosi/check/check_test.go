package check

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

var testSuite = pairing.NewSuiteBn256()

// TestMain_Check checks if the CLI command check works correctly
func TestCheck(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	os.Chdir(tmp)

	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	hosts, roster, _ := local.GenTree(10, true)

	publicToml := path.Join(tmp, "public.toml")
	errorToml := path.Join(tmp, "error.toml")

	// missing group file
	err := CothorityCheck("", false)
	require.Error(t, err)

	// corrupted group file
	err = ioutil.WriteFile(errorToml, []byte("abc"), 0644)
	require.NoError(t, err)
	err = CothorityCheck(errorToml, false)
	require.Error(t, err)

	// empty roster
	group := &app.Group{Roster: &onet.Roster{List: []*network.ServerIdentity{}}}
	err = group.Save(testSuite, publicToml)
	require.NoError(t, err)
	err = CothorityCheck(publicToml, false)
	require.Error(t, err)

	// correct request
	group = &app.Group{Roster: roster}
	err = group.Save(testSuite, publicToml)
	require.NoError(t, err)
	err = CothorityCheck(publicToml, true)
	require.NoError(t, err)

	// one failure
	hosts[0].Close()
	err = CothorityCheck(publicToml, false)
	require.Error(t, err)
}
