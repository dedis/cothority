package main

import (
	"flag"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/ineiti/cothorities/helpers/logutils"
	"os"
)

// Wrapper around exec.go to enable measuring of cpu time

var hostname string
var configFile string
var logger string
var app string
var pprofaddr string
var physaddr string
var rootwait int
var debug bool
var failures int
var rFail int
var fFail int
var rounds int
var amroot bool
var testConnect bool
var suite string

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&configFile, "config", "cfg.json", "the json configuration file")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&app, "app", "stamp", "application to run [sign|stamp|vote]")
	flag.IntVar(&rounds, "rounds", 100, "number of rounds to run")
	flag.StringVar(&pprofaddr, "pprof", ":10000", "the address to run the pprof server at")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.IntVar(&rootwait, "rootwait", 30, "the amount of time the root should wait")
	flag.BoolVar(&debug, "debug", false, "set debugging")
	flag.IntVar(&failures, "failures", 0, "percent showing per node probability of failure")
	flag.IntVar(&rFail, "rfail", 0, "number of consecutive rounds each root runs before it fails")
	flag.IntVar(&fFail, "ffail", 0, "number of consecutive rounds each follower runs before it fails")
	flag.BoolVar(&amroot, "amroot", false, "am I root")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&suite, "suite", "nist256", "abstract suite to use [nist256, nist512, ed25519]")
}

func main() {
	flag.Parse()
	// connect with the logging server
	if logger != "" && (amroot || debug) {
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(logger, hostname, app)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("ERROR SETTING UP LOGGING SERVER:", err)
		}
		log.AddHook(lh)
	}
	//log.Println("IN FORKEXEC")
	////log.SetOutput(ioutil.Discard)
	////log.Println("Log Test")
	////fmt.Println("exiting logger block")
	//}
	// log.Println("IN FORK EXEC")
	// recombine the flags for exec to use
	args := []string{
		"-failures=" + strconv.Itoa(failures),
		"-rfail=" + strconv.Itoa(rFail),
		"-ffail=" + strconv.Itoa(fFail),
		"-hostname=" + hostname,
		"-config=" + configFile,
		"-logger=" + logger,
		"-app=" + app,
		"-pprof=" + pprofaddr,
		"-physaddr=" + physaddr,
		"-rootwait=" + strconv.Itoa(rootwait),
		"-debug=" + strconv.FormatBool(debug),
		"-rounds=" + strconv.Itoa(rounds),
		"-amroot=" + strconv.FormatBool(amroot),
		"-test_connect=" + strconv.FormatBool(testConnect),
		"-suite=" + suite,
	}
	cmd := exec.Command("./exec", args...)
	//cmd.Stdout = log.StandardLogger().Writer()
	//cmd.Stderr = log.StandardLogger().Writer()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("running command:", cmd)
	err := cmd.Run()
	if err != nil {
		log.Errorln("cmd run:", err)
	}

	// get CPU usage stats
	st := cmd.ProcessState.SystemTime()
	ut := cmd.ProcessState.UserTime()
	log.WithFields(log.Fields{
		"file":     logutils.File(),
		"type":     "forkexec",
		"systime":  st,
		"usertime": ut,
	}).Info("")

}
