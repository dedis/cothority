package ssh_ks_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/ssh-ks"
	"github.com/dedis/crypto/config"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func newTest(nbr int) (*ssh_ks.ClientKS, []*ssh_ks.ServerKS) {
	tmp, err := ssh_ks.SetupTmpHosts()
	dbg.ErrFatal(err)
	ca, err := ssh_ks.ReadClientKS(tmp + "/config.bin")
	dbg.ErrFatal(err)
	servers := make([]*ssh_ks.ServerKS, nbr)
	for i := range servers {
		servers[i] = newServerLocal(2000 + i)
		servers[i].Start()
	}
	return ca, servers
}

func createServerKSs(nbr int) []*ssh_ks.ServerKS {
	ret := make([]*ssh_ks.ServerKS, nbr)
	for i := range ret {
		ret[i] = newServerLocal(2000 + i)
	}
	return ret
}

func newServerLocal(port int) *ssh_ks.ServerKS {
	key := config.NewKeyPair(network.Suite)
	tmp, err := ssh_ks.SetupTmpHosts()
	dbg.ErrFatal(err)
	sa, err := ssh_ks.NewServerKS(key, "localhost:"+strconv.Itoa(port), tmp, tmp)
	dbg.ErrFatal(err)
	return sa
}

func closeServers(t *testing.T, servers []*ssh_ks.ServerKS) error {
	for _, s := range servers {
		err := s.Stop()
		if err != nil {
			t.Fatal("Couldn't stop server:", err)
		}
	}
	return nil
}

func createSrvaSeCla(nbr int) ([]*ssh_ks.ServerKS, []*ssh_ks.Server, *ssh_ks.ClientKS) {
	srvApps := createServerKSs(nbr)
	servers := make([]*ssh_ks.Server, nbr)
	for s := range srvApps {
		srvApps[s].Start()
		var err error
		servers[s] = srvApps[s].This
		dbg.ErrFatal(err)
	}
	clApp := ssh_ks.NewClientKS("")
	clApp.Config = srvApps[0].Config
	return srvApps, servers, clApp
}

func startServers(nbr int) []*ssh_ks.ServerKS {
	servers := addServers(nbr)
	for _, s := range servers {
		s.Start()
	}
	return servers
}

func addServers(nbr int) []*ssh_ks.ServerKS {
	srvApps := createServerKSs(nbr)
	for _, c1 := range srvApps {
		for _, c2 := range srvApps {
			c1.AddServer(c2.This)
		}
	}
	return srvApps
}
