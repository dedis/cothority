package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/ssh-ks"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

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
		sc, err := ReadServerConfig(s.DirSSHD + "/server.conf")
		if err != nil {
			t.Fatal(err)
		}
		if sc.DirSSHD != s.DirSSHD {
			t.Fatal("Directories should be the same")
		}
		if !sc.CoNode.This.Entity.Equal(s.CoNode.This.Entity) {
			t.Fatal("Entities are not the same")
		}
		if !sc.CoNode.Private.Equal(s.CoNode.Private) {
			t.Fatal("Entities are not the same")
		}
	}

}

func TestStartStopServer(t *testing.T) {
	servers, err := createServers(2, t)
	if err != nil {
		t.Fatal("Server-creation:", err)
	}
	for _, server := range servers {
		err := server.Start()
		if err != nil {
			t.Fatal("Couldn't start server")
		}
	}
	for _, server := range servers {
		err := server.Stop()
		if err != nil {
			t.Fatal("Couldn't stop server")
		}
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

func TestAskServerConfig(t *testing.T) {
	tmp, err := ssh_ks.SetupTmpHosts()
	if err != nil {
		t.Fatal("Couldn't setup temp host:", err)
	}
	sc, err := AskServerConfig(strings.NewReader("\n"+tmp+"\n"+tmp+"\n"), bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Couldn't create new config: ", err)
	}
	err = checkServerConfig(sc, "localhost:2000", tmp)
	if err != nil {
		t.Fatal("Didn't get correct config:", err)
	}
	sc, err = AskServerConfig(strings.NewReader("localhost:2001\n"+tmp+"\n"+tmp+"\n"), bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Couldn't create new config: ", err)
	}
	err = checkServerConfig(sc, "localhost:2001", tmp)
	if err != nil {
		t.Fatal("Didn't get correct config:", err)
	}
}

func createServers(nbr int, t *testing.T) ([]*ServerConfig, error) {
	ret := make([]*ServerConfig, nbr)
	for i := range ret {
		tmp, err := ssh_ks.SetupTmpHosts()
		if err != nil {
			t.Fatal("Couldn't setup tmp:", err)
		}
		ret[i], err = CreateServerConfig("localhost:"+strconv.Itoa(2000+i), tmp, tmp)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func checkServerConfig(sc *ServerConfig, ip, sshd string) error {
	if sc.DirSSHD != sshd {
		return errors.New(fmt.Sprintf("SSHD-dir is wrong: %s instead of %s",
			sc.DirSSHD, sshd))
	}
	if sc.CoNode.This.Entity.Addresses[0] != ip {
		return errors.New(fmt.Sprintf("IP is wrong: %s instead of %s",
			sc.CoNode.This.Entity.Addresses[0], ip))
	}
	return nil
}
