package service

import (
	"testing"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestFinalStatement_ToToml(t *testing.T) {
	pk := config.NewKeyPair(network.Suite)
	si := network.NewServerIdentity(pk.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:     "test",
			DateTime: "yesterday",
			Roster:   roster,
		},
		Attendees: []abstract.Point{pk.Public},
	}
	fs.Signature = fs.Desc.Hash()
	fsStr := fs.ToToml()
	log.LLvl2(fsStr)
	fs2 := NewFinalStatementFromString(fsStr)
	require.Equal(t, fs.Desc.DateTime, fs2.Desc.DateTime)
	require.True(t, fs.Desc.Roster.Aggregate.Equal(fs2.Desc.Roster.Aggregate))
	require.True(t, fs.Attendees[0].Equal(fs2.Attendees[0]))
}
