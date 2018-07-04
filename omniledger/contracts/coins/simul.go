package main

import (
	// Service needs to be imported here to be instantiated.
	_ "github.com/dedis/cothority/omniledger/contracts"
	_ "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/simul"
)

func main() {
	simul.Start()
}
