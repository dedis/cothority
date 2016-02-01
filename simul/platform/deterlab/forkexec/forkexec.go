package main

import (
	"flag"
	"os/exec"
	"strconv"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/simul/platform"
	"os"
	"strings"
	"sync"
)

// Wrapper around app to enable measuring of cpu time

func main() {
	var deter platform.Deterlab
	deter.ReadConfig()
	// The flags are defined in lib/app
	var ourIp string
	flag.StringVar(&ourIp, "internal", "", "the internal IP for our host")
	flag.Parse()
	simulConfig, err := sda.LoadSimulationConfig(".", "")
	if err != nil {
		dbg.Fatal("Couldn't load config:", err)
	}

	var wg sync.WaitGroup
	monitorAddr := deter.MonitorAddress + ":" + strconv.Itoa(monitor.SinkPort)
	ourIp += ":"
	rootname := simulConfig.Tree.Root.Entity.Addresses[0]
	for _, name := range simulConfig.EntityList.List {
		dbg.Lvl3("Comparing", name.Addresses[0], "with", ourIp)
		if strings.Contains(name.Addresses[0], ourIp) {
			dbg.Lvl3("Starting", name, "on", ourIp)
			wg.Add(1)
			go func(nameport string) {
				dbg.Lvl3("Running on", ourIp, "starting", nameport, rootname)
				defer wg.Done()

				amroot := nameport == rootname
				args := []string{
					"-address=" + nameport,
					"-simul=" + deter.Simulation,
					"-start=" + strconv.FormatBool(amroot),
					"-monitor=" + monitorAddr,
				}

				dbg.Lvl3("Starting on", ourIp, "with args", args)
				cmdApp := exec.Command("./"+deter.Simulation, args...)
				cmdApp.Stdout = os.Stdout
				cmdApp.Stderr = os.Stderr
				err := cmdApp.Run()
				if err != nil {
					dbg.Lvl1("cmd run:", err)
				}

				dbg.Lvl3("Finished with app", ourIp)
			}(name.Addresses[0])
		}
	}
	dbg.Lvl3(ourIp, "Finished starting apps")
	wg.Wait()
	dbg.Lvl2(ourIp, "forkexec exited")
}
