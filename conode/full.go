//go:build test
// +build test

package main

import (
	_ "go.dedis.ch/cothority/v3/authprox"
	// you don't want to include this service in the docker container, as it
	// expects a database and will try to connect to it
	// _ "go.dedis.ch/cothority/v3/bypros"
	_ "go.dedis.ch/cothority/v3/byzcoin"
	_ "go.dedis.ch/cothority/v3/byzcoin/contracts"
	_ "go.dedis.ch/cothority/v3/calypso"
	_ "go.dedis.ch/cothority/v3/eventlog"
	_ "go.dedis.ch/cothority/v3/personhood"
)
