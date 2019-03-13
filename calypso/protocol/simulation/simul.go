package main

import (
	// Service needs to be imported here to be instantiated.
	_ "go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul"
)

func main() {
	_, err := onet.RegisterNewService(ocsServiceName, newService)
	log.ErrFatal(err)

	simul.Start()
}
