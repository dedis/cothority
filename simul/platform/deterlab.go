// Deterlab is responsible for setting up everything to test the application
// on deterlab.net
// Given a list of hostnames, it will create an overlay
// tree topology, using all but the last node. It will create multiple
// nodes per server and run timestamping processes. The last node is
// reserved for the logging server, which is forwarded to localhost:8081
//
// Creates the following directory structure:
// build/ - where all cross-compiled executables are stored
// remote/ - directory to be copied to the deterlab server
//
// The following apps are used:
//   deter - runs on the user-machine in deterlab and launches the others
//   forkexec - runs on the other servers and launches the app, so it can measure its cpu usage

package platform

import (
	"os"
	"os/exec"
	"strings"
	"sync"

	"bufio"
	_ "errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type Deterlab struct {
	// *** Deterlab-related configuration
	// The login on the platform
	Login string
	// The outside host on the platform
	Host string
	// The name of the project
	Project string
	// Name of the Experiment - also name of hosts
	Experiment string
	// Number of available servers
	Servers int

	// Name of the simulation
	Simulation string
	// Directory holding the cothority-go-file
	cothorityDir string
	// Directory where everything is copied into
	deployDir string
	// Directory for building
	buildDir string
	// Working directory of deterlab
	deterDir string
	// DNS-resolvable names
	Phys []string
	// VLAN-IP names (physical machines)
	Virt []string

	// ProxyAddress : the proxy will redirect every traffic it
	// receives to this address
	ProxyAddress string
	// MonitorAddress is the address given to clients to connect to the monitor
	// It is actually the Proxy that will listen to that address and clients
	// won't know a thing about it
	MonitorAddress string
	// Port number of the monitor and the proxy
	MonitorPort int

	// Number of machines
	Hosts int
	// Channel to communication stopping of experiment
	sshDeter chan string
	// Whether the simulation is started
	started bool
	// Debugging-level: 0 is none - 5 is everything
	Debug int
}

var simulConfig *sda.SimulationConfig

func (d *Deterlab) Configure() {
	// Directory setup - would also be possible in /tmp
	pwd, _ := os.Getwd()
	d.cothorityDir = pwd + "/cothority"
	d.deterDir = pwd + "/platform/deterlab"
	d.deployDir = d.deterDir + "/remote"
	d.buildDir = d.deterDir + "/build"
	d.MonitorPort = monitor.SinkPort
	dbg.Lvl3("Dirs are:", d.deterDir, d.deployDir)
	d.LoadAndCheckDeterlabVars()

	d.Debug = dbg.DebugVisible
	if d.Simulation == "" {
		dbg.Fatal("No simulation defined in runconfig")
	}

	// Setting up channel
	d.sshDeter = make(chan string)
}

// build is the name of the app to build
// empty = all otherwise build specific package
func (d *Deterlab) Build(build string) error {
	dbg.Lvl1("Building for", d.Login, d.Host, d.Project, build)
	start := time.Now()

	var wg sync.WaitGroup

	// Start with a clean build-directory
	current, _ := os.Getwd()
	dbg.Lvl3("Current dir is:", current, d.deterDir)
	defer os.Chdir(current)

	// Go into deterlab-dir and create the build-dir
	os.Chdir(d.deterDir)
	os.RemoveAll(d.buildDir)
	os.Mkdir(d.buildDir, 0777)

	// start building the necessary packages
	packages := []string{"simul", "users"}
	if build != "" {
		packages = strings.Split(build, ",")
	}
	dbg.Lvl3("Starting to build all executables", packages)
	for _, p := range packages {
		src_dir := d.deterDir + "/" + p
		basename := path.Base(p)
		if p == "simul" {
			src_dir = d.cothorityDir
			basename = "cothority"
		}
		dst := d.buildDir + "/" + basename

		dbg.Lvl3("Building", p, "from", src_dir, "into", basename)
		wg.Add(1)
		processor := "amd64"
		system := "linux"
		if p == "users" {
			processor = "386"
			system = "freebsd"
		}
		go func(src, dest string) {
			defer wg.Done()
			// deter has an amd64, linux architecture
			src_rel, _ := filepath.Rel(d.deterDir, src)
			dbg.Lvl3("Relative-path is", src, src_rel, d.deterDir)
			out, err := cliutils.Build("./"+src_rel, dest,
				processor, system)
			if err != nil {
				cliutils.KillGo()
				dbg.Lvl1(out)
				dbg.Fatal(err)
			}
		}(src_dir, dst)
	}
	// wait for the build to finish
	wg.Wait()
	dbg.Lvl1("Build is finished after", time.Since(start))
	return nil
}

