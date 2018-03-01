package evoting_test

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	_ "github.com/dedis/cothority/evoting/service"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := evoting.NewClient()
	r, _ := c.Ping(roster, 0)
	assert.Equal(t, uint32(1), r.Nonce)
}

func TestLookupSciper(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	_, err := evoting.NewClient().LookupSciper(roster, "")
	require.NotNil(t, err)
	_, err = evoting.NewClient().LookupSciper(roster, "12345")
	require.NotNil(t, err)
	_, err = evoting.NewClient().LookupSciper(roster, "1234567")
	require.NotNil(t, err)
	_, err = evoting.NewClient().LookupSciper(roster, "000000")
	require.NotNil(t, err)
	vcard, err := evoting.NewClient().LookupSciper(roster, "107537")
	require.Nil(t, err)
	require.Equal(t, "Martin Vetterli", vcard.FullName)
	require.Equal(t, "TYPE=INTERNET:martin.vetterli@epfl.ch", vcard.Email)
}
