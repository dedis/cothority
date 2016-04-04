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
	servers := createServers(2)
	for i, s := range servers {
		if s.Entity.Addresses[0] != "localhost:"+strconv.Itoa(2000+i) {
			t.Fatal("Couldn't verify server", i, s)
		}
	}
}

func TestServerAdd(t *testing.T) {
	servers := createServers(2)
	servers[0].AddServer(servers[1])
	_, ok := servers[0].Config.Servers[servers[1].Entity.Addresses[0]]
	if !ok {
		t.Fatal("Didn't find server 1 in server 0")
	}
	servers[0].DelServer(servers[1])
	_, ok = servers[0].Config.Servers[servers[1].Entity.Addresses[0]]
	if ok {
		t.Fatal("Shouldn't find server 1 in server 0")
	}
}

func TestServerStart(t *testing.T) {
	servers := createServers(2)
	servers[0].AddServer(servers[1])
	err := servers[0].Start()
	if err != nil {
		t.Fatal("Couldn't start server:", err)
	}
	defer servers[0].Stop()
	err = servers[1].Start()
	if err != nil {
		t.Fatal("Couldn't start server:", err)
	}
	defer servers[1].Stop()

	err = servers[0].Check()
	if err != nil {
		t.Fatal("Couldn't check servers:", err)
	}
}

func TestConfigEntityList(t *testing.T) {
	servers := createServers(2)
	s := servers[0]
	s.AddServer(servers[1])
	el := s.Config.EntityList(s.Entity)
	if len(el.List) == 0 {
		t.Fatal("List shouldn't be of length 0")
	}
	if el.List[0] != s.Entity {
		t.Fatal("First element should be server 0")
	}
	el = s.Config.EntityList(servers[1].Entity)
	if el.List[0] != servers[1].Entity {
		t.Fatal("First element should be server 1")
	}
}

func TestServerSign(t *testing.T) {
	servers := startServers(2)
	defer closeServers(t, servers)
	s := servers[0]
	err := s.Sign()
	if err != nil {
		t.Fatal("Couldn't sign config")
	}
	if s.Config.Version != 1 {
		t.Fatal("Version should now be 1")
	}
	if s.Config.Signature == nil {
		t.Fatal("Signature should not be nil")
	}
	agg := s.Entity.Public.Add(s.Entity.Public, servers[1].Entity.Public)
	err = s.Config.VerifySignature(agg)
	if err != nil {
		t.Fatal("Signature verification failed:", err)
	}

	// Change the version and look if it fails
	s.Config.Version += 1
	err = s.Config.VerifySignature(agg)
	if err == nil {
		t.Fatal("Signature verification should fail now")
	} else {
		dbg.Lvl2("Expected error from comparison:", err)
	}

	// Change the response and look if it fails
	s.Config.Version -= 1
	su := network.Suite
	s.Config.Signature.Response.Add(s.Config.Signature.Response, su.Secret().One())
	err = s.Config.VerifySignature(agg)
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
	d := s.Secret().Add(a, b)
	dbg.Print(a, b, c, d)
}

func TestConfigHash(t *testing.T) {
	servers := createServers(2)
	s := servers[0]
	s.AddServer(servers[1])
	h1 := s.Config.Hash()
	h2 := s.Config.Hash()
	s.DelServer(servers[1])
	h3 := s.Config.Hash()
	if bytes.Compare(h1, h2) != 0 {
		t.Fatal("1st and 2nd hash should be the same")
	}
	if bytes.Compare(h2, h3) == 0 {
		t.Fatal("2nd and 3rd hash should be different")
	}
}

func newServerLocal(port int) *ssh_ks.Server {
	key := config.NewKeyPair(network.Suite)
	return ssh_ks.NewServer(key, "localhost:"+strconv.Itoa(port))
}

func closeServers(t *testing.T, servers []*ssh_ks.Server) error {
	for _, s := range servers {
		err := s.Stop()
		if err != nil {
			t.Fatal("Couldn't stop server:", err)
		}
	}
	return nil
}

func startServers(nbr int) []*ssh_ks.Server {
	servers := addServers(nbr)
	for _, s := range servers {
		s.Start()
	}
	return servers
}

func addServers(nbr int) []*ssh_ks.Server {
	servers := createServers(nbr)
	for _, s1 := range servers {
		for _, s2 := range servers {
			s1.AddServer(s2)
		}
	}
	return servers
}

func createServers(nbr int) []*ssh_ks.Server {
	ret := make([]*ssh_ks.Server, nbr)
	for i := range ret {
		ret[i] = newServerLocal(2000 + i)
	}
	return ret
}
