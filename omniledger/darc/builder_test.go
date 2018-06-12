package darc_test

import (
	"testing"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/stretchr/testify/require"
)

func TestDarcBuilder_New(t *testing.T) {
	b := darc.NewDarcBuilder()
	_, err := b.Build()
	require.Nil(t, err)
}

func TestDarcBuilder_Set(t *testing.T) {
	var v uint64 = 1
	desc := []byte("hello")
	id := darc.ID([]byte("some ID"))

	b := darc.NewDarcBuilder()
	b.SetVersion(v)
	b.SetDescription(desc)
	b.SetBaseID(id)

	d, err := b.Build()
	require.Nil(t, err)

	require.Equal(t, v, d.Version)
	require.Equal(t, desc, d.Description)
	require.True(t, id.Equal(d.BaseID))
}

func TestDarcBuilder_Evolution(t *testing.T) {
	// suppose we have a darc d
	d := darc.Darc{}
	b := d.StartEvolution()
	b.SetDescription([]byte("abc"))
	b.AddRule("write", []byte("some expr"))
	_, err := b.Build()
	require.Nil(t, err)
}
