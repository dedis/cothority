// Outputting data: output to csv files (for loading into excel)
//   make a datastructure per test output file
//   all output should be in the test_data subdirectory
//
// connect with logging server (receive json until "EOF" seen or "terminating")
//   connect to websocket ws://localhost:8080/log
//   receive each message as bytes
//		 if bytes contains "EOF" or contains "terminating"
//       wrap up the round, output to test_data directory, kill deploy2deter
//
// for memstats check localhost:8080/d/server-0-0/debug/vars
//   parse out the memstats zones that we are concerned with
//
// different graphs needed rounds:
//   load on the x-axis: increase messages per round holding everything else constant
//			hpn=40 bf=10, bf=50
//
// latency on y-axis, timestamp servers on x-axis push timestampers as higher as possible
//
//
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/deploy/platform"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

// Configuration-variables
var deployP platform.Platform
var port int = 8081

var platform_dst = "deterlab"
var app = ""
var nobuild = false
var build = ""
var machines = 3

func init() {
	flag.StringVar(&platform_dst, "platform", platform_dst, "platform to deploy to [deterlab,localhost]")
	flag.IntVar(&dbg.DebugVisible, "debug", dbg.DebugVisible, "Debugging-level. 0 is silent, 5 is flood")
	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")
	flag.StringVar(&build, "build", "", "List of packages to build")
	flag.IntVar(&machines, "machines", machines, "Number of machines on Deterlab")
}

// Reads in the platform that we want to use and prepares for the tests
func main() {
	flag.Parse()
	deployP = platform.NewPlatform(platform_dst)
	if deployP == nil {
		dbg.Fatal("Platform not recognized.", platform_dst)
	}
	dbg.Lvl1("Deploying to", platform_dst)

	simulations := flag.Args()
	if len(simulations) == 0 {
		dbg.Fatal("Please give a simulation to run")
	}

	for _, simulation := range simulations {
		runconfigs := platform.ReadRunFile(deployP, simulation)
		if len(runconfigs) == 0 {
			dbg.Fatal("No tests found in", simulation)
		}

		deployP.Configure()

		deployP.Stop()

		//testprint := strings.Replace(strings.Join(runconfigs, "--"), "\n", ", ", -1)
		//dbg.Lvl3("Going to run tests for", simulation, testprint)
		logname := strings.Replace(filepath.Base(simulation), ".toml", "", 1)
		RunTests(logname, runconfigs)
	}
}

// Runs the given tests and puts the output into the
// given file name. It outputs RunStats in a CSV format.
func RunTests(name string, runconfigs []platform.RunConfig) {
	s := stats{}
	s.InitStats(name, runconfigs)
	if nobuild == false {
		deployP.Build(build)
	}

	MkTestDir()
	nTimes := 1
	stopOnSuccess := true
	for run, t := range runconfigs {
		// run test t nTimes times
		// take the average of all successful runs
		var runs []RunStats
		for r := 0; r < nTimes; r++ {
			run, err := RunTest(t)
			if err != nil {
				log.Fatalln("error running test:", err)
			}

			if deployP.Stop() == nil {
				runs = append(runs, run)
				if stopOnSuccess {
					break
				}
			} else {
				dbg.Lvl1("Error for test ", r, " : ", err)
			}
		}

		s.WriteStats(run, runs)
	}
}

// Runs a single test - takes a test-file as a string that will be copied
// to the deterlab-server
func RunTest(rc platform.RunConfig) (RunStats, error) {
	done := make(chan struct{})
	var rs RunStats

	deployP.Deploy(rc)
	err := deployP.Start()
	if err != nil {
		log.Fatal(err)
		return rs, nil
	}

	go func() {
		if platform_dst != "deterlab" {
			dbg.Lvl1("Not starting monitor as not in deterlab-mode!")
			deployP.Wait()
			rs = RunStats{}
		} else {
			rs = Monitor()
			dbg.Lvl2("Test complete:", rs)
		}
		done <- struct{}{}
	}()

	// timeout the command if it takes too long
	select {
	case <-done:
		if platform_dst == "deterlab" && (isZero(rs.MinTime) || isZero(rs.MaxTime) || isZero(rs.AvgTime) || math.IsNaN(rs.Rate) || math.IsInf(rs.Rate, 0)) {
			return rs, fmt.Errorf("unable to get good data: %+v", rs)
		}
		return rs, nil
	}
}

type stats struct {
	rs         []RunStats
	name       string
	runconfigs []platform.RunConfig
	file       *os.File
}

func (s *stats) InitStats(name string, runconfigs []platform.RunConfig) {
	var err error
	s.name = name
	s.runconfigs = runconfigs
	s.rs = make([]RunStats, len(runconfigs))
	MkTestDir()
	s.file, err = os.OpenFile(TestFile(name), os.O_CREATE | os.O_RDWR | os.O_TRUNC, 0660)
	if err != nil {
		log.Fatal("error opening test file:", err)
	}
	_, err = s.file.Write(s.rs[0].CSVHeader())
	if err != nil {
		log.Fatal("error writing test file header:", err)
	}
	err = s.file.Sync()
	if err != nil {
		log.Fatal("error syncing test file:", err)
	}
}

func (s *stats) WriteStats(run int, runs []RunStats) {
	if len(runs) == 0 {
		dbg.Lvl1("unable to get any data for test:", s.runconfigs[run])
		return
	}

	s.rs[run] = RunStatsAvg(runs)
	//log.Println(fmt.Sprintf("Writing to CSV for %d: %+v", i, rs[i]))
	_, err := s.file.Write(s.rs[run].CSV())
	if err != nil {
		log.Fatal("error writing data to test file:", err)
	}
	err = s.file.Sync()
	if err != nil {
		log.Fatal("error syncing data to test file:", err)
	}

	MkTestDir()
	cl, err := os.OpenFile(
		TestFile("client_latency_" + s.name + "_" + strconv.Itoa(run)),
		os.O_CREATE | os.O_RDWR | os.O_TRUNC, 0660)
	if err != nil {
		log.Fatal("error opening test file:", err)
	}
	_, err = cl.Write(s.rs[run].TimesCSV())
	if err != nil {
		log.Fatal("error writing client latencies to file:", err)
	}
	err = cl.Sync()
	if err != nil {
		log.Fatal("error syncing data to latency file:", err)
	}
	cl.Close()
}

type runFile struct {
	Machines int
	Args     string
	Runs     string
}

func MkTestDir() {
	err := os.MkdirAll("test_data/", 0777)
	if err != nil {
		log.Fatal("failed to make test directory")
	}
}

func TestFile(name string) string {
	return "test_data/" + name + ".csv"
}

func isZero(f float64) bool {
	return math.Abs(f) < 0.0000001
}
