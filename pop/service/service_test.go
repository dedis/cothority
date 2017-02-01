package service

import (
	"testing"

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

func TestServiceSave(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	servers := local.GenServers(1)
	service := local.GetServices(servers, serviceId)[0].(*Service)
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
	service := local.GetServices(servers, serviceId)[0].(*Service)
	require.Equal(t, "", service.data.Pin)
	pub, _ := network.Suite.Point().Pick(nil, network.Suite.Cipher([]byte("test")))
	_, cerr := service.PinRequest(&PinRequest{"", pub})
	require.NotNil(t, cerr)
	require.NotEqual(t, "", service.data.Pin)
	_, cerr = service.PinRequest(&PinRequest{service.data.Pin, pub})
	log.Error(cerr)
	require.Equal(t, service.data.Public, pub)
}
