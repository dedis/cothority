package main

import (
	"flag"
	"os/exec"
	"strconv"

	"github.com/dedis/cothority/deploy/platform"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"net"
	"os"
	"sync"
)

// Wrapper around app to enable measuring of cpu time

var deter platform.Deterlab
var testConnect bool
var physToServer map[string][]string
var rootname string

func main() {
	deter.ReadConfig()
	// The flags are defined in lib/app
	app.FlagInit()
	flag.Parse()

	setup_deter()

	var wg sync.WaitGroup
	virts := physToServer[app.RunFlags.PhysAddr]
	if len(virts) > 0 {
		dbg.Lvl3("starting", len(virts), "servers of", deter.App, "on", virts)
		for _, name := range virts {
			dbg.Lvl3("Starting", name, "on", app.RunFlags.PhysAddr)
			wg.Add(1)
			go func(nameport string) {
				dbg.Lvl3("Running on", app.RunFlags.PhysAddr, "starting", nameport, rootname)
				defer wg.Done()

				amroot := nameport == rootname
				args := []string{
					"-hostname=" + nameport,
					"-physaddr=" + app.RunFlags.PhysAddr,
					"-amroot=" + strconv.FormatBool(amroot),
					"-test_connect=" + strconv.FormatBool(testConnect),
					"-logger=" + app.RunFlags.Logger,
					"-mode=server",
				}

				dbg.Lvl3("Starting on", app.RunFlags.PhysAddr, "with args", args)
				cmdApp := exec.Command("./"+deter.App, args...)
				cmdApp.Stdout = os.Stdout
				cmdApp.Stderr = os.Stderr
				err := cmdApp.Run()
				if err != nil {
					dbg.Lvl1("cmd run:", err)
				}

				dbg.Lvl3("Finished with app", app.RunFlags.PhysAddr)
			}(name)
		}
		dbg.Lvl3(app.RunFlags.PhysAddr, "Finished starting apps")
		wg.Wait()
	} else {
		dbg.Lvl2("No apps for", app.RunFlags.PhysAddr)
	}
	dbg.Lvl2(app.RunFlags.PhysAddr, "forkexec exited")
}

func setup_deter() {
	vpmap := make(map[string]string)
	for i := range deter.Virt {
		vpmap[deter.Virt[i]] = deter.Phys[i]
	}

	deter.Phys = deter.Phys[:]
	deter.Virt = deter.Virt[:]

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

}
