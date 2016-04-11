package sshks_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"strconv"
	"testing"
)

func TestServerCreation(t *testing.T) {
	srvApps := createServerKSs(2)
	for i, s := range srvApps {
		if s.This.Entity.Addresses[0] != "localhost:"+strconv.Itoa(2000+i) {
			t.Fatal("Couldn't verify server", i, s)
		}
	}
}

func TestServerAdd(t *testing.T) {
	srvApps := createServerKSs(2)
	srvApps[0].AddServer(srvApps[1].This)
	addr1 := srvApps[1].This.Entity.Addresses[0]
	_, ok := srvApps[0].Config.Servers[addr1]
	if !ok {
		t.Fatal("Didn't find server 1 in server 0")
	}
	srvApps[0].DelServer(srvApps[1].This)
	_, ok = srvApps[0].Config.Servers[addr1]
	if ok {
		t.Fatal("Shouldn't find server 1 in server 0")
	}
}

func TestServerStart(t *testing.T) {
	srvApps := createServerKSs(2)
	srvApps[0].AddServer(srvApps[1].This)
	err := srvApps[0].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer srvApps[0].Stop()
	err = srvApps[1].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer srvApps[1].Stop()

	err = srvApps[0].Check()
	dbg.TestFatal(t, err, "Couldn't check servers:")
}

func TestConfigEntityList(t *testing.T) {
	srvApps := createServerKSs(2)
	c := srvApps[0]
	c.AddServer(srvApps[1].This)
	el := c.Config.EntityList(c.This.Entity)
	if len(el.List) == 0 {
		t.Fatal("List shouldn't be of length 0")
	}
	if el.List[0] != c.This.Entity {
		t.Fatal("First element should be server 0")
	}
	el = c.Config.EntityList(srvApps[1].This.Entity)
	if el.List[0] != srvApps[1].This.Entity {
		t.Fatal("First element should be server 1")
	}
}

func TestServerSign(t *testing.T) {
	srvApps := startServers(2)
	defer closeServers(t, srvApps)
	for i := 0; i < 10; i++ {
		err := srvApps[0].Sign()
		dbg.TestFatal(t, err, "Couldn't sign config")
		for _, sa := range srvApps {
			if sa.Config.Version != i+1 {
				t.Fatal("Version should now be", i+1)
			}
			if sa.Config.Signature == nil {
				t.Fatal("Signature should not be nil")
			}
			err = sa.Config.VerifySignature()
			dbg.TestFatal(t, err, "Signature verification failed:")

			// Change the version and look if it fails
			sa.Config.Version += 1
			err = sa.Config.VerifySignature()
			if err == nil {
				t.Fatal("Signature verification should fail now")
			} else {
				dbg.Lvl2("Expected error from comparison:", err)
			}

			// Change the response and look if it fails
			sa.Config.Version -= 1
			su := network.Suite
			sa.Config.Signature.Response.Add(sa.Config.Signature.Response, su.Secret().One())
			err = sa.Config.VerifySignature()
			if err == nil {
				t.Fatal("Signature verification should fail now")
			} else {
				dbg.Lvl2("Expected error from comparison:", err)
			}
		}
	}
}
