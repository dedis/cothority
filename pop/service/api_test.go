package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

var tSuite = cothority.Suite

func TestClient_VerifyLink(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(1, true)
	defer l.CloseAll()
	addr := roster.List[0].Address
	services := l.GetServices(servers, onet.ServiceFactory.ServiceID(Name))
	service := services[0].(*Service)
	c := NewClient()
	kp := key.NewKeyPair(cothority.Suite)

	err := c.VerifyLink(addr, kp.Public)
	require.NotNil(t, err)
	err = c.PinRequest(addr, "", kp.Public)
	require.NotNil(t, err)
	err = c.PinRequest(addr, service.data.Pin, kp.Public)
	require.Nil(t, err)
	err = c.VerifyLink(addr, kp.Public)
	require.Nil(t, err)
}

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
	kp := key.NewKeyPair(pairingSuite)
	si := network.NewServerIdentity(kp.Public, network.NewAddress(network.PlainTCP, "0:2000"))
	roster := onet.NewRoster([]*network.ServerIdentity{si})
	fs := &FinalStatement{
		Desc: &PopDesc{
			Name:     "test",
			DateTime: "yesterday",
			Roster:   roster,
		},
		Attendees: []kyber.Point{kp.Public},
	}
	require.NotNil(t, fs.Verify())
	h, err := fs.Hash()
	log.ErrFatal(err)
	fs.Signature, err = bls.Sign(pairingSuite, kp.Private, h)
	log.ErrFatal(err)
	require.Nil(t, fs.Verify())
	fs.Attendees = append(fs.Attendees, kp.Public)
	require.NotNil(t, fs.Verify())
}

func TestClient_GetLink(t *testing.T) {
	ts := newTSer(t)
	defer ts.Close()

	glr, err := NewClient().GetLink(ts.addr)
	require.Nil(t, err)
	kp := key.NewKeyPair(cothority.Suite)
	ts.services[0].data.Public = kp.Public
	glr, err = NewClient().GetLink(ts.addr)
	require.Nil(t, err)
	require.True(t, glr.Equal(kp.Public))
}

func TestClient_GetFinalStatement(t *testing.T) {
	ts := newTSer(t)
	defer ts.Close()

	fss, err := NewClient().GetFinalStatements(ts.addr)
	require.Nil(t, err)
	require.Equal(t, 0, len(fss))

	ts.services[0].data.Finals[ts.fsIDstr] = ts.fs

	fss, err = NewClient().GetFinalStatements(ts.addr)
	require.Nil(t, err)
	require.Equal(t, 1, len(fss))
	require.NotNil(t, fss[ts.fsIDstr])
	fsIDC, err := fss[ts.fsIDstr].Hash()
	require.Nil(t, err)
	require.Equal(t, ts.fsID, fsIDC)
}

func TestClient_StoreGetKeys(t *testing.T) {
	ts := newTSer(t)
	defer ts.Close()

	gkr, err := NewClient().GetKeys(ts.addr, ts.fsID)
	require.NotNil(t, err)
	require.Equal(t, 0, len(gkr))

	var keys []kyber.Point
	for i := 0; i < 4; i++ {
		kp := key.NewKeyPair(cothority.Suite)
		keys = append(keys, kp.Public)
	}

	err = NewClient().StoreKeys(ts.addr, ts.fsID, keys)
	require.Nil(t, err)
	gkr, err = NewClient().GetKeys(ts.addr, ts.fsID)
	require.Nil(t, err)
	for i, p := range gkr {
		require.True(t, keys[i].Equal(p))
	}
}

func TestClient_StoreGetInstanceID(t *testing.T) {
	ts := newTSer(t)
	defer ts.Close()

	iid := byzcoin.NewInstanceID(random.Bits(256, true, random.New()))
	darcID := darc.ID(random.Bits(256, true, random.New()))
	err := NewClient().StoreInstanceID(ts.addr, ts.fsID, iid, darcID)
	require.Nil(t, err)

	gii, dID, err := NewClient().GetInstanceID(ts.addr, ts.fsID)
	require.Nil(t, err)
	require.True(t, gii.Equal(iid))
	require.True(t, darcID.Equal(dID))
}

func TestClient_StoreGetSigner(t *testing.T) {
	ts := newTSer(t)
	defer ts.Close()

	gs, err := NewClient().GetSigner(ts.addr, ts.fsID)
	require.NotNil(t, err)

	kp := key.NewKeyPair(cothority.Suite)
	signer := darc.NewSignerEd25519(kp.Public, kp.Private)

	err = NewClient().StoreSigner(ts.addr, ts.fsID, signer)
	require.Nil(t, err)
	gs, err = NewClient().GetSigner(ts.addr, ts.fsID)
	require.Nil(t, err)
	p, err := gs.GetPrivate()
	require.Nil(t, err)
	require.True(t, p.Equal(kp.Private))
}

type tSer struct {
	local    *onet.LocalTest
	servers  []*onet.Server
	roster   *onet.Roster
	services []*Service
	addr     network.Address
	fs       *FinalStatement
	fsID     []byte
	fsIDstr  string
}

func newTSer(t *testing.T) (ts tSer) {
	ts.local = onet.NewTCPTest(cothority.Suite)
	ts.servers, ts.roster, _ = ts.local.GenTree(1, true)
	ts.addr = ts.roster.List[0].Address
	services := ts.local.GetServices(ts.servers, onet.ServiceFactory.ServiceID(Name))
	for _, s := range services {
		ts.services = append(ts.services, s.(*Service))
	}

	ts.fs = &FinalStatement{
		Desc: &PopDesc{
			Name:   "test",
			Roster: ts.roster,
		},
	}
	var err error
	ts.fsID, err = ts.fs.Hash()
	require.Nil(t, err)
	ts.fsIDstr = string(ts.fsID)
	return
}

func (ts *tSer) Close() {
	ts.local.CloseAll()
}
