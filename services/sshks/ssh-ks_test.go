package sshks_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/sshks"
	"github.com/dedis/crypto/config"
)

func TestAbstract(t *testing.T) {
	s := network.Suite
	a := s.Secret().One()
	b := s.Secret().Set(a)
	c := a
	dbg.Lvl2(a, b, c)
	//d := s.Secret().Add(a, b)
	d := a.Add(a, b)
	dbg.Lvl2(a, b, c, d)
}

func TestConfigHash(t *testing.T) {
	srvApps := createServerKSs(2)
	c := srvApps[0]
	c.AddServer(srvApps[1].This)
	h1 := c.Config.Hash()
	h2 := c.Config.Hash()
	c.DelServer(srvApps[1].This)
	h3 := c.Config.Hash()
	if bytes.Compare(h1, h2) != 0 {
		t.Fatal("1st and 2nd hash should be the same")
	}
	if bytes.Compare(h2, h3) == 0 {
		t.Fatal("2nd and 3rd hash should be different")
	}
}

func TestReadConfig(t *testing.T) {
	tmp, err := sshks.SetupTmpHosts()
	dbg.TestFatal(t, err)
	conf, err := sshks.ReadConfig(tmp + "/config.bin")
	dbg.TestFatal(t, err)
	// Take a non-existent directory
	conf, err = sshks.ReadConfig(tmp + "1")
	dbg.TestFatal(t, err)
	if len(conf.Clients) > 0 {
		t.Fatal("This should be empty")
	}
}

func TestWriteConfig(t *testing.T) {
	tmp, _ := sshks.SetupTmpHosts()
	file := tmp + "/config.bin"
	conf1 := sshks.NewConfig(10)
	err := conf1.WriteConfig(file)
	if err != nil {
		t.Fatal(err)
	}

	conf2, err := sshks.ReadConfig(file)
	dbg.TestFatal(t, err)
	if conf1.Version != conf2.Version {
		t.Fatal("Didn't find same version")
	}
}

func TestNetworkGetServer(t *testing.T) {
	conode := startServers(1)[0]
	defer conode.Stop()
	srv, err := sshks.NetworkGetServer(conode.This.Entity.Addresses[0])
	dbg.TestFatal(t, err)
	if !srv.Entity.Equal(conode.This.Entity) {
		t.Fatal("Didn't get the same Entity")
	}
}

func TestNetworkGetConfig(t *testing.T) {
	conode := startServers(1)[0]
	defer conode.Stop()
	srv, _ := sshks.NetworkGetServer(conode.This.Entity.Addresses[0])
	cl := sshks.NewClientKS("")
	conf, _, err := cl.NetworkGetConfig(srv)
	dbg.TestFatal(t, err)
	if len(conf.Servers) != 1 {
		t.Fatal("There should be exactly 1 server in the config")
	}
}

func TestCAServerAdd(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0], servers[1]
	dbg.Lvl1("Adding 1st server")
	ca.AddServer(srv1.This)
	if len(srv1.Config.Servers) != 1 {
		t.Fatal("Server 1 should still only know himself")
	}
	if len(srv2.Config.Servers) != 1 {
		t.Fatal("Server 2 should still only know himself")
	}
	if srv1.This.Entity.Public != srv1.Config.Servers["localhost:2000"].Entity.Public {
		t.Fatal("Server.This should be the same as config")
	}
	dbg.Lvl1("Adding 2nd server")
	ca.AddServer(srv2.This)
	if len(srv1.Config.Servers) != 2 {
		t.Fatal("Server 1 should have two servers stored")
	}
	if len(srv2.Config.Servers) != 2 {
		t.Fatal("Server 2 should have two servers stored")
	}
}

func TestCAServerDel(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0], servers[1]
	dbg.Lvl1("Adding 1st server")
	ca.AddServer(srv1.This)
	ca.AddServer(srv2.This)
	if len(srv1.Config.Servers) != 2 {
		t.Fatal("Server 1 should have two servers stored")
	}
	if len(srv2.Config.Servers) != 2 {
		t.Fatal("Server 2 should have two servers stored")
	}
	if len(ca.Config.Servers) != 2 {
		t.Fatal("ClientKS should have two servers stored")
	}
	ca.DelServer(srv2.This)
	if len(srv1.Config.Servers) != 1 {
		t.Fatal("Server 1 should have only one server stored")
	}
	if len(ca.Config.Servers) != 1 {
		t.Fatal("ClientKS should have one server stored")
	}
	ca.DelServer(srv1.This)
	if len(ca.Config.Servers) != 0 {
		t.Fatal("ClientKS should have no server stored")
	}
	if len(ca.Config.Servers) != 0 {
		t.Fatal("ClientKS should have no servers stored")
	}
}

