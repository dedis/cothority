package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/eddsa"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

var tSuite = cothority.Suite

func TestFinalStatement_ToToml(t *testing.T) {
	pk := key.NewKeyPair(tSuite)
	si := network.NewServerIdentity(pk.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:     "test",
			DateTime: "yesterday",
			Roster:   roster,
		},
		Attendees: []kyber.Point{pk.Public},
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
	eddsa := eddsa.NewEdDSA(random.New())
	si := network.NewServerIdentity(eddsa.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:     "test",
			DateTime: "yesterday",
			Roster:   roster,
		},
		Attendees: []kyber.Point{eddsa.Public},
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
