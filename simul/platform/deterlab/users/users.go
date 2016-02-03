// deter is the deterlab process that should run on the boss node
//
// It spawns multiple timestampers and clients, while constructing
// the topology defined on tree.json. It assumes that hosts.txt has
// the entire list of hosts to run timestampers on and that the final
// host is the designated logging server.
//
// The overall topology that is created is defined by tree.json.
// The port layout for each node, however, is specified here.
// tree.json will assign each node a port p. This is the port
// that each singing node is listening on. The timestamp server
// to which clients connect is listneing on port p+1. And the
// pprof server for each node is listening on port p+2. This
// means that in order to debug each client, you can forward
// the p+2 port of each node to your localhost.
//
// In the future the loggingserver will be connecting to the
// servers on the pprof port in order to gather extra data.
package main

import (
	"flag"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/simul/platform"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

var deter platform.Deterlab
var kill = false

func init() {
	flag.BoolVar(&kill, "kill", false, "kill everything (and don't start anything)")
}

func main() {
	deter.ReadConfig()
	flag.Parse()

	// kill old processes
	var wg sync.WaitGroup
	re := regexp.MustCompile(" +")
	hosts, err := exec.Command("/usr/testbed/bin/node_list", "-e", deter.Project+","+deter.Experiment).Output()
	if err != nil {
		dbg.Fatal("Deterlab experiment", deter.Project+"/"+deter.Experiment, "seems not to be swapped in. Aborting.")
		os.Exit(-1)
	}
	hosts_trimmed := strings.TrimSpace(re.ReplaceAllString(string(hosts), " "))
	hostlist := strings.Split(hosts_trimmed, " ")
	doneHosts := make([]bool, len(hostlist))
	dbg.Lvl2("Found the following hosts:", hostlist)
	if kill {
		dbg.Lvl1("Cleaning up", len(hostlist), "hosts.")
	}
	for i, h := range hostlist {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			if kill {
				dbg.Lvl3("Cleaning up host", h, ".")
				cliutils.SshRun("", h, "sudo killall -9 cothority scp 2>/dev/null >/dev/null")
				time.Sleep(1 * time.Second)
				cliutils.SshRun("", h, "sudo killall -9 cothority 2>/dev/null >/dev/null")
				time.Sleep(1 * time.Second)
				// Also kill all other process that start with "./" and are probably
				// locally started processes
				cliutils.SshRun("", h, "sudo pkill -9 -f '\\./'")
				time.Sleep(1 * time.Second)
				if dbg.DebugVisible > 3 {
					dbg.Lvl4("Cleaning report:")
					cliutils.SshRunStdout("", h, "ps aux")
				}
			} else {
				dbg.Lvl3("Setting the file-limit higher on", h)

				// Copy configuration file to make higher file-limits
				err := cliutils.SshRunStdout("", h, "sudo cp remote/cothority.conf /etc/security/limits.d")
				if err != nil {
					dbg.Fatal("Couldn't copy limit-file:", err)
				}
			}
			doneHosts[i] = true
			dbg.Lvl3("Host", h, "cleaned up")
		}(i, h)
	}

	cleanupChannel := make(chan string)
	go func() {
		wg.Wait()
		dbg.Lvl3("Done waiting")
		cleanupChannel <- "done"
	}()
	select {
	case msg := <-cleanupChannel:
		dbg.Lvl3("Received msg from cleanupChannel", msg)
	case <-time.After(time.Second * 20):
		for i, m := range doneHosts {
			if !m {
				dbg.Lvl1("Missing host:", hostlist[i], "- You should run")
				dbg.Lvl1("/usr/testbed/bin/node_reboot", hostlist[i])
			}
		}
		dbg.Fatal("Didn't receive all replies while cleaning up - aborting.")
	}

	if kill {
		dbg.Lvl2("Only cleaning up - returning")
		return
	}

	// ADDITIONS : the monitoring part
	// Proxy will listen on Sink:SinkPort and redirect every packet to
	// RedirectionAddress:SinkPort-1. With remote tunnel forwarding it will
	// be forwarded to the real sink
	proxyAddress := deter.ProxyAddress + ":" + strconv.Itoa(monitor.SinkPort+1)
	dbg.Lvl2("Launching proxy redirecting to", proxyAddress)
	err = monitor.Proxy(proxyAddress)
	if err != nil {
		dbg.Fatal("Couldn't start proxy:", err)
	}

	dbg.Lvl1("starting", deter.Servers, "cothorities for a total of", deter.Hosts, "processes.")
	for i, phys := range deter.Phys {
		dbg.Lvl2("Launching cothority on", phys)
		wg.Add(1)
		go func(phys, internal string) {
			//dbg.Lvl4("running on", phys, cmd)
			defer wg.Done()
			monitorAddr := deter.MonitorAddress + ":" + strconv.Itoa(monitor.SinkPort)
			dbg.Lvl4("Starting servers on physical machine ", internal, "with monitor = ",
				monitorAddr)
			args := " -address=" + internal +
				" -simul=" + deter.Simulation +
				" -monitor=" + monitorAddr +
				" -debug=" + strconv.Itoa(dbg.DebugVisible)
			dbg.Lvl3("Args is", args)
			err := cliutils.SshRunStdout("", phys, "cd remote; sudo ./cothority "+
				args)
			if err != nil {
				dbg.Lvl1("Error starting cothority:", err, internal)
			}
			dbg.Lvl4("Finished with cothority on", internal)
		}(phys, deter.Virt[i])
	}

	// wait for the servers to finish before stopping
	wg.Wait()
}
