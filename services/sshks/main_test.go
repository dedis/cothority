package sshks_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/sshks"
	"github.com/dedis/crypto/config"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func newTest(nbr int) (*sshks.ClientKS, []*sshks.ServerKS) {
	tmp, err := sshks.SetupTmpHosts()
	dbg.ErrFatal(err)
	ca, err := sshks.ReadClientKS(tmp + "/config.bin")
	dbg.ErrFatal(err)
	servers := make([]*sshks.ServerKS, nbr)
	for i := range servers {
		servers[i] = newServerLocal(2000 + i)
		servers[i].Start()
	}
	return ca, servers
}

func createServerKSs(nbr int) []*sshks.ServerKS {
	ret := make([]*sshks.ServerKS, nbr)
	for i := range ret {
		ret[i] = newServerLocal(2000 + i)
	}
	return ret
}

func newServerLocal(port int) *sshks.ServerKS {
	key := config.NewKeyPair(network.Suite)
	tmp, err := sshks.SetupTmpHosts()
	dbg.ErrFatal(err)
	sa, err := sshks.NewServerKS(key, "localhost:"+strconv.Itoa(port), tmp, tmp)
	dbg.ErrFatal(err)
	return sa
}

func closeServers(t *testing.T, servers []*sshks.ServerKS) error {
	for _, s := range servers {
		err := s.Stop()
		if err != nil {
			t.Fatal("Couldn't stop server:", err)
		}
	}
	return nil
}

func createSrvaSeCla(nbr int) ([]*sshks.ServerKS, []*sshks.Server, *sshks.ClientKS) {
	srvApps := createServerKSs(nbr)
	servers := make([]*sshks.Server, nbr)
	for s := range srvApps {
		srvApps[s].Start()
		var err error
		servers[s] = srvApps[s].This
		dbg.ErrFatal(err)
	}
	clApp := sshks.NewClientKS("")
	clApp.Config = srvApps[0].Config
	return srvApps, servers, clApp
}

func startServers(nbr int) []*sshks.ServerKS {
	servers := addServers(nbr)
	for _, s := range servers {
		s.Start()
	}
	return servers
}

func addServers(nbr int) []*sshks.ServerKS {
	srvApps := createServerKSs(nbr)
	for _, c1 := range srvApps {
		for _, c2 := range srvApps {
			c1.AddServer(c2.This)
		}
	}
	return srvApps
}
