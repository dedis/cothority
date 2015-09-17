package main

import (
	"flag"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"os"
"github.com/dedis/cothority/deploy"
)

// Wrapper around exec.go to enable measuring of cpu time

var deter *deploy.Deter
var conf *deploy.Config
var hostname string
var logger string
var physaddr string
var amroot bool
var testConnect bool

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&amroot, "amroot", false, "am I root")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
}

func main() {
	deter, err := deploy.ReadConfig()
	if err != nil {
		log.Fatal("Couldn't load config-file in forkexec:", err)
	}
	conf = deter.Config
	dbg.DebugVisible = conf.Debug

	flag.Parse()

	// connect with the logging server
	if logger != "" && (amroot || conf.Debug > 0) {
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(logger, hostname, conf.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
	}

	args := []string{
		"-hostname=" + hostname,
		"-logger=" + logger,
		"-physaddr=" + physaddr,
		"-amroot=" + strconv.FormatBool(amroot),
		"-test_connect=" + strconv.FormatBool(testConnect),
	}
	cmd := exec.Command("./exec", args...)
	//cmd.Stdout = log.StandardLogger().Writer()
	//cmd.Stderr = log.StandardLogger().Writer()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	dbg.Lvl2("fork-exec is running command:", cmd)
	err = cmd.Run()
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
