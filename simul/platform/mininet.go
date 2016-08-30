// Mininet is the platform-implementation that uses the MiniNet-framework
// set in place by Marc-Andre Luthi from EPFL. It is based on MiniNet,
// as it uses a lot of similar routines

package platform

import (
	_ "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"net"

	"io/ioutil"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
)

type MiniNet struct {
	// *** Mininet-related configuration
	// The login on the platform
	Login string
	// The outside host on the platform
	Host string
	// Directory we start - supposed to be `cothority/simul`
	wd string
	// Directory holding the cothority-go-file
	cothorityDir string
	// Directory storing the additional files
	mininetDir string
	// Directory for building
	buildDir string
	// IPs of all hosts
	HostIPs []string
	// Channel to communicate stopping of experiment
	sshMininet chan string
	// Whether the simulation is started
	started bool

	// ProxyAddress : the proxy will redirect every traffic it
	// receives to this address
	ProxyAddress string
	// Port number of the monitor and the proxy
	MonitorPort int

	// Simulation to be run
	Simulation string
	// Number of servers to be used
	Servers int
	// Number of machines
	Hosts int
	// Debugging-level: 0 is none - 5 is everything
	Debug int
	// The number of seconds to wait for closing the connection
	CloseWait int
}

func (m *MiniNet) Configure(pc *Config) {
	// Directory setup - would also be possible in /tmp
	// Supposes we're in `cothority/simul`
	m.wd, _ = os.Getwd()
	m.cothorityDir = m.wd + "/cothority"
	m.mininetDir = m.wd + "/platform/mininet"
	m.buildDir = m.mininetDir + "/build"
	m.Login = "root"
	log.Print(m.wd, m.cothorityDir)
	m.Host = "iccluster026.iccluster.epfl.ch"
	m.ProxyAddress = "localhost"
	m.MonitorPort = pc.MonitorPort
	m.Debug = pc.Debug

	// Clean the MiniNet-dir, create it and change into it
	os.RemoveAll(m.buildDir)
	os.Mkdir(m.buildDir, 0700)
	sda.WriteTomlConfig(*m, m.mininetDir+"/mininet.toml", m.mininetDir)

	if m.Simulation == "" {
		log.Fatal("No simulation defined in runconfig")
	}

	// Setting up channel
	m.sshMininet = make(chan string)
}

// build is the name of the app to build
// empty = all otherwise build specific package
func (m *MiniNet) Build(build string, arg ...string) error {
	log.Lvl1("Building for", m.Login, m.Host, build, "cothorityDir=", m.cothorityDir)
	start := time.Now()

	// Start with a clean build-directory
	processor := "amd64"
	system := "linux"
	src_rel, err := filepath.Rel(m.wd, m.cothorityDir)
	log.ErrFatal(err)

	log.LLvl3("Relative-path is", src_rel, " will build into ", m.buildDir)
	out, err := Build("./"+src_rel, m.buildDir+"/cothority",
		processor, system, arg...)
	log.ErrFatal(err, out)

	log.Lvl1("Build is finished after", time.Since(start))
	return nil
}

// Kills all eventually remaining processes from the last Deploy-run
func (m *MiniNet) Cleanup() error {
	// Cleanup eventual ssh from the proxy-forwarding to the logserver
	err := exec.Command("pkill", "-9", "-f", "ssh -nNTf").Run()
	if err != nil {
		log.Lvl3("Error stopping ssh:", err)
	}

	// SSH to the MiniNet-server and end all running users-processes
	log.Lvl3("Going to stop everything")
	//err = SSHRunStdout(m.Login, m.Host, "ps aux")
	//if err != nil {
	//	log.Lvl3(err)
	//}
	return nil
}

// Creates the appropriate configuration-files and copies everything to the
// MiniNet-installation.
func (m *MiniNet) Deploy(rc RunConfig) error {
	log.Lvl2("Localhost: Deploying and writing config-files")
	sim, err := sda.NewSimulation(m.Simulation, string(rc.Toml()))
	if err != nil {
		return err
	}

	// Initialize the mininet-struct with our current structure (for debug-levels
	// and such), then read in the app-configuration to overwrite eventual
	// 'Servers', 'Hosts', '' or other fields
	mininet := *m
	mininetConfig := m.mininetDir + "/mininet.toml"
	_, err = toml.Decode(string(rc.Toml()), &mininet)
	if err != nil {
		return err
	}
	log.Lvl3("Creating hosts")
	mininet.readServers()
	log.Lvl3("Writing the config file :", mininet)
	sda.WriteTomlConfig(mininet, mininetConfig, m.buildDir)

	if err = m.readServers(); err != nil {
		return err
	}
	hosts, list, err := m.getHostList(rc)
	if err != nil {
		return err
	}
	log.Lvl3("Hosts are:", hosts)
	log.Lvl3("List is:", list)
	err = ioutil.WriteFile(m.buildDir+"/list", []byte(list), 0660)
	if err != nil {
		return err
	}
	simulConfig, err := sim.Setup(m.buildDir, hosts)
	if err != nil {
		return err
	}
	simulConfig.Config = string(rc.Toml())
	log.Lvl3("Saving configuration")
	simulConfig.Save(m.buildDir)

	// Copy our script
	err = Copy(m.buildDir, m.mininetDir+"/start.py")
	if err != nil {
		log.Error(err)
		return err
	}
	// Copy everything over to MiniNet
	log.Lvl1("Copying over to", m.Login, "@", m.Host)
	err = Rsync(m.Login, m.Host, m.buildDir+"/", "mininet_run/")
	if err != nil {
		log.Fatal(err)
	}
	log.Lvl2("Done copying")

	return nil
}

