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

	"bufio"
	"encoding/json"
	_ "errors"
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/config"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"io/ioutil"
	"path"
	"strings"
	"time"
	"path/filepath"
	"runtime"
)

type Deter struct {
	// The login on the platform
	Login        string
	// The outside host on the platform
	Host         string
	// The name of the project
	Project      string
	// Name of the Experiment - also name of hosts
	Experiment   string
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
	TestConnect  bool
}

func (d *Deter) Configure(config *Config) {
	// Directory setup - would also be possible in /tmp
	pwd, _ := os.Getwd()
	d.DeterDir = pwd + "/deterlab"
	d.DeployDir = d.DeterDir + "/remote"
	d.BuildDir = d.DeterDir + "/build"
	dbg.Lvl3("Dirs are:", d.DeterDir, d.DeployDir, d.BuildDir)

	// Setting up channel
	d.sshDeter = make(chan string)
	d.checkDeterlabVars()
}

func (d *Deter) Build(build, app string) error {
	dbg.Lvl1("Building for", d.Login, d.Host, d.Project, build)
	start := time.Now()

	var wg sync.WaitGroup

	// Start with a clean build-directory
	current, _ := os.Getwd()
	dbg.Lvl3("Current dir is:", current, d.DeterDir)
	defer os.Chdir(current)

	// Go into deterlab-dir and create the build-dir
	os.Chdir(d.DeterDir)
	os.RemoveAll(d.BuildDir)
	os.Mkdir(d.BuildDir, 0777)

	// start building the necessary packages
	packages := []string{"logserver", "forkexec", "app", "deter"}
	if build != "" {
		packages = strings.Split(build, ",")
	}
	dbg.Lvl3("Starting to build all executables", packages)
	for _, p := range packages {
		if p == "app" {
			p = "../../app/" + app
		}
		basename := path.Base(p)
		dst := d.BuildDir + "/" + basename

		src_dir := d.DeterDir + "/" + p
		dbg.Lvl3("Building ", p, "from", src_dir, "into", basename)
		wg.Add(1)
		if p == "deter" {
			go func(src, dest string) {
				defer wg.Done()
				// the users node has a 386 FreeBSD architecture
				// go won't compile on an absolute path so we need to
				// convert it to a relative one
				src_rel, _ := filepath.Rel(d.DeterDir, src)
				out, err := cliutils.Build("./" + src_rel, dest, "386", "freebsd")
				if err != nil {
					cliutils.KillGo()
					fmt.Println(out)
					log.Fatal(err)
				}
			}(src_dir, dst)
			continue
		}
		go func(src, dest string) {
			defer wg.Done()
			// deter has an amd64, linux architecture
			src_rel, _ := filepath.Rel(d.DeterDir, src)
			out, err := cliutils.Build("./" + src_rel, dest, "amd64", "linux")
			if err != nil {
				cliutils.KillGo()
				fmt.Println(out)
				log.Fatal(err)
			}
		}(src_dir, dst)
	}
	// wait for the build to finish
	wg.Wait()
	dbg.Lvl1("Build is finished after", time.Since(start))
	return nil
}

