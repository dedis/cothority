// NOTE: SHOULD BE RUN FROM run_tests directory
// note: deploy2deter must be run from within it's directory
//
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
// run time command with the deploy2deter exec.go (timestamper) instance associated with the root
//    and a random set of servers
//
// latency on y-axis, timestamp servers on x-axis push timestampers as higher as possible
//
//
// RunTest(hpn, bf), Monitor() -> RunStats() -> csv -> Excel
//
package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"
	"github.com/ineiti/cothorities/deploy"
	"github.com/ineiti/cothorities/deploy/deterlab"
)

// Configuration-variables
var config *deploy.Config
var platform deploy.Platform
var login string = "ineiti"
var host string = "users.deterlab.net"
var project string = "Dissent-CS"
var nobuild bool = false
var port int = 8081
// time-per-round * DefaultRounds = 10 * 20 = 3.3 minutes now
// this leaves us with 7 minutes for test setup and tear-down
var DefaultRounds int = 1

func NewPlatform() deploy.Platform {
	return &deploy_deter.Deter{Config:deploy.NewConfig()}
}

func init() {
	config = deploy.NewConfig()
	platform = NewPlatform()

	flag.StringVar(&login, "user", login, "User on the deterlab-machines")
	flag.StringVar(&host, "host", host, "User on the deterlab-machines")
	flag.StringVar(&project, "project", project, "Name of the project on DeterLab")

	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")
	flag.IntVar(&config.Nmachs, "machines", config.Nmachs, "Number of machines (servers running the client)")
	flag.IntVar(&config.Nloggers, "loggers", config.Nloggers, "Number of loggers")
	flag.IntVar(&port, "port", port, "Port to forward debugging-information")
	flag.IntVar(&config.Bf, "branch", config.Bf, "Branching Factor")
	flag.IntVar(&config.Hpn, "hpn", config.Hpn, "Host per node (physical machine)")
	flag.IntVar(&config.Debug, "debug", config.Debug, "Debugging-level. 0 is silent, 5 is flood")
}


type T struct {
	nmachs      int
	hpn         int
	bf          int

	rate        int
	rounds      int
	failures    int

	rFail       int
	fFail       int
	testConnect bool
	app         string
}

var StampTestSingle = []T{
	{0, 1, 2,
		30, 20, 0,
		0, 0, false, "stamp"},
}

var SignTestSingle = []T{
	{0, 1, 2, 30, 20, 0, 0, 0, false, "sign"},
}

func main() {
	flag.Parse()
	platform.Configure(config)
	dbg.DebugVisible = config.Debug

	dbg.Lvl1("Setting up everything")
	dbg.Lvl1(fmt.Sprintf("Options : machines %d,loggers %d, user %s, project %s", config.Nmachs, config.Nloggers, login, project))
	login = fmt.Sprintf("-user=%s", login)

	platform.Stop()

	if nobuild == false {
		platform.Build()
	}

	dbg.Lvl1("Starting tests")
	DefaultRounds = 5
	RunTests("stamp_test_single", StampTestSingle)
	//RunTests("sing_test_single", SignTestSingle)
	// test the testing framework
	//RunTests("vote_test_no_signing.csv", VTest)
	//RunTests("hosts_test", HostsTest)
	// t := FailureTests
	// RunTests("failure_test.csv", t)
	// RunTests("vote_test", VotingTest)
	// RunTests("failure_test", FailureTests)
	//RunTests("sign_test", SignTest)
	// t := FailureTests
	// RunTests("failure_test", t)
	// t = ScaleTest(10, 1, 100, 2)
	// RunTests("scale_test.csv", t)
	// how does the branching factor effect speed
	// t = DepthTestFixed(100)
	// RunTests("depth_test.csv", t)

	// load test the client
	// t = RateLoadTest(40, 10)
	// RunTests("load_rate_test_bf10.csv", t)
	// t = RateLoadTest(40, 50)
	// RunTests("load_rate_test_bf50.csv", t)

}

