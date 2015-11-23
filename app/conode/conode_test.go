package main
import (
	"testing"
	"os"
	"io/ioutil"
	"fmt"
	"sync"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/conode"
)

func TestBuild(t *testing.T) {
	// Just testing that build is done correctly
}

func TestMakeConfig(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 2)
	os.Chdir("/tmp")
	KeyGeneration("key1", "localhost:2000")
	KeyGeneration("key2", "localhost:2010")
	key1, _ := ioutil.ReadFile("key1.pub")
	key2, _ := ioutil.ReadFile("key2.pub")
	ioutil.WriteFile("hosts", []byte(fmt.Sprintf("%s%s", key1, key2)), 0666)
	Build("hosts", 2, "config.toml")

	wg := sync.WaitGroup{}
	wg.Add(2)
	conode.PeerMaxRounds = 3
	go (func() {
		Run("config.toml", "key1")
		wg.Done()
	})()
	go (func() {
		Run("config.toml", "key2")
		wg.Done()
	})()
	wg.Wait()
}