func (d *Deter) Deploy(conf *Config) error {
	dbg.Lvl1("Assembling all files and configuration options")
	os.RemoveAll(d.DeployDir)
	os.Mkdir(d.DeployDir, 0777)

	dbg.Lvl1("Writing config-files")

	d.generateHostsFile(conf)
	d.readHosts(conf)
	d.calculateGraph(conf)
	WriteConfig(d, "deter.toml", d.DeployDir)
	WriteConfig(conf, "deploy.toml", d.DeployDir)

	// copy the webfile-directory of the logserver to the remote directory
	err := exec.Command("cp", "-a", d.DeterDir + "/logserver/webfiles",
		d.DeterDir + "/cothority.conf", d.DeployDir).Run()
	if err != nil {
		log.Fatal("error copying webfiles:", err)
	}
	build, err := ioutil.ReadDir(d.BuildDir)
	for _, file := range build {
		err = exec.Command("cp", d.BuildDir + "/" + file.Name(), d.DeployDir).Run()
		if err != nil {
			log.Fatal("error copying build-file:", err)
		}
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

func (d *Deter) Start(conf *Config) error {
	dbg.Lvl1("Running with", conf.Nmachs, "nodes *", conf.Hpn, "hosts per node =",
		conf.Nmachs * conf.Hpn, "and", conf.Nloggers, "loggers")

	// setup port forwarding for viewing log server
	dbg.Lvl3("setup port forwarding for master logger: ", d.masterLogger, d.Login, d.Host)
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

	go func() {
		dbg.Lvl3(cliutils.SshRunStdout(d.Login, d.Host, "cd remote; GOMAXPROCS=8 ./deter"))
		dbg.Lvl3("Sending stop of ssh")
		d.sshDeter <- "stop"
	}()

	return nil
}

func (d *Deter) Stop() error {
	killssh := exec.Command("pkill", "-f", "ssh -t -t")
	killssh.Stdout = os.Stdout
	killssh.Stderr = os.Stderr
	err := killssh.Run()
	if err != nil {
		dbg.Lvl3("Stopping ssh: ", err)
	}
	select {
	case msg := <-d.sshDeter:
		if msg == "stop" {
			dbg.Lvl3("SSh is stopped")
		} else {
			dbg.Lvl1("Received other command", msg)
		}
	case <-time.After(time.Second * 3):
		dbg.Lvl3("Timeout error when waiting for end of ssh")
	}
	return nil
}

func ReadConfigDeter(deter *Deter, conf *Config){
	err := ReadConfig(deter, "deter.toml")
	if err != nil {
		log.Fatal("Couldn't read config in", runtime.Caller(1), ":", err)
	}
	err = ReadConfig(conf, "deploy.toml")
	if err != nil {
		log.Fatal("Couldn't read config in", runtime.Caller(1), ":", err)
	}
}

/*
* Write the hosts.txt file automatically
* from project name and number of servers
 */
func (d *Deter) generateHostsFile(conf *Config) error {
	hosts_file := d.DeployDir + "/hosts.txt"
	num_servers := conf.Nmachs + conf.Nloggers

	// open and erase file if needed
	if _, err1 := os.Stat(hosts_file); err1 == nil {
		dbg.Lvl4("Hosts file", hosts_file, "already exists. Erasing ...")
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
	name := d.Project + ".isi.deterlab.net"
	for i := 1; i <= num_servers; i++ {
		f.WriteString(fmt.Sprintf("server-%d.%s.%s\t%s%d\n", i - 1, d.Experiment, name, ip, i))
	}
	dbg.Lvl4(fmt.Sprintf("Created hosts file description (%d hosts)", num_servers))
	return err

}

// parse the hosts.txt file to create a separate list (and file)
// of physical nodes and virtual nodes. Such that each host on line i, in phys.txt
// corresponds to each host on line i, in virt.txt.
func (d *Deter) readHosts(conf *Config) {
	hosts_file := d.DeployDir + "/hosts.txt"
	nmachs, nloggers := conf.Nmachs, conf.Nloggers

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

	// phys.txt and virt.txt only contain the number of machines that we need
	dbg.Lvl3("Reading phys and virt")
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
func (d *Deter) calculateGraph(conf *Config) {
	d.virt = d.virt[conf.Nloggers:]
	d.phys = d.phys[conf.Nloggers:]
	t, hostnames, depth, err := graphs.TreeFromList(d.virt, conf.Hpn, conf.Bf)
	dbg.Lvl2("Depth:", depth)
	dbg.Lvl2("Total hosts:", len(hostnames))
	total := conf.Nmachs * conf.Hpn
	if len(hostnames) != total {
		dbg.Lvl1("Only calculated", len(hostnames), "out of", total, "hosts - try changing number of",
			"machines or hosts per node")
		log.Fatal("Didn't calculate enough hosts")
	}

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
func (d *Deter) checkDeterlabVars() {
	// Write
	config := Deter{}
	err := ReadConfig(&config, "deter.toml", d.DeterDir)

	if err != nil {
		dbg.Lvl1("Couldn't read config-file - asking for default values")
	}

	if config.Host == "" {
		d.Host = readString("Please enter the hostname of deterlab", "users.deterlab.net")
	} else {
		d.Host = config.Host
	}

	if config.Login == "" {
		d.Login = readString("Please enter the login-name on " + d.Host, "")
	} else {
		d.Login = config.Login
	}

	if config.Project == "" {
		d.Project = readString("Please enter the project on deterlab", "SAFER")
	} else {
		d.Project = config.Project
	}

	if config.Experiment == "" {
		d.Experiment = readString("Please enter the Experiment on " + d.Project, "Dissent-CS")
	} else {
		d.Experiment = config.Experiment
	}

	WriteConfig(*d, "deter.toml", d.DeterDir)
}

// Shows a messages and reads in a string, eventually returning a default (dft) string
func readString(msg, dft string) string {
	fmt.Printf("%s [%s]: ", msg, dft)

	reader := bufio.NewReader(os.Stdin)
	strnl, _ := reader.ReadString('\n')
	str := strings.TrimSpace(strnl)
	if str == "" {
		return dft
	}
	return str
}
