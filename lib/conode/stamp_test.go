package conode_test

import (
	"fmt"
	"github.com/dedis/cothority/lib/conode"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	runConode(t)
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

func runConode(t *testing.T) {
	os.Chdir("testdata")
	if err := exec.Command("go", "build", "../../../app/conode").Run(); err != nil {
		t.Error(fmt.Sprintf("Error building : %v", err))
	}

	go exec.Command("./conode", "run", "-key", "key1").Run()
	go exec.Command("./conode", "run", "-key", "key2").Run()
	time.Sleep(time.Second * 2)
}

func stopConode() {
	exec.Command("killall", "conode").Run()
}
