package deploy
import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"bytes"
	"os"
)

type Platform interface {
	Configure(*Deter)
	Build(build string) error
	Deploy(string) error
	Start() error
	Stop() error
}

func NewPlatform() Platform {
	return &Deter{}
}

type Config struct {
	// hpn is the replication factor of hosts per node: how many hosts do we want per node
	Hpn int
	// bf is the branching factor of the tree that we want to build
	Bf int

	// How many messages to send
	Nmsgs int
	// The speed of messages/s
	Rate int
	// How many rounds
	Rounds int
	// Pre-defined failure rate
	Failures int
	// Rounds for root to wait before failing
	RFail int
	// Rounds for follower to wait before failing
	FFail int

	// RootWait - how long the root timestamper waits for the clients to start up
	RootWait int
	// Which app to run
	App string
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite string
}

func WriteConfig(conf interface{}, filename string, dirOpt ...string) {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(conf); err != nil {
		dbg.Fatal(err)
	}
	dir := "."
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	}
	err := ioutil.WriteFile(dir + "/" + filename, buf.Bytes(), 0660)
	if err != nil {
		dbg.Fatal(err)
	}
}

func ReadConfig(conf interface{}, filename string, dirOpt ...string) (error) {
	dir := "."
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	}
	buf, err := ioutil.ReadFile(dir + "/" + filename)
	if err != nil {
		pwd, _ := os.Getwd()
		dbg.Lvl1("Didn't find", filename, "in", pwd)
		return err
	}

	_, err = toml.Decode(string(buf), conf)
	if err != nil {
		dbg.Fatal(err)
	}

	return nil
}