// RunTests runs the given tests and puts the output into the
// given file name. It outputs RunStats in a CSV format.
func RunTests(name string, ts []T) {
	for i, _ := range ts {
		ts[i].bf = config.Bf
		if config.Hpn > 0 {
			ts[i].hpn = config.Hpn
		}

		ts[i].nmachs = config.Nmachs
	}

	MkTestDir()
	rs := make([]RunStats, len(ts))
	f, err := os.OpenFile(TestFile(name), os.O_CREATE | os.O_RDWR | os.O_TRUNC, 0660)
	if err != nil {
		log.Fatal("error opening test file:", err)
	}
	_, err = f.Write(rs[0].CSVHeader())
	if err != nil {
		log.Fatal("error writing test file header:", err)
	}
	err = f.Sync()
	if err != nil {
		log.Fatal("error syncing test file:", err)
	}

	nTimes := 1
	stopOnSuccess := true
	for i, t := range ts {
		// run test t nTimes times
		// take the average of all successful runs
		var runs []RunStats
		for r := 0; r < nTimes; r++ {
			run, err := RunTest(t)
			if err != nil {
				log.Fatalln("error running test:", err)
			}

			if platform.Stop() == nil {
				runs = append(runs, run)
				if stopOnSuccess {
					break
				}
			} else {
				dbg.Lvl1("Error for test ", r, " : ", err)
			}
		}

		if len(runs) == 0 {
			dbg.Lvl1("unable to get any data for test:", t)
			continue
		}

		rs[i] = RunStatsAvg(runs)
		//log.Println(fmt.Sprintf("Writing to CSV for %d: %+v", i, rs[i]))
		_, err := f.Write(rs[i].CSV())
		if err != nil {
			log.Fatal("error writing data to test file:", err)
		}
		err = f.Sync()
		if err != nil {
			log.Fatal("error syncing data to test file:", err)
		}

		cl, err := os.OpenFile(
			TestFile("client_latency_" + name + "_" + strconv.Itoa(i)),
			os.O_CREATE | os.O_RDWR | os.O_TRUNC, 0660)
		if err != nil {
			log.Fatal("error opening test file:", err)
		}
		_, err = cl.Write(rs[i].TimesCSV())
		if err != nil {
			log.Fatal("error writing client latencies to file:", err)
		}
		err = cl.Sync()
		if err != nil {
			log.Fatal("error syncing data to latency file:", err)
		}
		cl.Close()

	}
}

// hpn, bf, nmsgsG
func RunTest(t T) (RunStats, error) {
	// add timeout for 10 minutes?
	done := make(chan struct {})
	var rs RunStats
	cfg := &deploy.Config{
		t.nmachs, config.Nloggers, t.hpn, t.bf,
		-1, t.rate, t.rounds, t.failures, t.rFail, t.fFail,
		config.Debug, config.App, config.Suite }

	dbg.Lvl1(fmt.Sprintf("Running test with parameters %+v", cfg))
	dbg.Lvl1("Failures percent is", t.failures)

	platform.Configure(cfg)
	platform.Deploy()
	err := platform.Start()
	if err != nil {
		log.Fatal(err)
		return rs, nil
	}

	// give it a while to start up
	time.Sleep(10 * time.Second)

	go func() {
		rs = Monitor(t.bf)
		platform.Stop()
		dbg.Lvl2("TEST COMPLETE:", rs)
		done <- struct {}{}
	}()

	// timeout the command if it takes too long
	select {
	case <-done:
		if isZero(rs.MinTime) || isZero(rs.MaxTime) || isZero(rs.AvgTime) || math.IsNaN(rs.Rate) || math.IsInf(rs.Rate, 0) {
			return rs, errors.New(fmt.Sprintf("unable to get good data: %+v", rs))
		}
		return rs, nil
	case <-time.After(5 * time.Minute):
		return rs, errors.New("timed out")
	}
}