// Kills all eventually remaining processes from the last Deploy-run
func (d *Deterlab) Cleanup() error {
	// Cleanup eventual ssh from the proxy-forwarding to the logserver
	err := exec.Command("pkill", "-9", "-f", "ssh -nNTf").Run()
	if err != nil {
		dbg.Lvl3("Error stopping ssh:", err)
	}

	// SSH to the deterlab-server and end all running users-processes
	dbg.Lvl3("Going to kill everything")
	var sshKill chan string
	sshKill = make(chan string)
	go func() {
		// Cleanup eventual residues of previous round - users and sshd
		cliutils.SshRun(d.Login, d.Host, "killall -9 users sshd")
		err := cliutils.SshRunStdout(d.Login, d.Host, "test -f remote/users && ( cd remote; ./users -kill )")
		if err != nil {
			dbg.Lvl1("NOT-Normal error from cleanup")
			sshKill <- "error"
		}
		sshKill <- "stopped"
	}()

	for {
		select {
		case msg := <-sshKill:
			if msg == "stopped" {
				dbg.Lvl3("Users stopped")
				return nil
			} else {
				dbg.Lvl2("Received other command", msg, "probably the app didn't quit correctly")
			}
		case <-time.After(time.Second * 20):
			dbg.Lvl3("Timeout error when waiting for end of ssh")
			return nil
		}
	}

	return nil
}

// Creates the appropriate configuration-files and copies everything to the
// deterlab-installation.
func (d *Deterlab) Deploy(rc RunConfig) error {
	os.RemoveAll(d.deployDir)
	os.Mkdir(d.deployDir, 0777)

	dbg.Lvl2("Localhost: Deploying and writing config-files")
	sim, err := sda.NewSimulation(d.Simulation, string(rc.Toml()))
	if err != nil {
		return err
	}
	// Initialize the deter-struct with our current structure (for debug-levels
	// and such), then read in the app-configuration to overwrite eventual
	// 'Machines', 'ppm', '' or other fields
	deter := *d
	deterConfig := d.deployDir + "/deter.toml"
	_, err = toml.Decode(string(rc.Toml()), &deter)
	if err != nil {
		return err
	}
	dbg.Lvl3("Creating hosts")
	deter.createHosts()
	app.WriteTomlConfig(deter, deterConfig, d.deployDir)

	simulConfig, err = sim.Setup(d.deployDir, deter.Virt)
	if err != nil {
		return err
	}
	simulConfig.Config = string(rc.Toml())
	dbg.Lvl3("Saving configuration")
	simulConfig.Save(d.deployDir)

	// Copy limit-files for more connections
	err = exec.Command("cp", d.deterDir+"/cothority.conf", d.deployDir).Run()

	// Copying build-files to deploy-directory
	build, err := ioutil.ReadDir(d.buildDir)
	for _, file := range build {
		err = exec.Command("cp", d.buildDir+"/"+file.Name(), d.deployDir).Run()
		if err != nil {
			dbg.Fatal("error copying build-file:", err)
		}
	}

	// Copy everything over to Deterlab
	dbg.Lvl1("Copying over to", d.Login, "@", d.Host)
	err = cliutils.Rsync(d.Login, d.Host, d.deployDir+"/", "remote/")
	if err != nil {
		dbg.Fatal(err)
	}
	dbg.Lvl2("Done copying")

	return nil
}

func (d *Deterlab) Start(args ...string) error {
	// setup port forwarding for viewing log server
	d.started = true
	// Remote tunneling : the sink port is used both for the sink and for the
	// proxy => the proxy redirects packets to the same port the sink is
	// listening.
	// -n = stdout == /Dev/null, -N => no command stream, -T => no tty
	redirection := strconv.Itoa(monitor.SinkPort+1) + ":" + d.ProxyAddress + ":" + strconv.Itoa(monitor.SinkPort)
	cmd := []string{"-nNTf", "-o", "StrictHostKeyChecking=no", "-o", "ExitOnForwardFailure=yes", "-R",
		redirection, fmt.Sprintf("%s@%s", d.Login, d.Host)}
	exCmd := exec.Command("ssh", cmd...)
	if err := exCmd.Start(); err != nil {
		dbg.Fatal("Failed to start the ssh port forwarding:", err)
	}
	if err := exCmd.Wait(); err != nil {
		dbg.Fatal("ssh port forwarding exited in failure:", err)
	}
	dbg.Lvl3("Setup remote port forwarding", cmd)
	go func() {
		err := cliutils.SshRunStdout(d.Login, d.Host, "cd remote; GOMAXPROCS=8 ./users")
		if err != nil {
			dbg.Lvl3(err)
		}
		d.sshDeter <- "finished"
	}()

	return nil
}

