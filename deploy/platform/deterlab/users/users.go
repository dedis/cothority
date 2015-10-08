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
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/deploy/platform"
	"os"
	"os/exec"
	"fmt"
	"strconv"
	log "github.com/Sirupsen/logrus"
)

var deterlab platform.Deterlab
var kill = false

func init() {
	flag.BoolVar(&kill, "kill", false, "kill everything (and don't start anything)")
}

func main() {
	deterlab.ReadConfig()
	flag.Parse()

	vpmap := make(map[string]string)
	for i := range deterlab.Virt {
		vpmap[deterlab.Virt[i]] = deterlab.Phys[i]
	}
	// kill old processes
	var wg sync.WaitGroup
	doneHosts := make([]bool, len(deterlab.Phys))
	for i, h := range deterlab.Phys {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			dbg.Lvl4("Cleaning up host", h)
			cliutils.SshRun("", h, "sudo killall " + deterlab.App + " forkexec logserver timeclient scp ssh 2>/dev/null >/dev/null")
			time.Sleep(1 * time.Second)
			cliutils.SshRun("", h, "sudo killall " + deterlab.App + " 2>/dev/null >/dev/null")
			if dbg.DebugVisible > 3 {
				dbg.Lvl4("Cleaning report:")
				cliutils.SshRunStdout("", h, "ps aux")
			}

			if !kill {
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
	case <-time.After(time.Second * 10):
		for i, m := range doneHosts {
			if !m {
				dbg.Lvl1("Missing host:", deterlab.Phys[i])
				// expinfo gets a list of all mappings from physical to logical
				// node names. Then we grep the missing host and keep only
				// the logical node
				grep := "grep '" + strings.Split(deterlab.Phys[i], ".")[0] + " ' | sed -e 's/.* //'"
				cmd := fmt.Sprintf("expinfo -e %s,%s -m | %s",
					deterlab.Project, deterlab.Experiment, grep)
				info, _ := exec.Command("bash", "-c", cmd).Output()
				dbg.Lvl1("You might want to run\nnode_reboot", string(info), cmd)
			}
		}
		dbg.Fatal("Didn't receive all replies while cleaning up - aborting.")
	}

	if kill {
		dbg.Lvl1("Returning only from cleanup")
		return
	}

	nloggers := deterlab.Loggers
	masterLogger := deterlab.Phys[0]
	loggers := []string{masterLogger}
	dbg.Lvl3("Going to create", nloggers, "loggers")
	for n := 1; n < nloggers; n++ {
		loggers = append(loggers, deterlab.Phys[n])
	}

	hostnames := deterlab.Hostnames
	dbg.Lvl4("hostnames:", hostnames)

	// mapping from physical node name to the timestamp servers that are running there
	// essentially a reverse mapping of vpmap except ports are also used
	physToServer := make(map[string][]string)
	for _, virt := range hostnames {
		v, _, _ := net.SplitHostPort(virt)
		p := vpmap[v]
		ss := physToServer[p]
		ss = append(ss, virt)
		physToServer[p] = ss
	}


	// start up the logging server on the final host at port 10000
	dbg.Lvl1("starting up logservers: ", loggers)
	// start up the master logger
	loggerports := make([]string, len(loggers))
	for i, logger := range loggers {
		loggerport := logger + ":10000"
		loggerports[i] = loggerport
		// redirect to the master logger
		master := masterLogger + ":10000"
		// if this is the master logger than don't set the master to anything
		if loggerport == masterLogger + ":10000" {
			master = ""
		}

		dbg.Lvl3("Logger:", logger)
		go cliutils.SshRunStdout("", logger, "cd remote; sudo ./logserver -addr=" + loggerport +
		" -master=" + master)
	}

	i := 0
	// For coll_stamp we have to wait for everything in place which takes quite some time
	// We set up a directory and every host writes a file once he's ready to listen
	// When everybody is ready, the directory is deleted and the test starts
	coll_stamp_dir := "coll_stamp_up"
	if deterlab.App == "stamp" || deterlab.App == "sign" {
		os.RemoveAll(coll_stamp_dir)
		os.MkdirAll(coll_stamp_dir, 0777)
		time.Sleep(time.Second)
	}

	servers := len(physToServer)
	hpn := len(deterlab.Hostnames) / servers
	dbg.Lvl1("starting", servers, "forkexecs with", hpn, "processes each =", servers * hpn)
	totalServers := 0
	for phys, virts := range physToServer {
		if len(virts) == 0 {
			continue
		}
		totalServers += len(virts)
		dbg.Lvl1("Launching forkexec for", len(virts), "clients on", phys)
		i = (i + 1) % len(loggerports)
		wg.Add(1)
		go func(phys string) {
			//dbg.Lvl4("running on ", phys, cmd)
			defer wg.Done()
			dbg.Lvl4("Starting servers on physical machine ", phys)
			err := cliutils.SshRunStdout("", phys, "cd remote; sudo ./forkexec" +
			" -physaddr=" + phys + " -logger=" + loggerports[i])
			if err != nil {
				log.Fatal("Error starting timestamper:", err, phys)
			}
			dbg.Lvl4("Finished with Timestamper", phys)
		}(phys)
	}

	if deterlab.App == "stamp" || deterlab.App == "sign" {
		// Every stampserver that started up (mostly waiting for configuration-reading)
		// writes its name in coll_stamp_dir - once everybody is there, the directory
		// is cleaned to flag it's OK to go on.
		start_config := time.Now()
		for {
			files, err := ioutil.ReadDir(coll_stamp_dir)
			if err != nil {
				log.Fatal("Couldn't read directory", coll_stamp_dir, err)
			} else {
				dbg.Lvl1("Stampservers started:", len(files), "/", totalServers, "after", time.Since(start_config))
				if len(files) == totalServers {
					os.RemoveAll(coll_stamp_dir)
					// 1st second for everybody to see the deleted directory
					// 2nd second for everybody to start up listening
					time.Sleep(2 * time.Second)
					break
				}
			}
			time.Sleep(time.Second)
		}
	}

	switch deterlab.App{
	case "stamp":
		dbg.Lvl1("starting", len(physToServer), "time clients")
		// start up one timeclient per physical machine
		// it requests timestamps from all the servers on that machine
		amroot := true
		for p, ss := range physToServer {
			if len(ss) == 0 {
				dbg.Lvl3("ss is empty - not starting")
				continue
			}
			servers := strings.Join(ss, ",")
			dbg.Lvl3("Starting with ss=", ss)
			go func(i int, p string, a bool) {
				cmdstr := "cd remote; sudo ./" + deterlab.App + " -mode=client " +
				" -name=client@" + p +
				" -server=" + servers +
				" -amroot=" + strconv.FormatBool(a) +
				" -logger=" + loggerports[i]
				dbg.Print(cmdstr)
				err := cliutils.SshRunStdout("", p, cmdstr)
				if err != nil {
					dbg.Lvl4("Deter.go : error for", deterlab.App, err)
				}
				dbg.Lvl4("Deter.go : Finished with", deterlab.App, p)
			}(i, p, amroot)
			amroot = false
			i = (i + 1) % len(loggerports)
		}
	case "sign_no":
		// TODO: for now it's only a simple startup from the server
		dbg.Lvl1("Starting only one client")
	}

	// wait for the servers to finish before stopping
	wg.Wait()
}
