package service

import (
	"testing"

	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_Clock(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, _, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)

	for _, s := range services {
		log.Lvl2("Sending request to", s)
	}
}

func TestService_Count(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, _, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)

	for _, s := range services {
		log.Lvl2("Sending request to", s)
	}
}