// high and low specify how many milliseconds between messages
func RateLoadTest(hpn, bf int) []T {
	return []T{
		{0, hpn, bf, 5000, DefaultRounds, 0, 0, 0, false, "stamp"}, // never send a message
		{0, hpn, bf, 5000, DefaultRounds, 0, 0, 0, false, "stamp"}, // one per round
		{0, hpn, bf, 500, DefaultRounds, 0, 0, 0, false, "stamp"}, // 10 per round
		{0, hpn, bf, 50, DefaultRounds, 0, 0, 0, false, "stamp"}, // 100 per round
		{0, hpn, bf, 30, DefaultRounds, 0, 0, 0, false, "stamp"}, // 1000 per round
	}
}

func DepthTest(hpn, low, high, step int) []T {
	ts := make([]T, 0)
	for bf := low; bf <= high; bf += step {
		ts = append(ts, T{0, hpn, bf, 10, DefaultRounds, 0, 0, 0, false, "stamp"})
	}
	return ts
}

func DepthTestFixed(hpn int) []T {
	return []T{
		{0, hpn, 1, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 2, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 4, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 8, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 16, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 32, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 64, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 128, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 256, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{0, hpn, 512, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
	}
}

func ScaleTest(bf, low, high, mult int) []T {
	ts := make([]T, 0)
	for hpn := low; hpn <= high; hpn *= mult {
		ts = append(ts, T{0, hpn, bf, 10, DefaultRounds, 0, 0, 0, false, "stamp"})
	}
	return ts
}

// nmachs=32, hpn=128, bf=16, rate=500, failures=20, root failures, failures
var FailureTests = []T{
	{0, 64, 16, 30, 50, 0, 0, 0, false, "stamp"},
	{0, 64, 16, 30, 50, 0, 5, 0, false, "stamp"},
	{0, 64, 16, 30, 50, 0, 10, 0, false, "stamp"},
	{0, 64, 16, 30, 50, 5, 0, 5, false, "stamp"},
	{0, 64, 16, 30, 50, 5, 0, 10, false, "stamp"},
	{0, 64, 16, 30, 50, 5, 0, 10, true, "stamp"},
}

var VotingTest = []T{
	{0, 64, 16, 30, 50, 0, 0, 0, true, "stamp"},
	{0, 64, 16, 30, 50, 0, 0, 0, false, "stamp"},
}

func FullTests() []T {
	var nmachs = []int{1, 16, 32}
	var hpns = []int{1, 16, 32, 128}
	var bfs = []int{2, 4, 8, 16, 128}
	var rates = []int{5000, 500, 100, 30}
	failures := 0

	var tests []T
	for _, nmach := range nmachs {
		for _, hpn := range hpns {
			for _, bf := range bfs {
				for _, rate := range rates {
					tests = append(tests, T{nmach, hpn, bf, rate, DefaultRounds, failures, 0, 0, false, "stamp"})
				}
			}
		}
	}

	return tests
}

var HostsTest = []T{
	{0, 1, 2, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 2, 3, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 4, 3, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 8, 8, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 16, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 32, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 64, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{0, 128, 16, 30, 50, 0, 0, 0, false, "stamp"},
}

var SignTest = []T{
	{0, 1, 2, 30, 20, 0, 0, 0, false, "sign"},
	{0, 2, 3, 30, 20, 0, 0, 0, false, "sign"},
	{0, 4, 3, 30, 20, 0, 0, 0, false, "sign"},
	{0, 8, 8, 30, 20, 0, 0, 0, false, "sign"},
	{0, 16, 16, 30, 20, 0, 0, 0, false, "sign"},
	{0, 32, 16, 30, 20, 0, 0, 0, false, "sign"},
	{0, 64, 16, 30, 20, 0, 0, 0, false, "sign"},
	{0, 128, 16, 30, 50, 0, 0, 0, false, "sign"},
}

var VTest = []T{
	{0, 1, 3, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 2, 4, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 4, 6, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 8, 8, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 16, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 32, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 64, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{0, 128, 16, 10000000, 20, 0, 0, 0, false, "vote"},
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
