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
	conode.Config = sshks.NewConfig(1)
	conode.Config.Servers[srv.Entity.Addresses[0]] = srv
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
	if !srv1.This.Entity.Public.Equal(srv1.Config.Servers[srv1.This.Id()].Entity.Public) {
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
	dbg.ErrFatal(ca.AddServer(srv1))
	dbg.ErrFatal(ca.AddServer(srv2))
	dbg.ErrFatal(ca.ServerCheck())
	dbg.Lvl2(ca.Config.Servers)

	dbg.ErrFatal(ca.DelServer(srv1))
	dbg.Lvl2(ca.Config.Servers)

	dbg.ErrFatal(ca.ServerCheck())

	dbg.ErrFatal(ca.DelServer(srv2))
	err := ca.ServerCheck()
	if err == nil {
		t.Fatal("Now the server-list should be empty")
	}
}

func TestCAClientAdd(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	dbg.ErrFatal(ca.AddServer(servers[0].This))
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	dbg.ErrFatal(ca.AddClient(client1))
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
	ca2 := newClient(1)
	dbg.ErrFatal(ca.AddClient(ca2.This))
	if len(ca.Config.Clients) != 2 {
		t.Fatal("There should be 2 clients now")
	}
	dbg.ErrFatal(ca2.AddServer(servers[0].This))

	dbg.ErrFatal(ca.DelClient(ca2.This))
	dbg.ErrFatal(ca2.Update(nil))
	dbg.ErrFatal(ca2.ConfirmNewConfig(nil))
	dbg.ErrFatal(ca.Update(nil))
	if len(ca.Config.Clients) != 1 {
		t.Fatal("There should be 1 client now")
	}
}

func TestCASign(t *testing.T) {
	ca, servers := newTest(2)
	defer closeServers(t, servers)
	ca.AddServer(servers[0].This)
	ca.AddServer(servers[1].This)
	client1 := sshks.NewClient(config.NewKeyPair(network.Suite).Public,
		"Client1")
	conf1, err := ca.Config.Copy()
	dbg.ErrFatal(err)
	dbg.ErrFatal(ca.AddClient(client1))

	if ca.Config.Version != conf1.Version+1 {
		t.Fatal("Version didn't increase while signing",
			ca.Config.Version, conf1.Version)
	}

	sign1 := conf1.Signature
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
	ca1, servers := newTest(1)
	defer closeServers(t, servers)
	srv1 := servers[0].This
	dbg.ErrFatal(ca1.AddServer(srv1))
	ca2 := newClient(2)
	dbg.ErrFatal(ca1.AddClient(ca2.This))
	dbg.ErrFatal(ca2.AddServer(srv1))

	// Now add a client to ca1, thus making ca2s config invalid
	if ca2.Config.Signature == nil {
		t.Fatal("Signature of ca2 is nil")
	}
	if len(ca2.Config.Servers) != 1 {
		t.Fatal("Should have one server")
	}
	if len(ca2.Config.Clients) != 2 {
		t.Fatal("Should have two clients")
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
