package main

import (
	"flag"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/ssh-ks"
	"os"
	"os/exec"
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

func TestCreateServerConfig(t *testing.T) {
	nbr := 2
	servers, err := createServers(nbr, t)
	if err != nil {
		t.Fatal("Server-creation:", err)
	}
	for _, server := range servers {
		err = server.ReadSSH()
		if err != nil {
			t.Fatal(err)
		}
		err = server.WriteConfig(server.DirSSH + "/server.conf")
		if err != nil {
			t.Fatal(err)
		}
	}

	servers_copy := make([]*ServerConfig, nbr)
	for i, s := range servers {
		servers_copy[i] = ReadServerConfig(s.DirSSH + "/server.conf")
	}
}

func TestStartServer(t *testing.T) {
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
}

func TestCreateSSHDir(t *testing.T) {
	servers, err := createServers(1, t)
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range servers {
		dbg.Print(server)
	}
}

func createServers(nbr int, t *testing.T) ([]*ServerConfig, error) {
	ret := make([]*ServerConfig, nbr)
	for i := range ret {
		ret[i] = CreateServerConfig("localhost:" + strconv.Itoa(2000+i))
		tmp, err := ssh_ks.SetupTmpHosts()
		if err != nil {
			t.Fatal("Couldn't setup tmp:", err)
		}
		ret[i].DirSSH = tmp
		ret[i].DirSSHD = tmp
		err = createBogusSSH(tmp, "id_rsa")
		if err != nil {
			return nil, err
		}
		err = createBogusSSH(tmp, "ssh_host_rsa_key")
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func createBogusSSH(dir, file string) error {
	dbg.Lvl2("Directory is:", dir)
	out, err := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-N", "", "-f",
		dir+file).CombinedOutput()
	dbg.Lvl5(string(out))
	if err != nil {
		return err
	}
	return nil
}
