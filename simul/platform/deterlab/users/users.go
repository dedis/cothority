// This is run on the users.deterlab.net server and will clean up the
// servers and then run 'cothority' on every server.
package main

import (
	"flag"
	"strings"
	"sync"
	"time"

	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/simul/platform"
)

var kill = false

func init() {
	flag.BoolVar(&kill, "kill", false, "kill everything (and don't start anything)")
}

func main() {
	// init with deter.toml
	deter := deterFromConfig()
	flag.Parse()

	// kill old processes
	var wg sync.WaitGroup
	re := regexp.MustCompile(" +")
	hosts, err := exec.Command("/usr/testbed/bin/node_list", "-e", deter.Project+","+deter.Experiment).Output()
	if err != nil {
		dbg.Fatal("Deterlab experiment", deter.Project+"/"+deter.Experiment, "seems not to be swapped in. Aborting.")
		os.Exit(-1)
	}
	hostsTrimmed := strings.TrimSpace(re.ReplaceAllString(string(hosts), " "))
	hostlist := strings.Split(hostsTrimmed, " ")
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
				runSSH(h, "sudo killall -9 cothority scp 2>/dev/null >/dev/null")
				time.Sleep(1 * time.Second)
				runSSH(h, "sudo killall -9 cothority 2>/dev/null >/dev/null")
				time.Sleep(1 * time.Second)
				// Also kill all other process that start with "./" and are probably
				// locally started processes
				runSSH(h, "sudo pkill -9 -f '\\./'")
				time.Sleep(1 * time.Second)
				if dbg.DebugVisible() > 3 {
					dbg.Lvl4("Cleaning report:")
					_ = platform.SSHRunStdout("", h, "ps aux")
				}
			} else {
				dbg.Lvl3("Setting the file-limit higher on", h)

				// Copy configuration file to make higher file-limits
				err := platform.SSHRunStdout("", h, "sudo cp remote/cothority.conf /etc/security/limits.d")
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
	proxyAddress := deter.ProxyAddress + ":" + strconv.Itoa(deter.MonitorPort+1)
	dbg.Lvl2("Launching proxy redirecting to", proxyAddress)
	err = monitor.Proxy(proxyAddress)
	if err != nil {
		dbg.Fatal("Couldn't start proxy:", err)
	}

	dbg.Lvl1("starting", deter.Servers, "cothorities for a total of", deter.Hosts, "processes.")
	killing := false
	for i, phys := range deter.Phys {
		dbg.Lvl2("Launching cothority on", phys)
		wg.Add(1)
		go func(phys, internal string) {
			//dbg.Lvl4("running on", phys, cmd)
			defer wg.Done()
			monitorAddr := deter.MonitorAddress + ":" + strconv.Itoa(deter.MonitorPort)
			dbg.Lvl4("Starting servers on physical machine ", internal, "with monitor = ",
				monitorAddr)
			args := " -address=" + internal +
				" -simul=" + deter.Simulation +
				" -monitor=" + monitorAddr +
				" -debug=" + strconv.Itoa(dbg.DebugVisible())
			dbg.Lvl3("Args is", args)
			err := platform.SSHRunStdout("", phys, "cd remote; sudo ./cothority "+
				args)
			if err != nil && !killing {
				dbg.Lvl1("Error starting cothority - will kill all others:", err, internal)
				killing = true
				err := exec.Command("killall", "ssh").Run()
				if err != nil {
					dbg.Fatal("Couldn't killall ssh:", err)
				}
			}
			dbg.Lvl4("Finished with cothority on", internal)
		}(phys, deter.Virt[i])
	}

	// wait for the servers to finish before stopping
	wg.Wait()
}

// Reads in the deterlab-config and drops out if there is an error
func deterFromConfig(name ...string) *platform.Deterlab {
	d := &platform.Deterlab{}
	configName := "deter.toml"
	if len(name) > 0 {
		configName = name[0]
	}
	err := sda.ReadTomlConfig(d, configName)
	_, caller, line, _ := runtime.Caller(1)
	who := caller + ":" + strconv.Itoa(line)
	if err != nil {
		dbg.Fatal("Couldn't read config in", who, ":", err)
	}
	dbg.SetDebugVisible(d.Debug)
	return d
}

// Runs a command on the remote host and outputs an eventual error if debug level >= 3
func runSSH(host, cmd string) {
	if _, err := platform.SSHRun("", host, cmd); err != nil {
		dbg.Lvlf3("Host %s got error %s while running [%s]", host, err.Error(), cmd)
	}
}
