package main

import (
	"strconv"
	"testing"

	"github.com/dedis/cothority/simul/platform"
)

func TestBuild(t *testing.T) {
}

func TestDepth(t *testing.T) {
	testStruct := []struct{ BF, depth, hosts int }{
		{2, 1, 3},
		{3, 1, 4},
		{3, 2, 13},
		{4, 1, 5},
		{4, 2, 21},
		{5, 1, 6},
		{5, 2, 31},
		{5, 3, 156},
	}
	for _, s := range testStruct {
		rc := *platform.NewRunConfig()
		rc.Put("bf", strconv.Itoa(s.BF))
		rc.Put("depth", strconv.Itoa(s.depth))
		CheckHosts(rc)
		hosts, err := rc.GetInt("hosts")
		if err != nil {
			t.Fatal("Couldn't get hosts:", err)
		}
		if hosts != s.hosts {
			t.Fatal(s, "gave", hosts)
		}
	}
}
