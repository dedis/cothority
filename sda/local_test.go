package sda

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
)

const clientServiceName = "ClientService"

func init() {
	RegisterNewService(clientServiceName, newClientService)
}

func TestGenLocalHost(t *testing.T) {
	l := NewLocalTest()
	hosts := l.genLocalHosts(2)
	defer l.CloseAll()

	log.Lvl4("Hosts are:", hosts[0].Address(), hosts[1].Address())
	if hosts[0].Address() == hosts[1].Address() {
		t.Fatal("Both addresses are equal")
	}
}

func TestNewTCPTest(t *testing.T) {
	l := NewTCPTest()
	_, el, _ := l.GenTree(3, true)
	defer l.CloseAll()

	c1 := NewClient(clientServiceName)
	_, cerr := c1.Send(el.List[0], "SimpleMessage", nil)
	log.ErrFatal(cerr)
	_, cerr = c1.Send(el.List[1], "SimpleMessage", nil)
	log.ErrFatal(cerr)
	_, cerr = c1.Send(el.List[2], "SimpleMessage", nil)
	log.ErrFatal(cerr)
}

type clientService struct {
	*ServiceProcessor
}

func (c *clientService) SimpleMessage(msg *SimpleMessage) (network.Body, ClientError) {
	log.Lvl3("Got request", msg)
	return nil, nil
}

func newClientService(c *Context, path string) Service {
	s := &clientService{
		ServiceProcessor: NewServiceProcessor(c),
	}
	log.ErrFatal(s.RegisterMessage(s.SimpleMessage))
	return s
}
