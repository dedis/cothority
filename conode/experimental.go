package main

/*
This imports the experimental services that will not be used in the
stable branch.
*/

import (
	_ "github.com/dedis/cothority/calypso"
	_ "github.com/dedis/cothority/eventlog"
	_ "github.com/dedis/cothority/evoting/service"
	_ "github.com/dedis/cothority/identity"
	_ "github.com/dedis/cothority/ocs/service"
	_ "github.com/dedis/cothority/omniledger/contracts"
	_ "github.com/dedis/cothority/omniledger/service"
)
