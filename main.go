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
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"time"
)

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

var user string = "-user=ineiti"
var host string = "-host=users.deterlab.net"
var DefaultMachs int = 14
var DefaultLoggers int = 3

var hosts_file string = "hosts.txt"
var default_project string = "Dissent-CS"

// time-per-round * DefaultRounds = 10 * 20 = 3.3 minutes now
// this leaves us with 7 minutes for test setup and tear-down
var DefaultRounds int = 1

var view bool
var debug string = "-debug=false"
var nobuild bool = false

func init() {
	flag.StringVar(&user, "user", "ineiti", "User on the deterlab-machines")
	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")

}

// hpn, bf, nmsgsG
func RunTest(t T) (RunStats, error) {
	// add timeout for 10 minutes?
	done := make(chan struct{})
	var rs RunStats
	nmachs := fmt.Sprintf("-nmachs=%d", t.nmachs)
	hpn := fmt.Sprintf("-hpn=%d", t.hpn)
	nmsgs := fmt.Sprintf("-nmsgs=%d", -1)
	bf := fmt.Sprintf("-bf=%d", t.bf)
	rate := fmt.Sprintf("-rate=%d", t.rate)
	rounds := fmt.Sprintf("-rounds=%d", t.rounds)
	failures := fmt.Sprintf("-failures=%d", t.failures)
	rFail := fmt.Sprintf("-rfail=%d", t.rFail)
	fFail := fmt.Sprintf("-ffail=%d", t.fFail)
	tcon := fmt.Sprintf("-test_connect=%t", t.testConnect)
	app := fmt.Sprintf("-app=%s", t.app)
	loggers := fmt.Sprintf("-nloggers=%d", DefaultLoggers)
	cmd := exec.Command("./deploy2deter", nmachs, hpn, nmsgs, bf, rate, rounds, debug, failures, rFail,
		fFail, tcon, app, user, host, loggers)
	log.Println("RUNNING TEST:", cmd.Args)
	log.Println("FAILURES PERCENT:", t.failures)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
		return rs, nil
	}

	// give it a while to start up
	time.Sleep(30 * time.Second)

	go func() {
		rs = Monitor(t.bf)
		cmd.Process.Kill()
		fmt.Println("TEST COMPLETE:", rs)
		done <- struct{}{}
	}()

	// timeout the command if it takes too long
	select {
	case <-done:
		if isZero(rs.MinTime) || isZero(rs.MaxTime) || isZero(rs.AvgTime) || math.IsNaN(rs.Rate) || math.IsInf(rs.Rate, 0) {
			return rs, errors.New("unable to get good data")
		}
		return rs, nil
	case <-time.After(10 * time.Minute):
		return rs, errors.New("timed out")
	}
}

