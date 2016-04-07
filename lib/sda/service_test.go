package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

type DummyProtocol struct {
	*sda.TreeNodeInstance
	link   chan bool
	config DummyConfig
}

type DummyConfig struct {
	A int
}

func NewDummyProtocol(tni *sda.TreeNodeInstance, conf DummyConfig, link chan bool) *DummyProtocol {
	return &DummyProtocol{tni, link, conf}
}

func (dm *DummyProtocol) ProcessMessage(msg *sda.Data) {
	dm.link <- true
}

// legcy reasons
func (dm *DummyProtocol) Dispatch() error {
	return nil
}

type DummyService struct {
	c        *sda.Context
	path     string
	link     chan bool
	fakeTree *sda.Tree
}

func (ds *DummyService) ProcessRequest(e *network.Entity, r *sda.Request) {
	if r.Type != "NewDummy" {
		ds.link <- false
		return
	}
	tni := ds.c.NewTreeNodeInstance(ds.fakeTree.Root)
	dp := NewDummyProtocol(tni, DummyConfig{}, ds.link)
	ds.c.RegisterProtocolInstance(dp)
}

func (ds *DummyService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dummyConf := conf.Data.(DummyConfig)
	dummyConf.A++
	dp := NewDummyProtocol(tn, dummyConf, ds.link)
	return dp, nil
}

func TestServiceNew(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(h *sda.Host, c *sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	go func() {
		h := sda.NewLocalHost(2000)
		h.Close()
	}()

	waitOrFatal(ds.link, t)
}

func TestServiceProcessRequest(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(h *sda.Host, c *sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	hostCh := make(chan *sda.Host)
	go func() {
		h := sda.NewLocalHost(2000)
		h.Listen()
		h.StartProcessMessages()
		dbg.Lvl1("Host created and listening")
		hostCh <- h
	}()

	// wait the creation
	waitOrFatal(ds.link, t)
	// get the host
	host := <-hostCh
	defer host.Close()
	// Send a request to the service
	re := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Type:    "wrongType",
	}
	// fake a client
	dbg.Print("Before Client")
	// have to listen on the service link also
	go func() {
		<-ds.link
	}()
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Print("After Client")
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link
	select {
	case v := <-ds.link:
		if v {
			t.Fatal("was expecting false !")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Too late")
	}
}

func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case _ = <-ch:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Waited too long")
	}
}
