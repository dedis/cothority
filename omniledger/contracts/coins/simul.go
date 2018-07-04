package main

import (
	// Service needs to be imported here to be instantiated.
	_ "github.com/dedis/cothority/omniledger/service"
	_ "github.com/dedis/cothority/omniledger/contracts"
	"github.com/dedis/onet/simul"
)

func main() {
	simul.Start()
}
