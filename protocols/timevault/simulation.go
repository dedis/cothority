package timevault

import (
	"bytes"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.SimulationRegister("TimeVault", NewSimulation)
}

// Simulation implements a TimeVault simulation.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation creates a TimeVault simulation.
func NewSimulation(config string) (sda.Simulation, error) {
	tvs := &Simulation{}
	_, err := toml.Decode(config, tvs)
	if err != nil {
		return nil, err
	}
	return tvs, nil
}

// Setup configures a TimeVault simulation.
func (tvs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	tvs.CreateEntityList(sim, hosts, 2000)
	err := tvs.CreateTree(sim)
	return sim, err
}

// Run initiates a TimeVault simulation
func (tvs *Simulation) Run(config *sda.SimulationConfig) error {

	msg := []byte("TimeVault Test")

	p, err := config.Overlay.CreateProtocol(config.Tree, "TimeVault")
	if err != nil {
		return err
	}
	proto := p.(*TimeVault)

	sealMeasure := monitor.NewTimeMeasure("round_seal")
	sealBWMeasure := monitor.NewCounterIOMeasure("round_seal_bw", config.Host)
	dbg.Lvl1("TimeVault - starting")
	proto.Start()
	dbg.Lvl1("TimeVault - setup done")

	sid, key, err := proto.Seal(time.Second * 2)
	if err != nil {
		dbg.Fatal(err)
	}
	dbg.Lvl1("TimeVault - seal created")

	// Do ElGamal encryption
	c, eKey := elGamalEncrypt(proto.Suite(), msg, key)
	sealMeasure.Record()
	sealBWMeasure.Record()

	<-time.After(time.Second * 5)

	openMeasure := monitor.NewTimeMeasure("round_open")
	openBWMeasure := monitor.NewCounterIOMeasure("round_open_bw", config.Host)
	x, err := proto.Open(sid)
	if err != nil {
		dbg.Fatal(err)
	}
	dbg.Lvl1("TimeVault - opening secret successful")
	openBWMeasure.Record()

	X := proto.Suite().Point().Mul(eKey, x)
	m, err := elGamalDecrypt(proto.Suite(), c, X)
	if err != nil {
		dbg.Fatal(err)
	}

	if !bytes.Equal(m, msg) {
		dbg.Fatal("Error, decryption failed")
	}
	openMeasure.Record()
	dbg.Lvl1("TimeVault - decryption successful")

	return nil
}

func elGamalEncrypt(suite abstract.Suite, msg []byte, key abstract.Point) (abstract.Point, abstract.Point) {
	kp := config.NewKeyPair(suite)
	m, _ := suite.Point().Pick(msg, random.Stream) // can take at most 29 bytes in one step
	c := suite.Point().Add(m, suite.Point().Mul(key, kp.Secret))
	return c, kp.Public
}

func elGamalDecrypt(suite abstract.Suite, c abstract.Point, key abstract.Point) ([]byte, error) {
	return suite.Point().Sub(c, key).Data()
}
