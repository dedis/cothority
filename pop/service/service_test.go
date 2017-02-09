package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

var serviceID onet.ServiceID

func init() {
	serviceID = onet.ServiceFactory.ServiceID(Name)
}

func TestMain(m *testing.M) {
	log.MainTest(m, 3)
}

func TestServiceSave(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	servers := local.GenServers(1)
	service := local.GetServices(servers, serviceID)[0].(*Service)
	service.data.Pin = "1234"
	service.save()
	service.data.Pin = ""
	log.ErrFatal(service.tryLoad())
	require.Equal(t, "1234", service.data.Pin)
}
func TestService_PinRequest(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	servers := local.GenServers(1)
	service := local.GetServices(servers, serviceID)[0].(*Service)
	require.Equal(t, "", service.data.Pin)
	pub, _ := network.Suite.Point().Pick(nil, network.Suite.Cipher([]byte("test")))
	_, cerr := service.PinRequest(&PinRequest{"", pub})
	require.NotNil(t, cerr)
	require.NotEqual(t, "", service.data.Pin)
	_, cerr = service.PinRequest(&PinRequest{service.data.Pin, pub})
	log.Error(cerr)
	require.Equal(t, service.data.Public, pub)
}

func TestService_StoreConfig(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	service := local.GetServices(nodes, serviceID)[0].(*Service)
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

func TestService_CheckConfig(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	desc, atts, srvcs := storeDesc(local.GetServices(nodes, serviceID), r, 2)
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
	desc, atts, srvcs := storeDesc(local.GetServices(nodes, serviceID), r, 2)
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

func TestService_FinalizeRequest(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nbrNodes := 3
	nbrAtt := 4
	nodes, r, _ := local.GenTree(nbrNodes, true)

	// Get all service-instances
	desc, atts, services := storeDesc(local.GetServices(nodes, serviceID), r, nbrAtt)
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
	fin, ok := final.(*FinalizeResponse)
	require.True(t, ok)
	require.Nil(t, fin.Final.Verify())
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
