package libsc_test

import (
	"testing"

	"fmt"

	"github.com/dedis/cothority/skipchain/internal"
	"github.com/dedis/cothority/skipchain/libsc"
	_ "github.com/dedis/cothority/skipchain/service"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1/log"
)

var invalidID = libsc.SkipBlockID([]byte{1, 2, 3})

func TestSkipBlockBunch_Length(t *testing.T) {
	tb := internal.NewTestBunch(3, 2, 1)
	defer tb.End()

	require.Equal(t, 2, tb.Bunch.Length())
}

func TestSkipBlockBunch_GetResponsible(t *testing.T) {
	tb := internal.NewTestBunch(3, 2, 1)
	defer tb.End()

	_, err := tb.Bunch.GetResponsible(nil)
	require.NotNil(t, err)

	_, err = tb.Bunch.GetResponsible(tb.Genesis)
	require.Nil(t, err)
	_, err = tb.Bunch.GetResponsible(tb.Skipblocks[1])
	require.Nil(t, err)

	bl := tb.Bunch.Latest.Copy()
	bl.BackLinkIDs = []libsc.SkipBlockID{}
	_, err = tb.Bunch.GetResponsible(bl)
	require.NotNil(t, err)
	bl.BackLinkIDs = []libsc.SkipBlockID{invalidID}
	_, err = tb.Bunch.GetResponsible(bl)
	require.NotNil(t, err)
}

func TestSkipBlockBunch_Parents(t *testing.T) {
	tb := internal.NewTestBunch(3, 2, 1)
	defer tb.End()

	newSC, cerr := tb.Client.CreateGenesis(tb.Roster, 1, 1, libsc.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	tb.Bunch.Store(newSC)
}

func TestSkipBlockBunch_VerifyLinks(t *testing.T) {
	tb := internal.NewTestBunch(3, 2, 4)
	defer tb.End()

	err := tb.Bunch.VerifyLinks(tb.Genesis)
	log.ErrFatal(err)

	g := tb.Genesis.Copy()
	g.ForwardLink[0].Hash = []byte{}
	require.NotNil(t, tb.Bunch.VerifyLinks(g))

	g.BackLinkIDs = []libsc.SkipBlockID{}
	require.NotNil(t, tb.Bunch.VerifyLinks(g))

	last := tb.Bunch.Latest.Copy()
	last.BackLinkIDs = []libsc.SkipBlockID{invalidID}
	require.NotNil(t, tb.Bunch.VerifyLinks(last))

	last = tb.Bunch.Latest.Copy()
	last.Hash = invalidID
	require.NotNil(t, tb.Bunch.VerifyLinks(last))

}

func TestSkipBlockBunch_GetFuzzy(t *testing.T) {
	tb := internal.NewTestBunch(3, 2, 0)
	defer tb.End()

	id := fmt.Sprintf("%x", tb.Genesis.Hash)
	require.NotNil(t, tb.Bunch.GetFuzzy(id))
	require.NotNil(t, tb.Bunch.GetFuzzy(id[0:10]))
	require.NotNil(t, tb.Bunch.GetFuzzy(id[10:]))
	require.NotNil(t, tb.Bunch.GetFuzzy(id[10:20]))
	require.Nil(t, tb.Bunch.GetFuzzy(fmt.Sprintf("%x", invalidID)))
}
