package medco

import (
	"bufio"
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/random"
	"math"
	"math/rand"
	"os"
	"strconv"


)



const NUM_MESS = 10000
const NUM_BUCKET = 1
var BUCKET_DESC = []int64{}
const needle = "code1"

func codeGen(i int) string {
	return "code" + strconv.Itoa(i%100)
}

func ageGen(i int) int64 {
	return int64(math.Min(100, math.Max(0, math.Floor(rand.NormFloat64()*20 + 40))))
}

func bucket(age int64) int {
	var i int
	for i=0; i < NUM_BUCKET-1 && age >= BUCKET_DESC[i]; i++ {}
	return i
}


func init() {
	sda.SimulationRegister("PrivateCount", NewPrivateCountSimulation)
}

type PrivateCountSimulation struct {
	sda.SimulationBFTree
}

func NewPrivateCountSimulation(config string) (sda.Simulation, error) {
	sim := &PrivateCountSimulation{}
	_,err := toml.Decode(config, sim)
	if err != nil {
		return nil,err
	}
	return sim, nil
}

// Send a file to all nodes ? Dir lisible dnas Run et Protocol
func (sim *PrivateCountSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateEntityList(sc, hosts, 2000)
	err := sim.CreateTree(sc)
	if err != nil {
		return nil, err
	}

	expectedResults := make([]int64, NUM_BUCKET)
	dbg.Lvl1("Begin test encrypted data generation")
	for _, node := range sc.Tree.List() {
		for bucket, count := range createTestDataFile(node.Id.String(), dir, NUM_MESS/sim.Hosts) {
			expectedResults[bucket] += count
		}
	}
	writeExpectedResults(expectedResults, dir)
	dbg.Lvl1("Ended test encrypted data generation, expected result: ", expectedResults)

	return sc, nil
}


// Run when all node ready , run as a node ?
func (sim *PrivateCountSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		root, err := config.Overlay.CreateNewNodeName("PrivateCount", config.Tree)
		if err != nil {
			return err
		}

		expectedResults, err := readExpectedResults()
		if err != nil {
			return err
		}

		suite := root.Suite()
		aggregateKey := root.EntityList().Aggregate

		clientSecret  := suite.Secret().Pick(random.Stream)
		clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)
		clientQuery,_ := EncryptBytes(suite, aggregateKey, []byte(needle))

		root.ProtocolInstance().(*PrivateCountProtocol).ClientPublicKey = &clientPublic
		root.ProtocolInstance().(*PrivateCountProtocol).ClientQuery = clientQuery
		root.ProtocolInstance().(*PrivateCountProtocol).BucketDesc = &BUCKET_DESC
		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")

		root.StartProtocol()

		result := <-root.ProtocolInstance().(*PrivateCountProtocol).FeedbackChannel
		dbg.Lvl1("Got result", DecryptIntVector(suite, clientSecret, result), "expected", expectedResults)
		round.Record()
	}
	// Clean up

	for _, node := range config.Tree.List() {
		os.Remove(node.Id.String()+".txt")
	}

	return nil
}

func createTestDataFile(name, dir  string, numMessage int) []int64 {
	dbg.Lvl1("Creating dataset of "+strconv.FormatInt(int64(numMessage),10)+" entries in files "+dir+"/"+name+".txt")
	file, _ := os.Create(dir+"/"+name+".txt")
	defer file.Close()
	targetCounts := make([]int64, NUM_BUCKET)
	for i := 0; i < numMessage; i++ {
		code := codeGen(i)
		age := ageGen(i)
		file.WriteString(code+" "+strconv.FormatInt(age, 10)+"\n")
		if code == needle {
			targetCounts[bucket(age)] += 1
		}
	}
	return targetCounts
}

func writeExpectedResults(results []int64, dir string) {
	file, _ := os.Create(dir+"/expected.txt")
	defer file.Close()
	for _,count := range results {
		file.WriteString(strconv.FormatInt(count,10)+" ")
	}
}

func readExpectedResults() ([]int64, error) {
	if file , err := os.Open("expected.txt"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanWords)
		expectedResults := make([]int64, NUM_BUCKET)
		for i := 0; scanner.Scan(); i++ {
			if i >= NUM_BUCKET {
				return []int64{}, errors.New("The bucket count in expected.txt does not match NUM_BUCKET.")
			}
			if count, err  := strconv.ParseInt(scanner.Text(),10,64); err == nil {
				expectedResults[i] = count
			} else {
				return []int64{}, err
			}
		}
		return expectedResults, nil
	} else {
		return []int64{}, err
	}
}

