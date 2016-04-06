package ssh_ks_test

import (
	"bytes"
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

func TestServerCreation(t *testing.T) {
	conodes := createCoNodes(2)
	for i, s := range conodes {
		if s.This.Entity.Addresses[0] != "localhost:"+strconv.Itoa(2000+i) {
			t.Fatal("Couldn't verify server", i, s)
		}
	}
}

func TestServerAdd(t *testing.T) {
	conodes := createCoNodes(2)
	conodes[0].AddServer(conodes[1].This)
	addr1 := conodes[1].This.Entity.Addresses[0]
	_, ok := conodes[0].Config.Servers[addr1]
	if !ok {
		t.Fatal("Didn't find server 1 in server 0")
	}
	conodes[0].DelServer(conodes[1].This)
	_, ok = conodes[0].Config.Servers[addr1]
	if ok {
		t.Fatal("Shouldn't find server 1 in server 0")
	}
}

func TestServerStart(t *testing.T) {
	conodes := createCoNodes(2)
	conodes[0].AddServer(conodes[1].This)
	err := conodes[0].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer conodes[0].Stop()
	err = conodes[1].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer conodes[1].Stop()

	err = conodes[0].Check()
	dbg.TestFatal(t, err, "Couldn't check servers:")
}

func TestConfigEntityList(t *testing.T) {
	conodes := createCoNodes(2)
	c := conodes[0]
	c.AddServer(conodes[1].This)
	el := c.Config.EntityList(c.This.Entity)
	if len(el.List) == 0 {
		t.Fatal("List shouldn't be of length 0")
	}
	if el.List[0] != c.This.Entity {
		t.Fatal("First element should be server 0")
	}
	el = c.Config.EntityList(conodes[1].This.Entity)
	if el.List[0] != conodes[1].This.Entity {
		t.Fatal("First element should be server 1")
	}
}

func TestServerSign(t *testing.T) {
	conodes := startServers(2)
	defer closeServers(t, conodes)
	c := conodes[0]
	err := c.Sign()
	dbg.TestFatal(t, err, "Couldn't sign config")
	if c.Config.Version != 1 {
		t.Fatal("Version should now be 1")
	}
	if c.Config.Signature == nil {
		t.Fatal("Signature should not be nil")
	}
	err = c.Config.VerifySignature()
	dbg.TestFatal(t, err, "Signature verification failed:")

	// Change the version and look if it fails
	c.Config.Version += 1
	err = c.Config.VerifySignature()
	if err == nil {
		t.Fatal("Signature verification should fail now")
	} else {
		dbg.Lvl2("Expected error from comparison:", err)
	}

	// Change the response and look if it fails
	c.Config.Version -= 1
	su := network.Suite
	c.Config.Signature.Response.Add(c.Config.Signature.Response, su.Secret().One())
	err = c.Config.VerifySignature()
	if err == nil {
		t.Fatal("Signature verification should fail now")
	} else {
		dbg.Lvl2("Expected error from comparison:", err)
	}
}

func TestAbstract(t *testing.T) {
	s := network.Suite
	a := s.Secret().One()
	b := s.Secret().Set(a)
	c := a
	dbg.Print(a, b, c)
	//d := s.Secret().Add(a, b)
	d := a.Add(a, b)
	dbg.Print(a, b, c, d)
}

func TestConfigHash(t *testing.T) {
	conodes := createCoNodes(2)
	c := conodes[0]
	c.AddServer(conodes[1].This)
	h1 := c.Config.Hash()
	h2 := c.Config.Hash()
	c.DelServer(conodes[1].This)
	h3 := c.Config.Hash()
	if bytes.Compare(h1, h2) != 0 {
		t.Fatal("1st and 2nd hash should be the same")
	}
	if bytes.Compare(h2, h3) == 0 {
		t.Fatal("2nd and 3rd hash should be different")
	}
}

func TestReadConfig(t *testing.T) {
	tmp, err := ssh_ks.SetupTmpHosts()
	dbg.TestFatal(t, err)
	conf, err := ssh_ks.ReadConfig(tmp + "/config.bin")
	dbg.TestFatal(t, err)
	// Take a non-existent directory
	conf, err = ssh_ks.ReadConfig(tmp + "1")
	dbg.TestFatal(t, err)
	if len(conf.Clients) > 0 {
		t.Fatal("This should be empty")
	}
}

func TestWriteConfig(t *testing.T) {
	tmp, _ := ssh_ks.SetupTmpHosts()
	file := tmp + "/config.bin"
	conf1 := ssh_ks.NewConfig(10)
	err := conf1.WriteConfig(file)
	if err != nil {
		t.Fatal(err)
	}

	conf2, err := ssh_ks.ReadConfig(file)
	dbg.TestFatal(t, err)
	if conf1.Version != conf2.Version {
		t.Fatal("Didn't find same version")
	}
}

func TestNetworkGetServer(t *testing.T) {
	conode := startServers(1)[0]
	defer conode.Stop()
	srv, err := ssh_ks.NetworkGetServer(conode.This.Entity.Addresses[0])
	dbg.TestFatal(t, err)
	if !srv.Entity.Equal(conode.This.Entity) {
		t.Fatal("Didn't get the same Entity")
	}
}

func TestNetworkGetConfig(t *testing.T) {
	conode := startServers(1)[0]
	defer conode.Stop()
	srv, _ := ssh_ks.NetworkGetServer(conode.This.Entity.Addresses[0])
	cl := ssh_ks.NewClientApp("")
	conf, err := cl.NetworkGetConfig(srv)
	dbg.TestFatal(t, err)
	if len(conf.Servers) != 1 {
		t.Fatal("There should be exactly 1 server in the config")
	}
}

func TestNetworkAddServer(t *testing.T) {
	conodes := startServers(2)
	defer closeServers(t, conodes)
	srv0, err := ssh_ks.NetworkGetServer(conodes[0].This.Entity.Addresses[0])
	dbg.TestFatal(t, err)
	srv1, err := ssh_ks.NetworkGetServer(conodes[1].This.Entity.Addresses[0])
	dbg.TestFatal(t, err)
	cl := ssh_ks.NewClientApp("")
	conf, _ := cl.NetworkGetConfig(srv0)
	cl.Config = conf
	err = cl.NetworkAddServer(srv0)
	dbg.TestFatal(t, err)
	if cl.Config == nil {
		t.Fatal("Config should now be created")
	}
	conf, _ = cl.NetworkGetConfig(srv0)
	if !conf.Servers[srv0.Entity.Addresses[0]].Entity.Public.Equal(srv0.Entity.Public) {
		t.Fatal("srv0 is not signed up")
	}
	err = cl.NetworkAddServer(srv1)
	conf, _ = cl.NetworkGetConfig(srv1)
	if !conf.Servers[srv1.Entity.Addresses[0]].Entity.Public.Equal(srv1.Entity.Public) {
		t.Fatal("srv1 is not signed up")
	}
	dbg.TestFatal(t, err)
}

func newServerLocal(port int) *ssh_ks.ServerApp {
	key := config.NewKeyPair(network.Suite)
	return ssh_ks.NewCoNode(key, "localhost:"+strconv.Itoa(port), "Phony SSH public key")
}

func closeServers(t *testing.T, servers []*ssh_ks.ServerApp) error {
	for _, s := range servers {
		err := s.Stop()
		if err != nil {
			t.Fatal("Couldn't stop server:", err)
		}
	}
	return nil
}

func startServers(nbr int) []*ssh_ks.ServerApp {
	servers := addServers(nbr)
	for _, s := range servers {
		s.Start()
	}
	return servers
}

func addServers(nbr int) []*ssh_ks.ServerApp {
	conodes := createCoNodes(nbr)
	for _, c1 := range conodes {
		for _, c2 := range conodes {
			c1.AddServer(c2.This)
		}
	}
	return conodes
}

func createCoNodes(nbr int) []*ssh_ks.ServerApp {
	ret := make([]*ssh_ks.ServerApp, nbr)
	for i := range ret {
		ret[i] = newServerLocal(2000 + i)
	}
	return ret
}
