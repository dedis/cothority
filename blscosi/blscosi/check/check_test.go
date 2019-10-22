package check

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v4/blscosi"
	"go.dedis.ch/cothority/v4/cosuite"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/app"
	"go.dedis.ch/onet/v4/network"
)

// TestMain_Check checks if the CLI command check works correctly
func TestCheck(t *testing.T) {
	suite := cosuite.NewBlsSuite()

	builder := onet.NewDefaultBuilder()
	builder.SetSuite(suite)
	blscosi.RegisterBlsCoSiService(builder, suite)

	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	os.Chdir(tmp)

	local := onet.NewLocalTest(builder)
	defer local.CloseAll()
	hosts, roster, _ := local.GenTree(10, true)

	publicToml := path.Join(tmp, "public.toml")
	errorToml := path.Join(tmp, "error.toml")

	// missing group file
	err := CothorityCheck(suite, "", false)
	require.Error(t, err)

	// corrupted group file
	err = ioutil.WriteFile(errorToml, []byte("abc"), 0644)
	require.NoError(t, err)
	err = CothorityCheck(suite, errorToml, false)
	require.Error(t, err)

	// empty roster
	group := &app.Group{Roster: &onet.Roster{List: []*network.ServerIdentity{}}}
	err = group.Save(publicToml)
	require.NoError(t, err)
	err = CothorityCheck(suite, publicToml, false)
	require.Error(t, err)

	// correct request
	group = &app.Group{Roster: roster}
	err = group.Save(publicToml)
	require.NoError(t, err)
	err = CothorityCheck(suite, publicToml, true)
	require.NoError(t, err)

	// one failure
	hosts[0].Close()
	err = CothorityCheck(suite, publicToml, false)
	require.Error(t, err)
}
