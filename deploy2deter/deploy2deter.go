// deploy2deter is responsible for kicking off the deployment process
// for deterlab. Given a list of hostnames, it will create an overlay
// tree topology, using all but the last node. It will create multiple
// nodes per server and run timestamping processes. The last node is
// reserved for the logging server, which is forwarded to localhost:8080
//
// options are "bf" which specifies the branching factor
//
// 	and "hpn" which specifies the replicaiton factor: hosts per node
//
// Creates the following directory structure in remote:
// exec, timeclient, logserver/...,
// this way it can rsync the remove to each of the destinations
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/dedis/prifi/coco/test/cliutils"
	"github.com/dedis/prifi/coco/test/config"
	"github.com/dedis/prifi/coco/test/graphs"
)

// bf is the branching factor of the tree that we want to build
var bf int

// hpn is the replication factor of hosts per node: how many hosts do we want per node
var hpn int

var nmsgs int
var debug bool
var rate int
var failures int
var rFail int
var fFail int
var kill bool
var rounds int
var nmachs int
var testConnect bool
var app string
var suite string

func init() {
	flag.IntVar(&bf, "bf", 2, "branching factor: default binary")
	flag.IntVar(&hpn, "hpn", 1, "hosts per node: default 1")
	flag.IntVar(&nmsgs, "nmsgs", 100, "number of messages per round")
	flag.IntVar(&rate, "rate", -1, "number of milliseconds between messages: if rate > 0 then used")
	flag.BoolVar(&debug, "debug", false, "run in debugging mode")
	flag.IntVar(&failures, "failures", 0, "percent showing per node probability of failure")
	flag.IntVar(&rFail, "rfail", 0, "number of consecutive rounds each root runs before it fails")
	flag.IntVar(&fFail, "ffail", 0, "number of consecutive rounds each follower runs before it fails")
	flag.IntVar(&rounds, "rounds", 100, "number of rounds to run for")
	flag.BoolVar(&kill, "kill", false, "kill all running processes (but don't start anything)")
	flag.IntVar(&nmachs, "nmachs", 14, "number of machines to use")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&app, "app", "stamp", "app to run")
	flag.StringVar(&suite, "suite", "nist256", "abstract suite to use [nist256, nist512, ed25519]")
}

