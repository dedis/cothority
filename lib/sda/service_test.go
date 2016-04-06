package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

type DummyProtocol struct {
	TreeNodeInstance
	link   chan bool
	config DummyConfig
}

type DummyConfig struct {
	A int
}

func NewDummyProtocol(tni TreeNodeInstance, conf DummyConfig, link chan bool) DummyProtocol {
	return &DummyProtocol{tni, link, conf}
}

func (dm *DummyProtocol) ProcessMessage(msg *sda.Message) {
	dm.link <- true
}

type DummyService struct {
	c        sda.Context
	path     string
	link     chan bool
	fakeTree *sda.Tree
}

func (ds *DummyService) ProcessRequest(r *Request) {
	if r.Type != "NewDummy" {
		return
	}

	dp := NewDummyProtocol(ds.c, ds.fakeTree.Root, DummyConfig{}, ds.link)
	c.RegisterProtocol(dp)
}

func (ds *DummyService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dummyConf := conf.Data.(DummyConfig)
	dummyConf.A++
	dp := NewDummyProtocol(tn, dummyConf, ds.link)
	c.RegisterProtocolInstance(dp)
}

func TestServiceNew(t *testing.t) {
	ds := &DummyService{
		c:    c,
		path: path,
		link: make(chan bool)}
	sda.RegisterNewService("DummyService", func(h *sda.Host, c *sda.Context, path string) sda.Service {
		return ds
	})
	go sda.NewLocalHost(2000)
}

func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case <-ch:
		return
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Waited too long")
	}
}
