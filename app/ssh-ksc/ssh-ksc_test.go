package main

import (
	"flag"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"github.com/dedis/crypto/config"
	"os"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	dbg.TestOutput(testing.Verbose(), 4)
	code := m.Run()
	dbg.AfterTest(nil)
	os.Exit(code)
}

func TestServerAdd(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(servers)
	srv1, srv2 := servers[0], servers[1]
	dbg.Lvl1("Adding 1st server")
	ServerAdd(ca, "localhost:2000")
	if len(srv1.Config.Servers) != 1 {
		t.Fatal("Server 1 should still only know himself")
	}
	if len(srv2.Config.Servers) != 1 {
		t.Fatal("Server 2 should still only know himself")
	}
	dbg.Lvl1("Adding 2nd server")
	ServerAdd(ca, "localhost:2001")
	if len(srv1.Config.Servers) != 2 {
		t.Fatal("Server 1 should have two servers stored")
	}
	if len(srv2.Config.Servers) != 2 {
		t.Fatal("Server 2 should have two servers stored")
	}
}

func TestServerDel(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(servers)
	srv1, srv2 := servers[0], servers[1]
	dbg.Lvl1("Adding 1st server")
	ServerAdd(ca, "localhost:2000")
	ServerAdd(ca, "localhost:2001")
	if len(srv1.Config.Servers) != 2 {
		t.Fatal("Server 1 should have two servers stored")
	}
	if len(srv2.Config.Servers) != 2 {
		t.Fatal("Server 2 should have two servers stored")
	}
	if len(ca.Config.Servers) != 2 {
		t.Fatal("ClientApp should have two servers stored")
	}
	ServerDel(ca, "localhost:2001")
	if len(srv1.Config.Servers) != 1 {
		t.Fatal("Server 1 should have only one server stored")
	}
	if len(ca.Config.Servers) != 1 {
		t.Fatal("ClientApp should have one server stored")
	}
	ServerDel(ca, "localhost:2000")
	if len(ca.Config.Servers) != 0 {
		t.Fatal("ClientApp should have no server stored")
	}
	if len(ca.Config.Servers) != 0 {
		t.Fatal("ClientApp should have no servers stored")
	}
}

func TestServerCheck(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(servers)
	addr1, addr2 := servers[0].This.Entity.Addresses[0],
		servers[1].This.Entity.Addresses[0]
	dbg.Lvl1("Adding 1st server")
	ServerAdd(ca, addr1)
	ServerAdd(ca, addr2)
	err := ServerCheck(ca)
	dbg.ErrFatal(err)
	dbg.Lvl2(ca.Config.Servers)
	ServerDel(ca, addr1)
	dbg.Lvl2(ca.Config.Servers)
	err = ServerCheck(ca)
	dbg.ErrFatal(err)
	ServerDel(ca, addr2)
	err = ServerCheck(ca)
	if err == nil {
		t.Fatal("Now the server-list should be empty")
	}
}

func closeServers(srvs []*ssh_ks.ServerApp) {
	for _, s := range srvs {
		s.Stop()
	}
}

func newTest(nbr int) (*ssh_ks.ClientApp, []*ssh_ks.ServerApp) {
	tmp, err := ssh_ks.SetupTmpHosts()
	dbg.ErrFatal(err)
	ca, err := ssh_ks.ReadClientApp(tmp + "/config.bin")
	dbg.ErrFatal(err)
	servers := make([]*ssh_ks.ServerApp, nbr)
	for i := range servers {
		servers[i] = newServerLocal(2000 + i)
		servers[i].Start()
	}
	return ca, servers
}

func newServerLocal(port int) *ssh_ks.ServerApp {
	key := config.NewKeyPair(network.Suite)
	return ssh_ks.NewServerApp(key, "localhost:"+strconv.Itoa(port), "Phony SSH public key")
}