func TestCAServerCheck(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0].This, servers[1].This
	dbg.Lvl1("Adding 1st server")
	ca.AddServer(srv1)
	ca.AddServer(srv2)
	err := ca.ServerCheck()
	dbg.ErrFatal(err)
	dbg.Lvl2(ca.Config.Servers)
	ca.DelServer(srv1)
	dbg.Lvl2(ca.Config.Servers)
	err = ca.ServerCheck()
	dbg.ErrFatal(err)
	ca.DelServer(srv2)
	err = ca.ServerCheck()
	if err == nil {
		t.Fatal("Now the server-list should be empty")
	}
}

func TestCAClientAdd(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	ca.AddServer(servers[0].This)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	client2 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client2")
	err := ca.AddClient(client1)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 1 {
		t.Fatal("There should be 1 client now")
	}
	err = ca.AddClient(client2)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 2 {
		t.Fatal("There should be 2 clients now")
	}
}

func TestWriteConfig2(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	ca.AddServer(servers[0].This)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	client2 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client2")
	err := ca.AddClient(client1)
	dbg.ErrFatal(err)
	err = ca.AddClient(client2)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 2 {
		t.Fatal("There should be 2 clients now")
	}

	tmp, _ := sshks.SetupTmpHosts()
	file := tmp + "/config.bin"
	conf1 := sshks.NewConfig(10)
	err = conf1.WriteConfig(file)
	if err != nil {
		t.Fatal(err)
	}

	conf2, err := sshks.ReadConfig(file)
	dbg.TestFatal(t, err)
	if conf1.Version != conf2.Version {
		t.Fatal("Didn't find same version")
	}
	if len(conf1.Clients) != len(conf2.Clients) {
		t.Fatal("Number of clients should be the same")
	}
}

func TestCAClientDel(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	ca.AddServer(servers[0].This)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	client2 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client2")
	err := ca.AddClient(client1)
	dbg.ErrFatal(err)
	err = ca.AddClient(client2)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 2 {
		t.Fatal("There should be 2 clients now")
	}
	err = ca.DelClient(client2)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 1 {
		t.Fatal("There should be 1 client now")
	}
	err = ca.DelClient(client1)
	dbg.ErrFatal(err)
	if len(ca.Config.Clients) != 0 {
		t.Fatal("There should be no client now")
	}
}

func TestCASign(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	ca.AddServer(servers[0].This)
	ca.AddServer(servers[1].This)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	err := ca.AddClient(client1)
	dbg.ErrFatal(err)
	version := ca.Config.Version
	sign1 := ca.Config.Signature
	if ca.Config.Version != version+1 {
		t.Fatal("Version didn't increase while signing")
	}
	sign2 := ca.Config.Signature
	if sign1.Challenge == sign2.Challenge {
		t.Fatal("Challenges should be different")
	}
	if sign1.Response == sign2.Response {
		t.Fatal("Responses should be different")
	}
	if bytes.Compare(sign1.Sum, sign2.Sum) == 0 {
		t.Fatal("Sums should be different")
	}
}

func TestCAUpdate(t *testing.T) {
	ca1, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0].This, servers[1].This
	ca1.AddServer(srv1)
	ca1.AddServer(srv2)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	tmp, err := sshks.SetupTmpHosts()
	dbg.ErrFatal(err)
	ca2, err := sshks.ReadClientKS(tmp + "/config.bin")
	dbg.ErrFatal(err)
	ca2.AddServer(srv1)
	// Now add a client to ca1, thus making ca2s config invalid
	err = ca1.AddClient(client1)
	dbg.ErrFatal(err)
	if bytes.Compare(ca1.Config.Signature.Sum, ca2.Config.Signature.Sum) == 0 {
		t.Fatal("Should have different signature now")
	}

	// Update and verify everything is the same
	ca2.Update(nil)
	if len(ca2.Config.Servers) != 2 {
		t.Fatal("Should now have two servers")
	}
	if len(ca2.Config.Clients) != 1 {
		t.Fatal("Should now have one client")
	}
	if bytes.Compare(ca1.Config.Signature.Sum, ca2.Config.Signature.Sum) != 0 {
		t.Fatal("Should have the same signature")
	}

	for i := 0; i < 10; i++ {
		dbg.Lvl3("Round", i)
		dbg.ErrFatal(ca1.DelServer(srv1))
		dbg.ErrFatal(ca2.Update(nil))
		if len(ca2.Config.Servers) == 2 {
			t.Fatal("There should be only 1 server left")
		}
		dbg.ErrFatal(ca1.AddServer(srv1))
		dbg.ErrFatal(ca2.Update(nil))
	}
}

func TestCreateBogusSSH(t *testing.T) {
	tmp, err := ioutil.TempDir("", "makeSSH")
	dbg.ErrFatal(err)
	err = sshks.CreateBogusSSH(tmp, "test")
	dbg.ErrFatal(err)
	_, err = os.Stat(tmp + "/test")
	if os.IsNotExist(err) {
		t.Fatal("Didn't create private key")
	}
	_, err = os.Stat(tmp + "/test.pub")
	if os.IsNotExist(err) {
		t.Fatal("Didn't create public key")
	}
}
