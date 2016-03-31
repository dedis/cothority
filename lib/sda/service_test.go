package sda_test

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

type DummyProtocol struct {
	*sda.Node
	start chan bool
}

func (dp *DummyProtocol) Start() error {
	dp.start <- true
	return nil
}

type DummyService struct {
	*sda.Host
	start chan bool
}

func (ds *DummyService) InstantiateProtocol(n *sda.Node) (sda.ProtocolInstance, error) {
	return &DummyProtocol{
		Node:  n,
		start: ds.start,
	}, nil
}

func (ds *DummyService) ProcessRequest(n *network.Entity, req *sda.Request) {
	ds.start <- true
}

func TestServiceFactory(t *testing.T) {
	ds := &DummyService{
		start: make(chan bool),
	}
	sda.ServiceFactory.RegisterByName("dummy", func(h *sda.Host, path string) sda.Service {
		ds.Host = h
		ds.start <- true
		return ds
	})

	var host *sda.Host
	go func() { host = sda.NewLocalHost(2000) }()
	select {
	case <-ds.start:
		break
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Could not create dummy service")
	}
}

func TestServiceDispatch(t *testing.T) {
	ds := &DummyService{
		start: make(chan bool),
	}
	sda.ServiceFactory.RegisterByName("dummy", func(h *sda.Host, path string) sda.Service {
		ds.Host = h
		return ds
	})

	host := sda.NewLocalHost(2000)
	host.Listen()
	defer host.Close()
	host.StartProcessMessages()
	host2 := sda.NewLocalHost(2001)
	defer host2.Close()
	if _, err := host2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}

	request := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("dummy"),
		Type:    "DummyRequest",
	}
	if err := host2.SendRaw(host.Entity, request); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ds.start:
		break
	case <-time.After(time.Millisecond * 100):
		t.Fatal("DummyService did not receive message")
	}
}