func (m *MiniNet) Start(args ...string) error {
	// setup port forwarding for viewing log server
	m.started = true
	// Remote tunneling : the sink port is used both for the sink and for the
	// proxy => the proxy redirects packets to the same port the sink is
	// listening.
	// -n = stdout == /Dev/null, -N => no command stream, -T => no tty
	var exCmd *exec.Cmd
	if true {
		redirection := strconv.Itoa(m.MonitorPort) + ":" + m.ProxyAddress + ":" + strconv.Itoa(m.MonitorPort)
		login := fmt.Sprintf("%s@%s", m.Login, m.Host)
		cmd := []string{"-nNTf", "-o", "StrictHostKeyChecking=no", "-o", "ExitOnForwardFailure=yes", "-R",
			redirection, login}
		exCmd = exec.Command("ssh", cmd...)
		if err := exCmd.Start(); err != nil {
			log.Fatal("Failed to start the ssh port forwarding:", err)
		}
		if err := exCmd.Wait(); err != nil {
			log.Fatal("ssh port forwarding exited in failure:", err)
		}
	} else {
		redirection := strconv.Itoa(m.MonitorPort) + ":" + m.ProxyAddress + ":" + strconv.Itoa(m.MonitorPort)
		login := fmt.Sprintf("%s@%s", m.Login, "icsil1-conodes-exp.epfl.ch")
		cmd := []string{"-nNTf", "-o", "StrictHostKeyChecking=no", "-o", "ExitOnForwardFailure=yes", "-R",
			redirection, login}
		exCmd = exec.Command("ssh", cmd...)
		log.Print(exCmd)
		if err := exCmd.Start(); err != nil {
			log.Fatal("Failed to start the 2nd ssh port forwarding:", err)
		}
		if err := exCmd.Wait(); err != nil {
			log.Fatal("2nd ssh port forwarding exited in failure:", err)
		}
	}
	go func() {
		log.LLvl3("Starting simulation over mininet")
		_, err := SSHRun(m.Login, m.Host, "cd mininet_run; ./start.py list go")
		if err != nil {
			log.Lvl3(err)
		}
		log.Print("Finished ssh-command")
		time.Sleep(time.Second * 100)
		m.sshMininet <- "finished"
	}()

	return nil
}

// Waiting for the process to finish
func (m *MiniNet) Wait() error {
	wait := m.CloseWait
	if wait == 0 {
		wait = 600
	}
	if m.started {
		log.Lvl3("Simulation is started")
		select {
		case msg := <-m.sshMininet:
			if msg == "finished" {
				log.Lvl3("Received finished-message, not killing users")
				return nil
			} else {
				log.Lvl1("Received out-of-line message", msg)
			}
		case <-time.After(time.Second * time.Duration(wait)):
			log.Lvl1("Quitting after ", wait/60,
				" minutes of waiting")
			m.started = false
		}
		m.started = false
	}
	return nil
}

// Returns the servers to use for mininet.
// TODO: make it more user-definable.
func (m *MiniNet) readServers() error {
	m.HostIPs = []string{}
	for _, h := range []string{"iccluster026", "iccluster027"} {
		ips, err := net.LookupIP(h + ".iccluster.epfl.ch")
		if err != nil {
			return err
		}
		log.LLvl3("Found IP for", h, ":", ips[0])
		m.HostIPs = append(m.HostIPs, ips[0].String())
	}
	log.LLvl3("Nodes are:", m.HostIPs)
	return nil
}

// getHostList returns the hosts that will be used for the communication
// and the list for `start.py`.
func (m *MiniNet) getHostList(rc RunConfig) (hosts []string, list string, err error) {
	hosts = []string{}
	list = ""
	physicalServers := len(m.HostIPs)
	nets := make([]*net.IPNet, physicalServers)
	ips := make([]net.IP, physicalServers)
	for n := range nets {
		ips[n], nets[n], err = net.ParseCIDR(fmt.Sprintf("10.%d.0.0/16", n+1))
		if err != nil {
			return
		}
		// We'll have to start with 10.1.0.2 as the first host.
		// So we set the LSByte to 1 which will be increased later.
		ips[n][len(ips[n])-1] = byte(1)
	}
	hosts = []string{}
	nbrServers, err := rc.GetInt("Servers")
	if err != nil {
		log.Print(rc)
		return
	}
	if nbrServers > physicalServers {
		log.Warn("Fewer physical servers available than requested.")
	}
	nbrHosts, err := rc.GetInt("Hosts")
	if err != nil {
		log.Print(rc)
		return
	}
	for i := 0; i < nbrHosts; i++ {
		ip := ips[i%physicalServers]
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] > 0 {
				break
			}
		}
		ips[i%physicalServers] = ip
		hosts = append(hosts, ip.String())
	}
	for i, s := range nets {
		if i >= nbrHosts {
			break
		}
		// Magical formula to get how many hosts run on each
		// physical server if we distribute them evenly, starting
		// from the first server.
		h := (nbrHosts + physicalServers - 1 - i) / physicalServers
		list += fmt.Sprintf("%s %s %d\n",
			m.HostIPs[i], s.String(), h)
	}
	return
}
