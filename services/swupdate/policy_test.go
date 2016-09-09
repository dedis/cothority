package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/require"
)

func TestNewDebianRelease(t *testing.T) {
	require := require.New(t)
	dr, err := NewDebianRelease("", "")
	require.NotNil(err)

	dr, err = NewDebianRelease("197001010000,ls,0.01", "")
	require.Equal("197001010000", dr.Snapshot)
	require.Equal("ls", dr.Name)
	require.Equal("0.01", dr.Version)
}

func TestGetReleases(t *testing.T) {
	require := require.New(t)
	dr, err := GetReleases("doesntexist")
	require.NotNil(err)
	dr, err = GetReleases("snapshot/updates.csv")
	log.ErrFatal(err)
	require.Equal(4, len(dr))
	require.Equal("ls", dr[0].Name)
	require.Equal("cp", dr[1].Name)
	require.Equal("0.1", dr[1].Version)
	for _, d := range dr {
		p := d.Policy
		require.Equal("1234caffee", p.BinaryHash)
		require.Equal("deadbeef", p.SourceHash)
		require.Equal(3, p.Threshold)
		for i, k := range p.Keys {
			pgp := NewPGPPublic(k)
			log.ErrFatal(pgp.Verify([]byte(p.SourceHash), d.Signatures[i]))
		}
	}
}
