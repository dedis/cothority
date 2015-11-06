package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	runConode()
	s, err := conode.NewStamp("config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	tsm, err := s.GetStamp([]byte("test"), "")
	if err != nil {
		t.Fatal("Couldn't get stamp from server:", err)
	}

	if !tsm.Srep.AggPublic.Equal(s.X0) {
		t.Fatal("Not correct aggregate public key")
	}
	stopConode()
}

func runConode() {
	os.Chdir("testdata")
	exec.Command("go", "build", "../../../app/conode").Run()
	go func() {
		exec.Command("./conode", "run", "-key", "key1").Run()
	}()
	go func() {
		exec.Command("./conode", "run", "-key", "key2").Run()
	}()
	time.Sleep(time.Second * 2)
}

func stopConode() {
	exec.Command("killall", "conode").Run()
}
