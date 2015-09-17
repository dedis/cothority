// Deterlab is responsible for setting up everything to test the application
// on deterlab.net
// Given a list of hostnames, it will create an overlay
// tree topology, using all but the last node. It will create multiple
// nodes per server and run timestamping processes. The last node is
// reserved for the logging server, which is forwarded to localhost:8081
//
// Creates the following directory structure in remote:
// build/ - where all cross-compiled executables are stored
// deploy/ - directory to be copied to the deterlab server
//
// The following apps are used:
//   deter - runs on the user-machine in deterlab and launches the others
//   logserver - runs on the first three servers - first is the master, then two slaves
//   forkexec - runs on the other servers and launches exec, so it can measure it's cpu usage
//
package deploy

import (
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"fmt"
	"strings"
	"io/ioutil"
	"github.com/dedis/cothority/lib/graphs"
	"encoding/json"
	"strconv"
	"bytes"
	"github.com/BurntSushi/toml"
	"time"
	_ "errors"
	"github.com/dedis/cothority/lib/config"
	"bufio"
	"path"
)


type Deter struct {
	Config       *Config
	// The login on the platform
	Login        string
	// The outside host on the platform
	Host         string
	// The name of the internal hosts
	Project      string
	// Directory where everything is copied into
	DeployDir    string
	// Directory for building
	BuildDir     string
	// Working directory of deterlab
	DeterDir     string
	// Where the main logging machine resides
	masterLogger string
	// DNS-resolvable names
	phys         []string
	// VLAN-IP names
	virt         []string
	physOut      string
	virtOut      string

	// Channel to communication stopping of experiment
	sshDeter     chan string

	// Testing the connection?
	TestConnect	bool
}

func (d *Deter) Configure(config *Config) {
	d.Config = config

	// Directory setup - would also be possible in /tmp
	pwd, _ := os.Getwd()
	d.DeterDir = pwd + "/deploy/deterlab"
	d.DeployDir = d.DeterDir + "/deploy"
	d.BuildDir = d.DeterDir + "/build"
	d.Config.Debug = dbg.DebugVisible

	// Setting up channel
	d.sshDeter = make(chan string)
	d.checkDeterlabVars()
}

func (d *Deter) Build(build string) (error) {
	dbg.Lvl1("Building for", d.Login, d.Host, d.Project, build)
	start := time.Now()

	var wg sync.WaitGroup

	// Start with a clean build-directory
	current, _ := os.Getwd()
	dbg.Lvl3("Current dir is:", current)
	defer os.Chdir(current)

	// Go into deterlab-dir and create the build-dir
	os.Chdir(d.DeterDir)
	os.RemoveAll(d.BuildDir)
	os.Mkdir(d.BuildDir, 0777)

	// start building the necessary packages
	packages := []string{"logserver", "forkexec", "../../app", "deter"}
	if build != "" {
		packages = strings.Split(build, ",")
	}
	dbg.Lvl2("Starting to build all executables", packages)
	for _, p := range packages {
		basename := path.Base(p)
		dbg.Lvl3("Building ", p, "into", basename)
		wg.Add(1)
		src := p + "/" + basename + ".go"
		dst := d.BuildDir + "/" + basename
		if p == "deter" {
			go func(s, d string) {
				defer wg.Done()
				// the users node has a 386 FreeBSD architecture
				out, err := cliutils.Build(s, d, "386", "freebsd")
				if err != nil {
					cliutils.KillGo()
					fmt.Println(out)
					log.Fatal(err)
				}
			}(src, dst)
			continue
		}
		go func(s, d string) {
			defer wg.Done()
			// deter has an amd64, linux architecture
			out, err := cliutils.Build(s, d, "amd64", "linux")
			if err != nil {
				cliutils.KillGo()
				fmt.Println(out)
				log.Fatal(err)
			}
		}(src, dst)
	}
	// wait for the build to finish
	wg.Wait()
	dbg.Lvl1("Build is finished after", time.Since(start))
	return nil
}

