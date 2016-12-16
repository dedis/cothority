package service

import (
	"testing"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

func TestFinalStatement_ToToml(t *testing.T) {
	pk := config.NewKeyPair(network.Suite)
	si := network.NewServerIdentity(pk.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:   "test",
			Date:   "yesterday",
			Roster: roster,
		},
		Attendees: []abstract.Point{pk.Public},
	}
	fs.Signature = fs.Desc.Hash()
	fsStr := fs.ToToml()
	log.LLvl2(fsStr)
	fs2 := NewFinalStatementFromString(fsStr)
	require.Equal(t, fs.Desc.Date, fs2.Desc.Date)
	require.True(t, fs.Desc.Roster.Aggregate.Equal(fs2.Desc.Roster.Aggregate))
	require.True(t, fs.Attendees[0].Equal(fs2.Attendees[0]))
}
