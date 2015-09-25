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
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"math"
	"os"
	"strconv"
	"time"
	"github.com/dedis/cothority/lib/deploy"
	"bufio"
	"github.com/BurntSushi/toml"
	"strings"
	"path/filepath"
)

// Configuration-variables
var deployP deploy.Platform
var port int = 8081

func init() {
	deployP = deploy.NewPlatform()
}

/*
 * Starting the simulation
 * it takes a slice of strings to configuration-files that are to be
 * copied over to the deterlab-server
 */
func Start(runconfigs []string) {
	if len(runconfigs) == 0 {
		dbg.Fatal("Please give a simulation to run")
	}

	for _, runconfig := range runconfigs {
		deter, tests := ReadRunfile(runconfig)
		deter.Debug = dbg.DebugVisible
		if len(tests) == 0 {
			dbg.Fatal("No tests found in", runconfig)
		}
		if deter.App == "" {
			dbg.Fatal("No app defined in", runconfig)
		}

		deployP.Configure(&deter)

		deployP.Stop()

		dbg.Lvl3("Going to run tests for", runconfig, strings.Replace(strings.Join(tests, "--"),
			"\n", ", ", -1))
		RunTests(filepath.Base(runconfig), tests)
	}
}

// Runs the given tests and puts the output into the
// given file name. It outputs RunStats in a CSV format.
func RunTests(name string, tests []string) {
	s := stats{}
	s.InitStats(name, tests)
	if nobuild == false {
		deployP.Build(build)
	}

	MkTestDir()
	nTimes := 1
	stopOnSuccess := true
	for run, t := range tests {
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
func RunTest(t string) (RunStats, error) {
	done := make(chan struct{})
	var rs RunStats

	deployP.Deploy(t)
	err := deployP.Start()
	if err != nil {
		log.Fatal(err)
		return rs, nil
	}

	// give it a while to start up
	time.Sleep(10 * time.Second)

	go func() {
		rs = Monitor()
		deployP.Stop()
		dbg.Lvl2("Test complete:", rs)
		done <- struct{}{}
	}()

	// timeout the command if it takes too long
	select {
	case <-done:
		if isZero(rs.MinTime) || isZero(rs.MaxTime) || isZero(rs.AvgTime) || math.IsNaN(rs.Rate) || math.IsInf(rs.Rate, 0) {
			return rs, errors.New(fmt.Sprintf("unable to get good data: %+v", rs))
		}
		return rs, nil
	}
}

type stats struct {
	rs          []RunStats
	name        string
	simulations []string
	file        *os.File
}

func (s *stats)InitStats(name string, simulations []string) {
	var err error
	s.name = name
	s.simulations = simulations
	s.rs = make([]RunStats, len(simulations))
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

func (s *stats)WriteStats(run int, runs []RunStats) {
	if len(runs) == 0 {
		dbg.Lvl1("unable to get any data for test:", s.simulations[run])
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

/* Reads in a configuration-file for a run. The configuration-file has the
 * following syntax:
 * Name1 = value1
 * Name2 = value2
 * [empty line]
 * n1, n2, n3, n4
 * v11, v12, v13, v14
 * v21, v22, v23, v24
 *
 * The Name1...Namen are general configuration-options for deploy.
 * n1..nn are configuration-options for the 'app'
 */
func ReadRunfile(filename string) (deploy.Deter, []string) {
	var deter deploy.Deter
	var tests []string

	dbg.Lvl3("Reading file", filename)

	file, err := os.Open(filename)
	if err != nil {
		dbg.Fatal("Couldn't open file", file, err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dbg.Lvl4("Decoding", scanner.Text())
		if scanner.Text() == "" {
			break
		}
		toml.Decode(scanner.Text(), &deter)
		dbg.Lvl4("Deter is now", deter)
	}

	scanner.Scan()
	args := strings.Split(scanner.Text(), ", ")
	for scanner.Scan() {
		test := ""
		for i, value := range strings.Split(scanner.Text(), ", ") {
			test += args[i] + " = " + value + "\n"
		}
		tests = append(tests, test)
	}

	return deter, tests
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
