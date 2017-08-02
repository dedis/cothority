package skipchain

import (
	"fmt"
	"testing"

	"github.com/dedis/backup.cothority.170531/skipchain"
	"github.com/dedis/cothority/skipchain/libsc"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestSBBStorage_AddBunch(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 0)
	defer tb.End()

	sbbs := skipchain.NewSBBStorage()
	require.NotNil(t, sbbs.AddBunch(tb.genesis))
	require.Nil(t, sbbs.AddBunch(tb.genesis))
}

func TestSBBStorage_Store(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 0)
	defer tb.End()

	newGen, cerr := tb.client.CreateGenesis(tb.roster, 2, 2, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	tb.storage.Store(newGen)
}

func TestSBBStorage_GetFuzzy(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 0)
	defer tb.End()

	sbbs := tb.storage
	id := fmt.Sprintf("%x", tb.genesis.Hash)
	require.NotNil(t, sbbs.GetFuzzy(id))
	require.NotNil(t, sbbs.GetFuzzy(id[0:10]))
	require.NotNil(t, sbbs.GetFuzzy(id[10:]))
	require.NotNil(t, sbbs.GetFuzzy(id[10:20]))
	require.Nil(t, sbbs.GetFuzzy(fmt.Sprintf("%x", invalidID)))
}

func TestSBBStorage_GetFromGenesisByID(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 0)
	defer tb.End()

	sb := tb.storage.GetFromGenesisByID(tb.genesis.Hash, tb.genesis.Hash)
	require.True(t, sb.Equal(tb.genesis))
}

func TestSBBStorage_GetLatest(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 0)
	defer tb.End()

	sb := tb.storage.GetLatest(tb.genesis.Hash)
	require.True(t, sb.Equal(tb.bunch.Latest))
}

func TestNewSBBStorage_VerifyLinks(t *testing.T) {
	tb := libsc_test.NewTestBunch(3, 2, 2)
	defer tb.End()

	for _, sb := range tb.skipblocks {
		log.ErrFatal(tb.storage.VerifyLinks(sb))
	}

	child, cerr := tb.client.CreateGenesis(tb.roster, 1, 1, skipchain.VerificationNone,
		nil, tb.bunch.GenesisID)
	require.NotNil(t, tb.storage.VerifyLinks(child))
	cc := child.Copy()
	cc.BackLinkIDs = []libsc.SkipBlockID{}
	require.NotNil(t, tb.storage.VerifyLinks(cc))

	tb.storage.Store(child)
	log.ErrFatal(cerr)
	log.ErrFatal(tb.storage.VerifyLinks(child))

	cc = child.Copy()
	cc.Hash = invalidID
	tb.storage.Store(cc)
	require.NotNil(t, tb.storage.VerifyLinks(cc))
}
