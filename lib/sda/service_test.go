package sda

import (
	"github.com/dedis/cothority/lib/network"
	"testing"
	"time"
)

type DummyProtocol struct {
	*Node
	start chan bool
}

func (dp *DummyProtocol) Start() error {
	dp.start <- true
	return nil
}

type DummyService struct {
	*Host
	start chan bool
}

func (ds *DummyService) InstantiateProtocol(n *Node) (ProtocolInstance, error) {
	return &DummyProtocol{
		Node:  n,
		start: ds.start,
	}, nil
}

func (ds *DummyService) ProcessRequest(n *network.Entity, r *Request) {
	return
}

func TestServiceS(t *testing.T) {
	ds := &DummyService{
		start: make(chan bool),
	}
	ServiceFactory.RegisterByName("dummy", func(h *Host) Service {
		ds.Host = h
		h.SubscribeToRequest("dummy", ds)
		ds.start <- true
		return ds
	})

	done := make(chan bool)

	var host *Host
	go func() { host = NewLocalHost(2000) }()
	select {
	case <-ds.start:
		break
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Could not create dummy service")
	}

}
