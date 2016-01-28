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
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dedis/cothority/simul/platform"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
)

// Configuration-variables
var deployP platform.Platform

var platformDst = "localhost"
var app = ""
var nobuild = false
var clean = true
var build = ""
var machines = 3
var monitorPort = 10000
var simRange = ""

// SHORT TERM solution of referencing
// the different apps.
// TODO: make the lib/app/*COnfig.go have their own reference
// so they can issue Stats, read their own config depending on platform,
// etc etc
const (
	ShamirSign string = "shamir"
	CollSign   string = "sign"
	CollStamp  string = "stamp"
	Naive      string = "naive"
	NTree      string = "ntree"
)

func init() {
	flag.StringVar(&platformDst, "platform", platformDst, "platform to deploy to [deterlab,localhost]")
	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")
	flag.BoolVar(&clean, "clean", false, "Only clean platform")
	flag.StringVar(&build, "build", "", "List of packages to build")
	flag.IntVar(&machines, "machines", machines, "Number of machines on Deterlab")
	flag.IntVar(&monitorPort, "mport", monitorPort, "Port-number for monitor")
	flag.StringVar(&simRange, "range", simRange, "Range of simulations to run. 0: or 3:4 or :4")
	flag.IntVar(&dbg.DebugVisible, "debug", dbg.DebugVisible, "Change debug level (0-5)")
}

// Reads in the platform that we want to use and prepares for the tests
func main() {
	flag.Parse()
	monitor.SinkPort = monitorPort
	deployP = platform.NewPlatform(platformDst)
	if deployP == nil {
		dbg.Fatal("Platform not recognized.", platformDst)
	}
	dbg.Lvl1("Deploying to", platformDst)

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

		if clean {
			deployP.Deploy(runconfigs[0])
			deployP.Cleanup()
		} else {
			logname := strings.Replace(filepath.Base(simulation), ".toml", "", 1)
			RunTests(logname, runconfigs)
		}
	}
}

// Runs the given tests and puts the output into the
// given file name. It outputs RunStats in a CSV format.
func RunTests(name string, runconfigs []platform.RunConfig) {

	if nobuild == false {
		deployP.Build(build)
	}

	MkTestDir()
	rs := make([]monitor.Stats, len(runconfigs))
	nTimes := 1
	stopOnSuccess := true
	var f *os.File
	args := os.O_CREATE | os.O_RDWR | os.O_TRUNC
	// If a range is given, we only append
	if simRange != "" {
		args = os.O_CREATE | os.O_RDWR | os.O_APPEND
	}
	f, err := os.OpenFile(TestFile(name), args, 0660)
	if err != nil {
		dbg.Fatal("error opening test file:", err)
	}
	defer f.Close()
	err = f.Sync()
	if err != nil {
		dbg.Fatal("error syncing test file:", err)
	}

	start, stop := getStartStop(len(runconfigs))
	for i, t := range runconfigs {
		// Implement a simple range-argument that will skip checks not in range
		if i < start || i > stop {
			dbg.Lvl1("Skipping", t, "because of range")
			continue
		}
		dbg.Lvl1("Doing run", t)

		// run test t nTimes times
		// take the average of all successful runs
		runs := make([]monitor.Stats, 0, nTimes)
		for r := 0; r < nTimes; r++ {
			stats, err := RunTest(t)
			if err != nil {
				dbg.Fatal("error running test:", err)
			}

			runs = append(runs, stats)
			if stopOnSuccess {
				break
			}
		}

		if len(runs) == 0 {
			dbg.Lvl1("unable to get any data for test:", t)
			continue
		}

		s := monitor.AverageStats(runs)
		if i == 0 {
			s.WriteHeader(f)
		}
		rs[i] = s
		rs[i].WriteValues(f)
		err = f.Sync()
		if err != nil {
			dbg.Fatal("error syncing data to test file:", err)
		}
	}
}

// Runs a single test - takes a test-file as a string that will be copied
// to the deterlab-server
func RunTest(rc platform.RunConfig) (monitor.Stats, error) {
	done := make(chan struct{})
	if platformDst == "localhost" {
		machs := rc.Get("machines")
		ppms := rc.Get("ppm")
		mach, _ := strconv.Atoi(machs)
		ppm, _ := strconv.Atoi(ppms)
		rc.Put("machines", "1")
		rc.Put("ppm", strconv.Itoa(ppm*mach))
	}
	rs := monitor.NewStats(rc.Map())
	monitor := monitor.NewMonitor(rs)

	if err := deployP.Deploy(rc); err != nil {
		dbg.Error(err)
		return *rs, err
	}
	if err := deployP.Cleanup(); err != nil {
		dbg.Error(err)
		return *rs, err
	}
	go func() {
		monitor.Listen()
	}()
	// Start monitor before so ssh tunnel can connect to the monitor
	// in case of deterlab.
	err := deployP.Start()
	if err != nil {
		dbg.Error(err)
		return *rs, err
	}

	go func() {
		var err error
		if err = deployP.Wait(); err != nil {
			dbg.Lvl3("Test failed:", err)
			deployP.Cleanup()
			done <- struct{}{}
		}
		dbg.Lvl3("Test complete:", rs)
		done <- struct{}{}
	}()

	// can timeout the command if it takes too long
	select {
	case <-done:
		monitor.Stop()
		return *rs, err
	}
}

type runFile struct {
	Machines int
	Args     string
	Runs     string
}

func MkTestDir() {
	err := os.MkdirAll("test_data/", 0777)
	if err != nil {
		dbg.Fatal("failed to make test directory")
	}
}

func TestFile(name string) string {
	return "test_data/" + name + ".csv"
}

func isZero(f float64) bool {
	return math.Abs(f) < 0.0000001
}

// returns a tuple of start and stop configurations to run
func getStartStop(rcs int) (int, int) {
	ss_str := strings.Split(simRange, ":")
	start, err := strconv.Atoi(ss_str[0])
	stop := rcs
	if err == nil {
		stop = start
		if len(ss_str) > 1 {
			stop, err = strconv.Atoi(ss_str[1])
			if err != nil {
				stop = rcs
			}
		}
	}
	dbg.Lvl2("Range is", start, "...", stop)
	return start, stop
}