func main() {
	log.SetFlags(log.Lshortfile)
	flag.Parse()
	log.Println("RUNNING DEPLOY2DETER WITH RATE:", rate)
	os.MkdirAll("remote", 0777)
	var wg sync.WaitGroup
	// start building the necessary packages
	log.Println("Starting to build all executables")
	packages := []string{"../logserver", "../timeclient", "../exec", "../forkexec", "../deter"}
	for _, p := range packages {
		log.Println("Building ", p)
		wg.Add(1)
		if p == "../deter" {
			go func(p string) {
				defer wg.Done()
				// the users node has a 386 FreeBSD architecture
				err := cliutils.Build(p, "386", "freebsd")
				if err != nil {
					log.Fatal(err)
				}
			}(p)
			continue
		}
		go func(p string) {
			defer wg.Done()
			// deter has an amd64, linux architecture
			err := cliutils.Build(p, "amd64", "linux")
			if err != nil {
				log.Fatal(err)
			}
		}(p)
	}

	// killssh processes on users
	log.Println("Stopping programs on user.deterlab.net")
	cliutils.SshRunStdout("ineiti", "users.deterlab.net", "killall ssh scp deter 2>/dev/null 1>/dev/null")

	// parse the hosts.txt file to create a separate list (and file)
	// of physical nodes and virtual nodes. Such that each host on line i, in phys.txt
	// corresponds to each host on line i, in virt.txt.
	physVirt, err := cliutils.ReadLines("hosts.txt")
	log.Println("Getting list of hosts ", len(physVirt), ":", nmachs)

	phys := make([]string, 0, len(physVirt)/2)
	virt := make([]string, 0, len(physVirt)/2)
	for i := 0; i < len(physVirt); i += 2 {
		phys = append(phys, physVirt[i])
		virt = append(virt, physVirt[i+1])
	}
	nloggers := 3
	// only use the number of machines that we need
	if nmachs + nloggers > len(phys) {
		log.Fatal("Error, having only ", len(phys), " hosts while ", nmachs + nloggers, " are needed")
	}
	phys = phys[:nmachs+nloggers]
	virt = virt[:nmachs+nloggers]
	physOut := strings.Join(phys, "\n")
	virtOut := strings.Join(virt, "\n")

	// phys.txt and virt.txt only contain the number of machines that we need
	log.Println("Reading phys and virt")
	err = ioutil.WriteFile("remote/phys.txt", []byte(physOut), 0666)
	if err != nil {
		log.Fatal("failed to write physical nodes file", err)
	}

	err = ioutil.WriteFile("remote/virt.txt", []byte(virtOut), 0666)
	if err != nil {
		log.Fatal("failed to write virtual nodes file", err)
	}

	masterLogger := phys[0]
	// slaveLogger1 := phys[1]
	// slaveLogger2 := phys[2]
	virt = virt[3:]
	phys = phys[3:]
	t, hostnames, depth, err := graphs.TreeFromList(virt, hpn, bf)
	log.Println("DEPTH:", depth)
	log.Println("TOTAL HOSTS:", len(hostnames))

	// wait for the build to finish
	wg.Wait()
	log.Println("Build is finished")

	// copy the logserver directory to the current directory
	err = exec.Command("rsync", "-au", "../logserver", "remote/").Run()
	if err != nil {
		log.Fatal("error rsyncing logserver directory into remote directory:", err)
	}
	err = exec.Command("rsync", "-au", "remote/phys.txt", "remote/virt.txt", "remote/logserver/").Run()
	if err != nil {
		log.Fatal("error rsyncing phys, virt, and remote/logserver:", err)
	}
	err = os.Rename("logserver", "remote/logserver/logserver")
	if err != nil {
		log.Fatal("error renaming logserver:", err)
	}

	b, err := json.Marshal(t)
	if err != nil {
		log.Fatal("unable to generate tree from list")
	}
	err = ioutil.WriteFile("remote/logserver/cfg.json", b, 0660)
	if err != nil {
		log.Fatal("unable to write configuration file")
	}

	// NOTE: now remote/logserver is ready for transfer
	// it has logserver/ folder, binary, and cfg.json, and phys.txt, virt.txt

	// generate the configuration file from the tree
	cf := config.ConfigFromTree(t, hostnames)
	cfb, err := json.Marshal(cf)
	err = ioutil.WriteFile("remote/cfg.json", cfb, 0666)
	if err != nil {
		log.Fatal(err)
	}

	// scp the files that we need over to the boss node
	files := []string{"timeclient", "exec", "forkexec", "deter"}
	for _, f := range files {
		cmd := exec.Command("rsync", "-au", f, "remote/")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatal("error unable to rsync file into remote directory:", err)
		}
	}
	err = cliutils.Rsync("ineiti", "users.deterlab.net", "remote", "")
	if err != nil {
		log.Fatal(err)
	}
	killssh := exec.Command("pkill", "-f", "ssh -t -t")
	killssh.Stdout = os.Stdout
	killssh.Stderr = os.Stderr
	err = killssh.Run()
	if err != nil {
		log.Print(err)
	}

	// setup port forwarding for viewing log server
	// ssh -L 8080:pcXXX:80 username@users.isi.deterlab.net
	// ssh username@users.deterlab.net -L 8118:somenode.experiment.YourClass.isi.deterlab.net:80
	fmt.Println("setup port forwarding for master logger: ", masterLogger)
	cmd := exec.Command(
		"ssh",
		"-t",
		"-t",
		"ineiti@users.isi.deterlab.net",
		"-L",
		"8080:"+masterLogger+":10000")
	err = cmd.Start()
	if err != nil {
		log.Fatal("failed to setup portforwarding for logging server")
	}
	log.Println("runnning deter with nmsgs:", nmsgs)
	// run the deter lab boss nodes process
	// it will be responsible for forwarding the files and running the individual
	// timestamping servers
	log.Fatal(cliutils.SshRunStdout("ineiti", "users.isi.deterlab.net",
		"GOMAXPROCS=8 remote/deter -nmsgs="+strconv.Itoa(nmsgs)+
			" -hpn="+strconv.Itoa(hpn)+
			" -bf="+strconv.Itoa(bf)+
			" -rate="+strconv.Itoa(rate)+
			" -rounds="+strconv.Itoa(rounds)+
			" -debug="+strconv.FormatBool(debug)+
			" -failures="+strconv.Itoa(failures)+
			" -rfail="+strconv.Itoa(rFail)+
			" -ffail="+strconv.Itoa(fFail)+
			" -test_connect="+strconv.FormatBool(testConnect)+
			" -app="+app+
			" -suite="+suite+
			" -kill="+strconv.FormatBool(kill)))
}