// Waiting for the process to finish
func (d *Deterlab) Wait() error {
	if d.started {
		dbg.Lvl3("Simulation is started")
		select {
		case msg := <-d.sshDeter:
			if msg == "finished" {
				dbg.Lvl3("Received finished-message, not killing users")
				return nil
			} else {
				dbg.Lvl1("Received out-of-line message", msg)
			}
		case <-time.After(time.Minute * 10):
			dbg.Lvl1("Quitting after 10 minutes of waiting")
		}
		d.started = false
	}
	return nil
}

// Reads in the deterlab-config and drops out if there is an error
func (d *Deterlab) ReadConfig(name ...string) {
	configName := "deter.toml"
	if len(name) > 0 {
		configName = name[0]
	}
	err := app.ReadTomlConfig(d, configName)
	_, caller, line, _ := runtime.Caller(1)
	who := caller + ":" + strconv.Itoa(line)
	if err != nil {
		dbg.Fatal("Couldn't read config in", who, ":", err)
	}
	dbg.DebugVisible = d.Debug
	monitor.SinkPort = d.MonitorPort
}

/*
* Write the hosts.txt file automatically
* from project name and number of servers
 */
func (d *Deterlab) createHosts() error {
	num_servers := d.Servers

	ip := "10.255.0."
	name := d.Project + ".isi.deterlab.net"
	d.Phys = make([]string, 0, num_servers)
	d.Virt = make([]string, 0, num_servers)
	for i := 1; i <= num_servers; i++ {
		d.Phys = append(d.Phys, fmt.Sprintf("server-%d.%s.%s", i-1, d.Experiment, name))
		d.Virt = append(d.Virt, fmt.Sprintf("%s%d", ip, i))
	}

	dbg.Lvl3("Physical:", d.Phys)
	dbg.Lvl3("Internal:", d.Virt)
	return nil
}

// Checks whether host, login and project are defined. If any of them are missing, it will
// ask on the command-line.
// For the login-variable, it will try to set up a connection to d.Host and copy over the
// public key for a more easy communication
func (d *Deterlab) LoadAndCheckDeterlabVars() {
	deter := Deterlab{}
	err := app.ReadTomlConfig(&deter, "deter.toml", d.deterDir)
	d.Host, d.Login, d.Project, d.Experiment, d.ProxyAddress, d.MonitorAddress =
		deter.Host, deter.Login, deter.Project, deter.Experiment,
		deter.ProxyAddress, deter.MonitorAddress

	if err != nil {
		dbg.Lvl1("Couldn't read config-file - asking for default values")
	}

	if d.Host == "" {
		d.Host = readString("Please enter the hostname of deterlab", "users.deterlab.net")
	}

	if d.Login == "" {
		d.Login = readString("Please enter the login-name on "+d.Host, "")
	}

	if d.Project == "" {
		d.Project = readString("Please enter the project on deterlab", "SAFER")
	}

	if d.Experiment == "" {
		d.Experiment = readString("Please enter the Experiment on "+d.Project, "Dissent-CS")
	}

	if d.MonitorAddress == "" {
		d.MonitorAddress = readString("Please enter the Monitor address (where clients will connect)", "users.isi.deterlab.net")
	}
	if d.ProxyAddress == "" {
		d.ProxyAddress = readString("Please enter the proxy redirection address", "localhost")
	}

	app.WriteTomlConfig(*d, "deter.toml", d.deterDir)
}

// Shows a messages and reads in a string, eventually returning a default (dft) string
func readString(msg, dft string) string {
	fmt.Printf("%s [%s]:", msg, dft)

	reader := bufio.NewReader(os.Stdin)
	strnl, _ := reader.ReadString('\n')
	str := strings.TrimSpace(strnl)
	if str == "" {
		return dft
	}
	return str
}
