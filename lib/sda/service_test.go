package sda_test

import (
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
	// Optional if you only create one ProtocolInstance in this function
	// ds.c.RegisterProtocolInstance(dp)
	return dp, nil
}

func TestServiceNew(t *testing.T) {
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(h *sda.Host, c *sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	go sda.NewLocalHost(2000)
	waitOrFatal(ds.link, t)
}

func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case <-ch:
		return
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Waited too long")
	}
}
