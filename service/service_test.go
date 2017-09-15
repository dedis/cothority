package service

import (
	"testing"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"

	"github.com/dedis/onchain-secrets"
	"github.com/dedis/onchain-secrets/protocol"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateSkipchains(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)
	log.Lvl2(roster, services)
	leader := services[0].(*Service)

	csr := &ocs.CreateSkipchainsRequest{
		Roster: roster,
	}
	rep, cerr := leader.CreateSkipchains(csr)
	log.ErrFatal(cerr)
	v, ok := leader.Storage.Shared[string(rep.OCS.Hash)]
	require.True(t, ok)
	require.NotNil(t, v)
	require.Equal(t, 0, v.Index)
}

func TestService_DecryptKeyRequest(t *testing.T) {
	test := initTest(5, 3)
	defer test.local.CloseAll()

	dkr := &ocs.DecryptKeyRequest{Read: test.readReply.SB.Hash}
	rep, cerr := test.leader.DecryptKeyRequest(dkr)
	log.ErrFatal(cerr)
	key, err := protocol.DecodeKey(network.Suite, test.createReply.X, test.Cs,
		rep.XhatEnc, test.reader.Secret)
	log.ErrFatal(err)
	require.Equal(t, test.key, key)
}

type test struct {
	local       *onet.LocalTest
	servers     []*onet.Server
	roster      *onet.Roster
	services    []*Service
	leader      *Service
	createReply *ocs.CreateSkipchainsReply
	writeReply  *ocs.WriteReply
	readReply   *ocs.ReadReply
	key         []byte
	U           abstract.Point
	Cs          []abstract.Point
	reader      *config.KeyPair
}

func initTest(nbrNodes, step int) *test {
	t := &test{
		local:    onet.NewTCPTest(),
		services: make([]*Service, nbrNodes),
	}
	// generate 5 servers, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	t.servers, t.roster, _ = t.local.GenTree(nbrNodes, true)
	services := t.local.GetServices(t.servers, templateID)
	log.Lvl2(t.roster, services)

	for i, s := range services {
		t.services[i] = s.(*Service)
	}
	t.leader = t.services[0]

	var cerr onet.ClientError
	for i := 0; i < step; i++ {
		switch i {
		case 0:
			csr := &ocs.CreateSkipchainsRequest{
				Roster: t.roster,
			}
			t.createReply, cerr = t.leader.CreateSkipchains(csr)
			log.ErrFatal(cerr)
		case 1:
			t.key = random.Bytes(64, random.Stream)
			t.U, t.Cs = protocol.EncodeKey(network.Suite, t.createReply.X, t.key)
			t.reader = config.NewKeyPair(network.Suite)
			wr := &ocs.WriteRequest{
				Write: &ocs.DataOCSWrite{
					Data: []byte{},
					U:    t.U,
					Cs:   t.Cs,
				},
				Readers: &ocs.Darc{
					ID:     []byte{},
					Public: []abstract.Point{t.reader.Public},
				},
				OCS: t.createReply.OCS.Hash,
			}
			t.writeReply, cerr = t.leader.WriteRequest(wr)
			log.ErrFatal(cerr)
		case 2:
			sig, err := crypto.SignSchnorr(network.Suite, t.reader.Secret,
				t.writeReply.SB.Hash)
			log.ErrFatal(err)
			read := &ocs.ReadRequest{
				Read: &ocs.DataOCSRead{
					Public:    t.reader.Public,
					DataID:    t.writeReply.SB.Hash,
					Signature: &sig,
				},
				OCS: t.createReply.OCS.Hash,
			}
			t.readReply, cerr = t.leader.ReadRequest(read)
			log.ErrFatal(cerr)
		}
	}
	return t
}