// RunTests runs the given tests and puts the output into the
// given file name. It outputs RunStats in a CSV format.
func RunTests(name string, ts []T) {

	rs := make([]RunStats, len(ts))
	f, err := os.OpenFile(TestFile(name), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0660)
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

	nTimes := 2
	stopOnSuccess := true
	for i, t := range ts {
		// run test t nTimes times
		// take the average of all successful runs
		var runs []RunStats
		for r := 0; r < nTimes; r++ {
			run, err := RunTest(t)
			if err != nil {
				log.Println("error running test:", err)
			}

			// Clean Up after test
			log.Println("KILLING REMAINING PROCESSES")
			cmd := exec.Command("./deploy2deter", "-kill=true",
				fmt.Sprintf("-nmachs=%d", DefaultMachs), user)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			if err == nil {
				runs = append(runs, run)
				if stopOnSuccess {
					break
				}
			}

		}

		if len(runs) == 0 {
			log.Println("unable to get any data for test:", t)
			continue
		}

		rs[i] = RunStatsAvg(runs)
		_, err := f.Write(rs[i].CSV())
		if err != nil {
			log.Fatal("error writing data to test file:", err)
		}
		err = f.Sync()
		if err != nil {
			log.Fatal("error syncing data to test file:", err)
		}

		cl, err := os.OpenFile(
			TestFile("client_latency_"+name+"_"+strconv.Itoa(i)),
			os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0660)
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

// high and low specify how many milliseconds between messages
func RateLoadTest(hpn, bf int) []T {
	return []T{
		{DefaultMachs, hpn, bf, 5000, DefaultRounds, 0, 0, 0, false, "stamp"}, // never send a message
		{DefaultMachs, hpn, bf, 5000, DefaultRounds, 0, 0, 0, false, "stamp"}, // one per round
		{DefaultMachs, hpn, bf, 500, DefaultRounds, 0, 0, 0, false, "stamp"},  // 10 per round
		{DefaultMachs, hpn, bf, 50, DefaultRounds, 0, 0, 0, false, "stamp"},   // 100 per round
		{DefaultMachs, hpn, bf, 30, DefaultRounds, 0, 0, 0, false, "stamp"},   // 1000 per round
	}
}

func DepthTest(hpn, low, high, step int) []T {
	ts := make([]T, 0)
	for bf := low; bf <= high; bf += step {
		ts = append(ts, T{DefaultMachs, hpn, bf, 10, DefaultRounds, 0, 0, 0, false, "stamp"})
	}
	return ts
}

func DepthTestFixed(hpn int) []T {
	return []T{
		{DefaultMachs, hpn, 1, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 2, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 4, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 8, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 16, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 32, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 64, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 128, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 256, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
		{DefaultMachs, hpn, 512, 30, DefaultRounds, 0, 0, 0, false, "stamp"},
	}
}

func ScaleTest(bf, low, high, mult int) []T {
	ts := make([]T, 0)
	for hpn := low; hpn <= high; hpn *= mult {
		ts = append(ts, T{DefaultMachs, hpn, bf, 10, DefaultRounds, 0, 0, 0, false, "stamp"})
	}
	return ts
}

// nmachs=32, hpn=128, bf=16, rate=500, failures=20, root failures, failures
var FailureTests = []T{
	{DefaultMachs, 64, 16, 30, 50, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 0, 5, 0, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 0, 10, 0, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 5, 0, 5, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 5, 0, 10, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 5, 0, 10, true, "stamp"},
}

var VotingTest = []T{
	{DefaultMachs, 64, 16, 30, 50, 0, 0, 0, true, "stamp"},
	{DefaultMachs, 64, 16, 30, 50, 0, 0, 0, false, "stamp"},
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
	{DefaultMachs, 1, 2, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 2, 3, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 4, 3, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 8, 8, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 16, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 32, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 64, 16, 30, 20, 0, 0, 0, false, "stamp"},
	{DefaultMachs, 128, 16, 30, 50, 0, 0, 0, false, "stamp"},
}

var VTest = []T{
	{DefaultMachs, 1, 3, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 2, 4, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 4, 6, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 8, 8, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 16, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 32, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 64, 16, 10000000, 20, 0, 0, 0, false, "vote"},
	{DefaultMachs, 128, 16, 10000000, 20, 0, 0, 0, false, "vote"},
}

func main() {
	SetDebug(false)
	flag.Parse()
	user = fmt.Sprintf("-user=%s", user)
	// view = true
	os.Chdir("deploy2deter")

	MkTestDir()

	err := exec.Command("go", "build", "-v").Run()
	if err != nil {
		log.Fatalln("error building deploy2deter:", err)
	}
	log.Println("KILLING REMAINING PROCESSES")
	build := "-build=true"
	if nobuild {
		build = "-build=false"
	}
	log.Println("Building is ", build)

	cmd := exec.Command("./deploy2deter", "-kill=true", build,
		fmt.Sprintf("-nmachs=%d", DefaultMachs), user, host)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// test the testing framework
	//RunTests("vote_test_no_signing.csv", VTest)
	DefaultRounds = 5
	// t := FailureTests
	// RunTests("failure_test.csv", t)
	// RunTests("vote_test", VotingTest)
	// RunTests("failure_test", FailureTests)
	RunTests("host_test", HostsTest)
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

func MkTestDir() {
	err := os.MkdirAll("test_data/", 0777)
	if err != nil {
		log.Fatal("failed to make test directory")
	}
}

func TestFile(name string) string {
	return "test_data/" + name + ".csv"
}

func SetDebug(b bool) {
	if b {
		debug = "-debug=true"
	} else {
		debug = "-debug=false"
	}
}

func isZero(f float64) bool {
	return math.Abs(f) < 0.0000001
}

/*
* Write the hosts.txt file automatically
* from project name and number of servers
 */
func GenerateHostsFile(project string, num_servers int) error {

	// open and erase file if needed
	if _, err1 := os.Stat(hosts_file); err1 == nil {
		log.Print(fmt.Sprintf("Hosts file %s already exists. Erasing ...", hosts_file))
		os.Remove(hosts_file)
	}
	// create the file
	f, err := os.Create(hosts_file)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not create hosts file description %s ><", hosts_file))
		return err
	}
	defer f.Close()

	// write the name of the server + \t + IP address
	ip := "10.255.0."
	name := "SAFER.isi.deterlab.net"
	for i := 1; i <= num_servers; i++ {
		f.WriteString(fmt.Sprintf("server-%d.%s.%s\t%s%d\n", i-1, project, name, ip, i))
	}
	log.Print(fmt.Sprintf("Created hosts file description (%d hosts)", num_servers))
	return err

}
