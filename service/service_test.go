package service

import (
	"testing"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateSkipchains(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)
	log.Lvl2(roster, services)
}
