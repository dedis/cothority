package sda_test

import (
	"errors"
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

func TestSimulationBF(t *testing.T) {
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
	/*
		sb.BF = 3
		sb.CreateTree(sc)
		if len(sc.Tree.Root.Children) != 3 {
			t.Fatal("Branching-factor 3 tree has not 3 children")
		}
	*/
}

func TestLoadSave(t *testing.T) {
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	sc.Save("/tmp")
	sc2, err := sda.LoadSimulationConfig("/tmp", "local1:2000")
	if err != nil {
		t.Fatal(err)
	}
	if sc2.Tree.Id != sc.Tree.Id {
		t.Fatal("Tree-id is not correct")
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
	if !sc.Tree.IsBinary(sc.Tree.Root) {
		return nil, nil, errors.New("Tree isn't binary")
	}

	return sc, sb, nil
}