func (d *Deter) Deploy() (error) {
	dbg.Lvl1("Assembling all files and configuration options")
	os.RemoveAll(d.DeployDir)
	os.Mkdir(d.DeployDir, 0777)

	d.generateHostsFile()
	d.readHosts()
	d.calculateGraph()
	d.WriteConfig()

	// copy the webfile-directory of the logserver to the remote directory
	err := exec.Command("cp", "-a", d.DeterDir + "/logserver/webfiles", d.BuildDir + "/",
		d.DeterDir + "/cothority.conf", d.DeployDir).Run()
	if err != nil {
		log.Fatal("error copying webfiles and build-dir:", err)
	}

	dbg.Lvl1("Copying over to", d.Login, "@", d.Host)
	// Copy everything over to deterlabs
	err = cliutils.Rsync(d.Login, d.Host, d.DeployDir + "/", "remote/")
	if err != nil {
		log.Fatal(err)
	}

	dbg.Lvl1("Done copying")
	return nil
}

func (d *Deter) Start() (error) {
	dbg.Lvl1("Running with", d.Config.Nmachs, "nodes *", d.Config.Hpn, "hosts per node =",
		d.Config.Nmachs * d.Config.Hpn, "and", d.Config.Nloggers, "loggers")

	// setup port forwarding for viewing log server
	dbg.Lvl2("setup port forwarding for master logger: ", d.masterLogger, d.Login, d.Host)
	cmd := exec.Command(
		"ssh",
		"-t",
		"-t",
		fmt.Sprintf("%s@%s", d.Login, d.Host),
		"-L",
		"8081:" + d.masterLogger + ":10000")
	err := cmd.Start()
	if err != nil {
		log.Fatal("failed to setup portforwarding for logging server")
	}

	dbg.Lvl2("runnning deter with nmsgs:", d.Config.Nmsgs, d.Login, d.Host)
	// run the deter lab boss nodes process
	// it will be responsible for forwarding the files and running the individual
	// timestamping servers

	go func() {
		dbg.Lvl2(cliutils.SshRunStdout(d.Login, d.Host,
			"GOMAXPROCS=8 remote/deter -nmsgs=" + strconv.Itoa(d.Config.Nmsgs) +
			" -hpn=" + strconv.Itoa(d.Config.Hpn) +
			" -bf=" + strconv.Itoa(d.Config.Bf) +
			" -rate=" + strconv.Itoa(d.Config.Rate) +
			" -rounds=" + strconv.Itoa(d.Config.Rounds) +
			" -debug=" + strconv.Itoa(d.Config.Debug) +
			" -failures=" + strconv.Itoa(d.Config.Failures) +
			" -rfail=" + strconv.Itoa(d.Config.RFail) +
			" -ffail=" + strconv.Itoa(d.Config.FFail) +
			" -app=" + d.Config.App +
			" -suite=" + d.Config.Suite))
		dbg.Lvl2("Sending stop of ssh")
		d.sshDeter <- "stop"
	}()

	return nil
}

func (d *Deter) Stop() (error) {
	killssh := exec.Command("pkill", "-f", "ssh -t -t")
	killssh.Stdout = os.Stdout
	killssh.Stderr = os.Stderr
	err := killssh.Run()
	if err != nil {
		dbg.Lvl2("Stopping ssh: ", err)
	}
	select {
	case msg := <-d.sshDeter:
		if msg == "stop" {
			dbg.Lvl2("SSh is stopped")
		}else {
			dbg.Lvl1("Received other command", msg)
		}
	case <-time.After(time.Second * 3):
		dbg.Lvl2("Timeout error when waiting for end of ssh")
	}
	return nil
}

func (d *Deter) WriteConfig(dirOpt ...string) {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(d); err != nil {
		log.Fatal(err)
	}
	dir := d.DeployDir
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	}
	err := ioutil.WriteFile(dir + "/config.toml", buf.Bytes(), 0660)
	if err != nil {
		log.Fatal(err)
	}
	dbg.Lvl3("Wrote login", d.Login, "to", dir)
}

func ReadConfig(dirOpt ...string) (*Deter, error) {
	var deter Deter

	dir := "."
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	}
	buf, err := ioutil.ReadFile(dir + "/config.toml")
	if err != nil {
		return &deter, err
	}

	_, err = toml.Decode(string(buf), &deter)
	if err != nil {
		log.Fatal(err)
	}

	return &deter, nil
}

/*
* Write the hosts.txt file automatically
* from project name and number of servers
 */
