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

var serviceId onet.ServiceID

func init() {
	serviceId = onet.ServiceFactory.ServiceID(Name)
}

func TestMain(m *testing.M) {
	log.MainTest(m, 3)
}

func TestService_FinalizeRequest(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nbrNodes := 3
	nbrAtt := 4
	nodes, r, _ := local.GenTree(nbrNodes, true)

	// Get all service-instances
	desc, atts, services := storeDesc(local.GetServices(nodes, serviceId), r, nbrAtt)
	descHash := desc.Hash()
	// Clear config of first one
	services[0].data.Final = nil

	// Send a request to all services
	for _, s := range services {
		_, err := s.FinalizeRequest(&FinalizeRequest{descHash, atts})
		require.NotNil(t, err)
	}

	// Create a new config for the first one
	services[0].StoreConfig(&StoreConfig{desc})

	// Send a request to all services but the first one
	for _, s := range services[1:] {
		log.Lvl2("Asking", s, "to finalize")
		_, err := s.FinalizeRequest(&FinalizeRequest{descHash, atts})
		require.NotNil(t, err)
	}

	log.Lvl2("Final finalizing")
	final, err := services[0].FinalizeRequest(&FinalizeRequest{descHash, atts})
	require.Nil(t, err)
	require.NotNil(t, final)
}

func TestService_CheckConfig(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	desc, atts, srvcs := storeDesc(local.GetServices(nodes, serviceId), r, 2)
	for _, s := range srvcs {
		s.data.Final.Attendees = make([]abstract.Point, len(atts))
		copy(s.data.Final.Attendees, atts)
	}

	cc := &CheckConfig{[]byte{}, atts}
	srvcs[0].SendRaw(r.List[1], cc)
	require.Nil(t, <-srvcs[0].ccChannel)

	cc.PopHash = desc.Hash()
	srvcs[0].SendRaw(r.List[1], cc)
	require.NotNil(t, <-srvcs[0].ccChannel)
	require.Equal(t, 2, len(srvcs[0].data.Final.Attendees))
	require.Equal(t, 2, len(srvcs[1].data.Final.Attendees))

	cc.Attendees = atts[:1]
	srvcs[0].SendRaw(r.List[1], cc)
	require.NotNil(t, <-srvcs[0].ccChannel)
	require.Equal(t, 1, len(srvcs[0].data.Final.Attendees))
	require.Equal(t, 1, len(srvcs[1].data.Final.Attendees))
}

func TestService_CheckConfigReply(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	desc, atts, srvcs := storeDesc(local.GetServices(nodes, serviceId), r, 2)
	s0 := srvcs[0]
	s0.data.Final.Attendees = make([]abstract.Point, len(atts))
	copy(s0.data.Final.Attendees, atts)

	ccr := &CheckConfigReply{0, desc.Hash(), atts}
	req := &network.Envelope{
		Msg:            ccr,
		ServerIdentity: nodes[1].ServerIdentity,
	}

	s0.CheckConfigReply(req)
	<-s0.ccChannel
	require.Equal(t, 2, len(s0.data.Final.Attendees))

	ccr.Attendees = atts[:1]
	req.Msg = ccr
	s0.CheckConfigReply(req)
	<-s0.ccChannel
	require.Equal(t, 2, len(s0.data.Final.Attendees))

	ccr.PopStatus = 3
	req.Msg = ccr
	s0.CheckConfigReply(req)
	<-s0.ccChannel
	require.Equal(t, 1, len(s0.data.Final.Attendees))
}

func TestService_StoreConfig(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	service := local.GetServices(nodes, serviceId)[0].(*Service)
	desc := &PopDesc{
		Name:     "test",
		DateTime: "tomorrow",
		Roster:   onet.NewRoster(r.List),
	}
	service.data.Public = network.Suite.Point().Null()
	msg, cerr := service.StoreConfig(&StoreConfig{desc})
	log.ErrFatal(cerr)
	_, ok := msg.(*StoreConfigReply)
	require.True(t, ok)
	hash := desc.Hash()
	require.Equal(t, service.data.Final.Desc.Hash(), hash)
}

func storeDesc(srvcs []onet.Service, el *onet.Roster, nbr int) (*PopDesc, []abstract.Point, []*Service) {
	desc := &PopDesc{
		Name:     "test",
		DateTime: "tomorrow",
		Roster:   onet.NewRoster(el.List),
	}
	atts := make([]abstract.Point, nbr)
	for i := range atts {
		kp := config.NewKeyPair(network.Suite)
		atts[i] = kp.Public
	}
	sret := []*Service{}
	for _, s := range srvcs {
		sret = append(sret, s.(*Service))
		s.(*Service).data.Public = network.Suite.Point().Null()
		s.(*Service).StoreConfig(&StoreConfig{desc})
	}
	return desc, atts, sret
}
