package main

import (
	"flag"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"os"
	"github.com/dedis/cothority/lib/cliutils"
	"net"
	"sync"
	"github.com/dedis/cothority/deploy/platform"
	"github.com/dedis/cothority/lib/app"
)

// Wrapper around app to enable measuring of cpu time

var deter platform.Deterlab
var testConnect bool

func main() {
	deter.ReadConfig()
	// The flags are defined in lib/app
	flag.Parse()

	// connect with the logging server
	if app.Flags.Logger != "" {
		dbg.Lvl4("Setting up logger at", app.Flags.Logger)
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(app.Flags.Logger, app.Flags.PhysAddr, deter.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
	}

	setup_deter()

	i := 0
	var wg sync.WaitGroup
	virts := physToServer[app.Flags.PhysAddr]
	if len(virts) > 0 {
		dbg.Lvl3("starting", len(virts), "servers of", deter.App, "on", virts)
		i = (i + 1) % len(loggerports)
		for _, name := range virts {
			dbg.Lvl3("Starting", name, "on", app.Flags.PhysAddr)
			wg.Add(1)
			go func(nameport string) {
				dbg.Lvl3("Running on", app.Flags.PhysAddr, "starting", nameport)
				defer wg.Done()

				args := []string{
					"-hostname=" + nameport,
					"-logger=" + app.Flags.Logger,
					"-physaddr=" + app.Flags.PhysAddr,
					"-amroot=" + strconv.FormatBool(nameport == rootname),
					"-test_connect=" + strconv.FormatBool(testConnect),
					"-mode=server",
				}

				dbg.Lvl3("Starting on", app.Flags.PhysAddr, "with args", args)
				cmdApp := exec.Command("./" + deter.App, args...)
				//cmd.Stdout = log.StandardLogger().Writer()
				//cmd.Stderr = log.StandardLogger().Writer()
				cmdApp.Stdout = os.Stdout
				cmdApp.Stderr = os.Stderr
				dbg.Lvl3("fork-exec is running command:", args)
				err := cmdApp.Run()
				if err != nil {
					dbg.Lvl1("cmd run:", err)
				}

				// get CPU usage stats
				st := cmdApp.ProcessState.SystemTime()
				ut := cmdApp.ProcessState.UserTime()
				log.WithFields(log.Fields{
					"file":     logutils.File(),
					"type":     "forkexec",
					"systime":  st,
					"usertime": ut,
				}).Info("")

				dbg.Lvl2("Finished with Timestamper", app.Flags.PhysAddr)
			}(name)
		}
		dbg.Lvl3(app.Flags.PhysAddr, "Finished starting timestampers")
		wg.Wait()
	} else {
		dbg.Lvl2("No timestampers for", app.Flags.PhysAddr)
	}
	dbg.Lvl2(app.Flags.PhysAddr, "timestampers exited")
}

var physToServer map[string][]string
var loggerports []string
var rootname string

func setup_deter() {
	virt, err := cliutils.ReadLines("virt.txt")
	if err != nil {
		log.Fatal(err)
	}
	phys, err := cliutils.ReadLines("phys.txt")
	if err != nil {
		log.Fatal(err)
	}
	vpmap := make(map[string]string)
	for i := range virt {
		vpmap[virt[i]] = phys[i]
	}
	nloggers := deter.Loggers
	masterLogger := phys[0]
	loggers := []string{masterLogger}
	for n := 1; n <= nloggers; n++ {
		loggers = append(loggers, phys[n])
	}

	phys = phys[nloggers:]
	virt = virt[nloggers:]

	hostnames := deter.Hostnames
	dbg.Lvl4("hostnames:", hostnames)

	rootname = hostnames[0]

	// mapping from physical node name to the timestamp servers that are running there
	// essentially a reverse mapping of vpmap except ports are also used
	physToServer = make(map[string][]string)
	for _, virt := range hostnames {
		v, _, _ := net.SplitHostPort(virt)
		p := vpmap[v]
		ss := physToServer[p]
		ss = append(ss, virt)
		physToServer[p] = ss
	}
	dbg.Lvl3("PhysToServer is", physToServer)

	loggerports = make([]string, len(loggers))
	for i, logger := range loggers {
		loggerports[i] = logger + ":10000"
	}

}