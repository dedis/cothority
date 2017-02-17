package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/eddsa"
	"gopkg.in/dedis/crypto.v0/random"
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
	fsStr, err := fs.ToToml()
	log.ErrFatal(err)
	log.Lvlf2("%x", fsStr)
	fs2, err := NewFinalStatementFromToml([]byte(fsStr))
	log.ErrFatal(err)
	require.Equal(t, fs.Desc.DateTime, fs2.Desc.DateTime)
	require.True(t, fs.Desc.Roster.Aggregate.Equal(fs2.Desc.Roster.Aggregate))
	require.True(t, fs.Attendees[0].Equal(fs2.Attendees[0]))
}

func TestFinalStatement_Verify(t *testing.T) {
	eddsa := eddsa.NewEdDSA(random.Stream)
	si := network.NewServerIdentity(eddsa.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:     "test",
			DateTime: "yesterday",
			Roster:   roster,
		},
		Attendees: []abstract.Point{eddsa.Public},
	}
	require.NotNil(t, fs.Verify())
	h, err := fs.Hash()
	log.ErrFatal(err)
	fs.Signature, err = eddsa.Sign(h)
	log.ErrFatal(err)
	require.Nil(t, fs.Verify())
	fs.Attendees = append(fs.Attendees, eddsa.Public)
	require.NotNil(t, fs.Verify())
}
