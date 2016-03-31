package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
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
	ds.start <- true
	return &DummyProtocol{
		Node:  n,
		start: ds.start,
	}, nil
}

func (ds *DummyService) ProcessRequest(n *network.Entity, req *sda.Request) {
	ds.start <- true
}

func TestServiceFactory(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		start: make(chan bool),
	}
	sda.ServiceFactory.RegisterByName("dummy", func(h *sda.Host, path string) sda.Service {
		ds.Host = h
		ds.start <- true
		return ds
	})

	go func() { sda.NewLocalHost(2000) }()
	select {
	case <-ds.start:
		break
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Could not create dummy service")
	}
}

func TestServiceDispatch(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
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

func TestServiceInstantiateProtocol(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	// setup
	ds := &DummyService{
		start: make(chan bool),
	}
	sda.ServiceFactory.RegisterByName("dummy", func(h *sda.Host, path string) sda.Service {
		ds.Host = h
		return ds
	})

	h1 := sda.NewLocalHost(2000)
	defer h1.Close()
	el := sda.NewEntityList([]*network.Entity{h1.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	done := make(chan bool)
	go func() {
		_, err := h1.StartNewNodeName("dummy", tree)
		if err != nil {
			t.Fatal("error starting new node", err)
		}
		done <- true
	}()

	// wait for the call of InstantiateProtocol
	waitOrFatal(ds.start, t)
	// wiat for the ProtocolInstance to start
	waitOrFatal(ds.start, t)
	// wait the end
	waitOrFatal(done, t)
}

func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case <-ch:
		return
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Wait too long")
	}
}
