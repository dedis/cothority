package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/stretchr/testify/require"
)

func TestClient_LatestUpdates(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)

	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, chain1.blocks[0].release, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	//verifyExistence(t, hosts, sc, policy1.Name, true)

	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{sc, chain1.blocks[1].release})
	log.ErrFatal(err)
	sc2 := upr.(*UpdatePackageRet).SwupChain

	client := NewClient(roster)
	lbret, err := client.LatestUpdates([]skipchain.SkipBlockID{sc.Data.Hash})
	log.ErrFatal(err)
	require.Equal(t, 1, len(lbret.Updates))
	require.Equal(t, sc2.Data.Hash, lbret.Updates[0][1].Hash)

	cpr, err = service.CreatePackage(nil,
		&CreatePackage{roster, chain2.blocks[0].release, 2, 10})
	log.ErrFatal(err)
	sc3 := cpr.(*CreatePackageRet).SwupChain
	//verifyExistence(t, hosts, sc, policy1.Name, true)

	upr, err = service.UpdatePackage(nil,
		&UpdatePackage{sc3, chain2.blocks[1].release})
	log.ErrFatal(err)
	sc4 := upr.(*UpdatePackageRet).SwupChain

	lbret, err = client.LatestUpdates([]skipchain.SkipBlockID{sc.Data.Hash,
		sc3.Data.Hash})
	log.ErrFatal(err)
	require.Equal(t, 2, len(lbret.Updates))
	require.Equal(t, 2, len(lbret.Updates[0]))
	require.Equal(t, 2, len(lbret.Updates[1]))
	require.Equal(t, sc2.Data.Hash, lbret.Updates[0][1].Hash)
	require.Equal(t, sc4.Data.Hash, lbret.Updates[1][1].Hash)

	lbret, err = client.LatestUpdates([]skipchain.SkipBlockID{sc2.Data.Hash,
		sc3.Data.Hash})
	log.ErrFatal(err)
	require.Equal(t, 1, len(lbret.Updates))
	require.Equal(t, 2, len(lbret.Updates[0]))
}