func (d *Deter)generateHostsFile() error {
	hosts_file := d.DeployDir + "/hosts.txt"
	num_servers := d.Config.Nmachs + d.Config.Nloggers

	// open and erase file if needed
	if _, err1 := os.Stat(hosts_file); err1 == nil {
		dbg.Lvl3("Hosts file", hosts_file, "already exists. Erasing ...")
		os.Remove(hosts_file)
	}
	// create the file
	f, err := os.Create(hosts_file)
	if err != nil {
		log.Fatal("Could not create hosts file description: ", hosts_file, " :: ", err)
		return err
	}
	defer f.Close()

	// write the name of the server + \t + IP address
	ip := "10.255.0."
	name := "SAFER.isi.deterlab.net"
	for i := 1; i <= num_servers; i++ {
		f.WriteString(fmt.Sprintf("server-%d.%s.%s\t%s%d\n", i - 1, d.Project, name, ip, i))
	}
	dbg.Lvl3(fmt.Sprintf("Created hosts file description (%d hosts)", num_servers))
	return err

}

// parse the hosts.txt file to create a separate list (and file)
// of physical nodes and virtual nodes. Such that each host on line i, in phys.txt
// corresponds to each host on line i, in virt.txt.
func (d *Deter)readHosts() {
	hosts_file := d.DeployDir + "/hosts.txt"
	nmachs, nloggers := d.Config.Nmachs, d.Config.Nloggers

	physVirt, err := cliutils.ReadLines(hosts_file)
	if err != nil {
		log.Panic("Couldn't find", hosts_file)
	}

	d.phys = make([]string, 0, len(physVirt) / 2)
	d.virt = make([]string, 0, len(physVirt) / 2)
	for i := 0; i < len(physVirt); i += 2 {
		d.phys = append(d.phys, physVirt[i])
		d.virt = append(d.virt, physVirt[i + 1])
	}
	d.phys = d.phys[:nmachs + nloggers]
	d.virt = d.virt[:nmachs + nloggers]
	d.physOut = strings.Join(d.phys, "\n")
	d.virtOut = strings.Join(d.virt, "\n")
	d.masterLogger = d.phys[0]
	// slaveLogger1 := phys[1]
	// slaveLogger2 := phys[2]

	// phys.txt and virt.txt only contain the number of machines that we need
	dbg.Lvl2("Reading phys and virt")
	err = ioutil.WriteFile(d.DeployDir + "/phys.txt", []byte(d.physOut), 0666)
	if err != nil {
		log.Fatal("failed to write physical nodes file", err)
	}

	err = ioutil.WriteFile(d.DeployDir + "/virt.txt", []byte(d.virtOut), 0666)
	if err != nil {
		log.Fatal("failed to write virtual nodes file", err)
	}
}

// Calculates a tree that is used for the timestampers
func (d *Deter)calculateGraph() {
	d.virt = d.virt[3:]
	d.phys = d.phys[3:]
	t, hostnames, depth, err := graphs.TreeFromList(d.virt, d.Config.Hpn, d.Config.Bf)
	dbg.Lvl2("DEPTH:", depth)
	dbg.Lvl2("TOTAL HOSTS:", len(hostnames))

	// generate the configuration file from the tree
	cf := config.ConfigFromTree(t, hostnames)
	cfb, err := json.Marshal(cf)
	err = ioutil.WriteFile(d.DeployDir + "/tree.json", cfb, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

// Checks whether host, login and project are defined. If any of them are missing, it will
// ask on the command-line.
// For the login-variable, it will try to set up a connection to d.Host and copy over the
// public key for a more easy communication
func (d *Deter)checkDeterlabVars() {
	// Write
	var config, err = ReadConfig(d.DeterDir)

	if err != nil {
		dbg.Lvl1("Couldn't read config-file - asking for default values")
	}

	if config.Host == "" {
		d.Host = readString("Please enter the hostname of deterlab (enter for 'users.deterlab.net'): ",
			"users.deterlab.net")
	} else {
		d.Host = config.Host
	}

	if config.Login == "" {
		d.Login = readString("Please enter the login-name on " + d.Host + ":", "")
	} else {
		d.Login = config.Login
	}

	if config.Project == "" {
		d.Project = readString("Please enter the project on deterlab: ", "Dissent-CS")
	} else {
		d.Project = config.Project
	}

	d.WriteConfig(d.DeterDir)
}

// Shows a messages and reads in a string, eventually returning a default (dft) string
func readString(msg, dft string) (string) {
	fmt.Print(msg)

	reader := bufio.NewReader(os.Stdin)
	strnl, _ := reader.ReadString('\n')
	str := strings.TrimSpace(strnl)
	if str == "" {
		return dft
	}
	return str
}