package main

import (
	"flag"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/deploy/platform"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"net"
	"os"
	"sync"
)

// Wrapper around app to enable measuring of cpu time

var deter platform.Deterlab
var testConnect bool
var physToServer map[string][]string
var loggerports []string
var rootname string

func main() {
	deter.ReadConfig()
	// The flags are defined in lib/app
	app.FlagInit()
	flag.Parse()

	setup_deter()
	app.ConnectLogservers()

	i := 0
	var wg sync.WaitGroup
	virts := physToServer[app.RunFlags.PhysAddr]
	if len(virts) > 0 {
		dbg.Lvl3("starting", len(virts), "servers of", deter.App, "on", virts)
		i = (i + 1) % len(loggerports)
		for _, name := range virts {
			dbg.Lvl3("Starting", name, "on", app.RunFlags.PhysAddr)
			wg.Add(1)
			go func(nameport string) {
				dbg.Lvl3("Running on", app.RunFlags.PhysAddr, "starting", nameport)
				defer wg.Done()

				amroot := nameport == rootname
				args := []string{
					"-hostname=" + nameport,
					"-logger=" + app.RunFlags.Logger,
					"-physaddr=" + app.RunFlags.PhysAddr,
					"-amroot=" + strconv.FormatBool(amroot),
					"-test_connect=" + strconv.FormatBool(testConnect),
					"-mode=server",
				}

				dbg.Lvl3("Starting on", app.RunFlags.PhysAddr, "with args", args)
				cmdApp := exec.Command("./"+deter.App, args...)
				//cmd.Stdout = log.StandardLogger().Writer()
				//cmd.Stderr = log.StandardLogger().Writer()
				cmdApp.Stdout = os.Stdout
				cmdApp.Stderr = os.Stderr
				dbg.Lvl3("fork-exec is running command:", args)
				err := cmdApp.Run()
				if err != nil {
					dbg.Lvl1("cmd run:", err)
				}

				if amroot {
					// get CPU usage stats, but only for root
					st := cmdApp.ProcessState.SystemTime()
					ut := cmdApp.ProcessState.UserTime()
					log.WithFields(log.Fields{
						"file":     logutils.File(),
						"type":     "forkexec",
						"systime":  st,
						"usertime": ut,
					}).Info("")
					log.WithField("type", "end").Info("")
				}

				dbg.Lvl2("Finished with app", app.RunFlags.PhysAddr)
			}(name)
		}
		dbg.Lvl3(app.RunFlags.PhysAddr, "Finished starting apps")
		wg.Wait()
	} else {
		dbg.Lvl2("No apps for", app.RunFlags.PhysAddr)
	}
	dbg.Lvl2(app.RunFlags.PhysAddr, "apps exited")
}

func setup_deter() {
	vpmap := make(map[string]string)
	for i := range deter.Virt {
		vpmap[deter.Virt[i]] = deter.Phys[i]
	}
	nloggers := deter.Loggers
	masterLogger := deter.Phys[0]
	loggers := []string{masterLogger}
	for n := 1; n <= nloggers; n++ {
		loggers = append(loggers, deter.Phys[n])
	}

	deter.Phys = deter.Phys[nloggers:]
	deter.Virt = deter.Virt[nloggers:]

	hostnames := deter.Hostnames
	dbg.Lvl4("hostnames:", hostnames)

	rootname = hostnames[0]

	// mapping from physical node name to the app servers that are running there
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
