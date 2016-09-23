package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/stretchr/testify/require"
)

func TestNewDebianRelease(t *testing.T) {
	require := require.New(t)
	dr, err := NewDebianRelease("", "", 3)
	require.NotNil(err)

	dr, err = NewDebianRelease("19700101000000,ls,0.01,hash1,hash2", "", 3)
	log.ErrFatal(err)
	require.Equal("19700101000000", dr.Snapshot)
	require.Equal("ls", dr.Policy.Name)
	require.Equal("0.01", dr.Policy.Version)
}

func TestGetReleases(t *testing.T) {
	require := require.New(t)
	dr, err := GetReleases("doesntexist")
	require.NotNil(err)
	dr, err = GetReleases("snapshot/updates.csv")
	log.ErrFatal(err)
	require.Equal(4, len(dr))
	require.Equal("ls", dr[0].Policy.Name)
	require.Equal("cp", dr[1].Policy.Name)
	require.Equal("0.1", dr[1].Policy.Version)
	for _, d := range dr {
		p := d.Policy
		require.Equal("1234caffee", p.BinaryHash)
		require.Equal("deadbeef", p.SourceHash)
		require.Equal(5, p.Threshold)
		for i, k := range p.Keys {
			pgp := NewPGPPublic(k)
			policyBin, err := network.MarshalRegisteredType(p)
			log.ErrFatal(err)
			log.ErrFatal(pgp.Verify(policyBin, d.Signatures[i]))
		}
	}
}
