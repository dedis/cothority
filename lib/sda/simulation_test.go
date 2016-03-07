package sda_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/testutil"
)

func TestSimulationBF(t *testing.T) {
	defer testutil.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	addresses := []string{
		"local1:2000", "local2:2000",
		"local1:2001", "local2:2001",
		"local1:2002", "local2:2002",
		"local1:2003",
	}
	for i, a := range sc.EntityList.List {
		if a.Addresses[0] != addresses[i] {
			t.Fatal("Address", a.Addresses[0], "should be", addresses[i])
		}
	}
	if !sc.Tree.IsBinary(sc.Tree.Root) {
		t.Fatal("Created tree is not binary")
	}

	sc, _, err = createBFTree(13, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Tree.Root.Children) != 3 {
		t.Fatal("Branching-factor 3 tree has not 3 children")
	}
	if !sc.Tree.IsNary(sc.Tree.Root, 3) {
		t.Fatal("Created tree is not binary")
	}
}

func TestBigTree(t *testing.T) {
	defer testutil.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	for i := uint(12); i < 15; i++ {
		_, _, err := createBFTree(1<<i-1, 2)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadSave(t *testing.T) {
	defer testutil.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	sc.Save("/tmp")
	sc2, err := sda.LoadSimulationConfig("/tmp", "local1:2000")
	if err != nil {
		t.Fatal(err)
	}
	if sc2[0].Tree.Id != sc.Tree.Id {
		t.Fatal("Tree-id is not correct")
	}
}

func TestMultipleInstances(t *testing.T) {
	defer testutil.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	sc.Save("/tmp")
	sc2, err := sda.LoadSimulationConfig("/tmp", "local1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sc2) != 4 {
		t.Fatal("We should have 4 local1-hosts but have", len(sc2))
	}
	if sc2[0].Host.Entity.Id == sc2[1].Host.Entity.Id {
		t.Fatal("Hosts are not copies")
	}
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
		simul := sda.SimulationBFTree{
			BF:    s.BF,
			Depth: s.depth,
		}
		simul.CreateEntityList(&sda.SimulationConfig{}, []string{"localhost"}, 2000)
		if simul.Hosts != s.hosts {
			t.Fatal(s, "gave", simul.Hosts)
		}
	}
}

func createBFTree(hosts, bf int) (*sda.SimulationConfig, *sda.SimulationBFTree, error) {
	sc := &sda.SimulationConfig{}
	sb := &sda.SimulationBFTree{
		Hosts: hosts,
		BF:    bf,
	}
	sb.CreateEntityList(sc, []string{"local1", "local2"}, 2000)
	if len(sc.EntityList.List) != hosts {
		return nil, nil, errors.New("Didn't get correct number of entities")
	}
	err := sb.CreateTree(sc)
	if err != nil {
		return nil, nil, err
	}
	if !sc.Tree.IsNary(sc.Tree.Root, bf) {
		return nil, nil, errors.New("Tree isn't " + strconv.Itoa(bf) + "-ary")
	}

	return sc, sb, nil
}
