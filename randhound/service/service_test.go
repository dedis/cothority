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
	purpose := "Pulsar[RandHound] - service test run"
	interval := 0
	nodes, roster, _ := local.GenTree(num, true)
	defer local.CloseAll()

	setupRequest := &randhound.SetupRequest{
		Roster:   roster,
		Groups:   groups,
		Purpose:  purpose,
		Interval: interval,
	}
	service := local.GetServices(nodes, randhoundService)[0].(*Service)

	_, err := service.Setup(setupRequest)
	log.ErrFatal(err, "Pulsar[RandHound] - service setup failed")

	randRequest := &randhound.RandRequest{}
	reply, err := service.Random(randRequest)
	log.ErrFatal(err, "Pulsar[RandHound] - service randomness request failed")

	log.Lvl1("Pulsar[RandHound] - randomness:", reply.R)
	log.Lvl1("Pulsar[RandHound] - transcript:", reply.T)
}
