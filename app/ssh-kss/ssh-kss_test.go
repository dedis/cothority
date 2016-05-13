package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/identity"
)

func TestCreateServerConfig(t *testing.T) {
	nbr := 2
	servers, err := createServers(nbr, t)
	if err != nil {
		t.Fatal("Server-creation:", err)
	}
	for _, server := range servers {
		err = server.WriteConfig(server.DirSSHD + "/server.conf")
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range servers {
		sc, err := identity.ReadServerKS(s.DirSSHD + "/server.conf")
		if err != nil {
			t.Fatal(err)
		}
		if sc.DirSSHD != s.DirSSHD {
			t.Fatal("Directories should be the same")
		}
		if !sc.This.Entity.Equal(s.This.Entity) {
			t.Fatal("Entities are not the same")
		}
		if !sc.Private.Equal(s.Private) {
			t.Fatal("Entities are not the same")
		}
	}

}

func TestAskServerConfig(t *testing.T) {
	tmp, err := identity.SetupTmpHosts()
	if err != nil {
		t.Fatal("Couldn't setup temp host:", err)
	}
	sc, err := askServerConfig(strings.NewReader("\n"+tmp+"\n"+tmp+"\n"), bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Couldn't create new config: ", err)
	}
	err = checkServerConfig(sc, "localhost:2000", tmp)
	if err != nil {
		t.Fatal("Didn't get correct config:", err)
	}
	sc, err = askServerConfig(strings.NewReader("localhost:2001\n"+tmp+"\n"+tmp+"\n"), bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Couldn't create new config: ", err)
	}
	err = checkServerConfig(sc, "localhost:2001", tmp)
	if err != nil {
		t.Fatal("Didn't get correct config:", err)
	}
}

func TestCreateSSHDir(t *testing.T) {
	servers, err := createServers(1, t)
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range servers {
		dbg.Lvl2(server)
	}
}

func createServers(nbr int, t *testing.T) ([]*identity.ServerKS, error) {
	ret := make([]*identity.ServerKS, nbr)
	for i := range ret {
		tmp, err := identity.SetupTmpHosts()
		if err != nil {
			t.Fatal("Couldn't setup tmp:", err)
		}
		ret[i], err = createServerConfig("localhost:"+strconv.Itoa(2000+i), tmp, tmp)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func checkServerConfig(sc *identity.ServerKS, ip, sshd string) error {
	if sc.DirSSHD != sshd {
		return errors.New(fmt.Sprintf("SSHD-dir is wrong: %s instead of %s",
			sc.DirSSHD, sshd))
	}
	if sc.This.Entity.Addresses[0] != ip {
		return errors.New(fmt.Sprintf("IP is wrong: %s instead of %s",
			sc.This.Entity.Addresses[0], ip))
	}
	return nil
}
