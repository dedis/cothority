package main

/*
This imports the experimental services that will not be used in the
stable branch.
*/

import (
	_ "go.dedis.ch/cothority/v3/authprox"
	_ "go.dedis.ch/cothority/v3/byzcoin"
	_ "go.dedis.ch/cothority/v3/byzcoin/contracts"
	_ "go.dedis.ch/cothority/v3/calypso"
	_ "go.dedis.ch/cothority/v3/eventlog"
	_ "go.dedis.ch/cothority/v3/evoting/service"
	_ "go.dedis.ch/cothority/v3/personhood"
)
