package main

import (
	"bytes"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"github.com/dedis/crypto/config"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}
