package service

import (
	"testing"

	"github.com/dedis/cothority/randhound"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestRandHoundService(t *testing.T) {
	local := onet.NewTCPTest()
	num := 10
	groups := 2
	purpose := "RandHound service test run"
	nodes, roster, _ := local.GenTree(num, true)
	defer local.CloseAll()

	setupRequest := &randhound.SetupRequest{roster, groups, purpose}
	service := local.GetServices(nodes, randhoundService)[0].(*Service)

	_, err := service.Setup(setupRequest)
	log.ErrFatal(err, "Service - setup failed")

	randRequest := &randhound.RandRequest{}
	reply, err := service.Random(randRequest)
	log.ErrFatal(err, "Service - request failed")

	log.Lvl1("Service - randomness:", reply.R)
	log.Lvl1("Service - transcript:", reply.T)
}
