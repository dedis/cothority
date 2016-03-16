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
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

type MiniNet struct {
	// *** Mininet-related configuration
	// The login on the platform
	Login string
	// The outside host on the platform
	Host string
	// Directory holding the cothority-go-file
	cothorityDir string
	// Working directory of mininet
	mininetDir string
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

func (m *MiniNet) Configure(pc *PlatformConfig) {
	// Directory setup - would also be possible in /tmp
	pwd, _ := os.Getwd()
	m.cothorityDir = pwd + "/cothority"
	m.mininetDir = pwd + "/platform/mininet"
	m.Login = "mininet"
	m.Host = "icsil1-conodes.epfl.ch"
	m.ProxyAddress = "localhost"
	m.MonitorPort = pc.MonitorPort
	m.Debug = pc.Debug

	// Clean the MiniNet-dir, create it and change into it
	os.RemoveAll(m.mininetDir)
	os.Mkdir(m.mininetDir, 0777)
	os.Chdir(m.mininetDir)
	sda.WriteTomlConfig(*m, "mininet.toml", m.mininetDir)

	if m.Simulation == "" {
		dbg.Fatal("No simulation defined in runconfig")
	}

	// Setting up channel
	m.sshMininet = make(chan string)
}

// build is the name of the app to build
// empty = all otherwise build specific package
func (m *MiniNet) Build(build string, arg ...string) error {
	dbg.Lvl1("Building for", m.Login, m.Host, build, "cothorityDir=", m.cothorityDir)
	start := time.Now()

	// Start with a clean build-directory
	current, _ := os.Getwd()
	dbg.Lvl3("Current dir is:", current, m.mininetDir)
	defer os.Chdir(current)

	src_dir := m.cothorityDir
	dst := m.mininetDir + "/cothority"
	processor := "amd64"
	system := "linux"
	src_rel, _ := filepath.Rel(m.mininetDir, src_dir)
	dbg.Lvl3("Relative-path is", src_rel, " will build into ", dst)
	out, err := cliutils.Build("./"+src_rel, dst,
		processor, system, arg...)
	if err != nil {
		cliutils.KillGo()
		dbg.Lvl1(out)
		dbg.Fatal(err)
	}

	dbg.Lvl1("Build is finished after", time.Since(start))
	return nil
}

// Kills all eventually remaining processes from the last Deploy-run
func (m *MiniNet) Cleanup() error {
	// Cleanup eventual ssh from the proxy-forwarding to the logserver
	err := exec.Command("pkill", "-9", "-f", "ssh -nNTf").Run()
	if err != nil {
		dbg.Lvl3("Error stopping ssh:", err)
	}

	// SSH to the MiniNet-server and end all running users-processes
	dbg.Lvl3("Going to stop everything")
	startcli := "echo -e \"stop\\nquit\\n\" | python cli.py"
	err = cliutils.SshRunStdout(m.Login, m.Host, "cd mininet/conodes/sites/icsil1; "+startcli)
	if err != nil {
		dbg.Lvl3(err)
	}
	dbg.LLvl3("Done with cli.py")
	return nil
}

// Creates the appropriate configuration-files and copies everything to the
// MiniNet-installation.
func (m *MiniNet) Deploy(rc RunConfig) error {
	dbg.Lvl2("Localhost: Deploying and writing config-files")
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
	dbg.Lvl3("Creating hosts")
	mininet.readHosts()
	dbg.Lvl3("Writing the config file :", mininet)
	sda.WriteTomlConfig(mininet, mininetConfig, m.mininetDir)

	simulConfig, err := sim.Setup(m.mininetDir, mininet.HostIPs)
	if err != nil {
		return err
	}
	simulConfig.Config = string(rc.Toml())
	dbg.Lvl3("Saving configuration")
	simulConfig.Save(m.mininetDir)

	// Copy everything over to MiniNet
	dbg.Lvl1("Copying over to", m.Login, "@", m.Host)
	err = cliutils.Rsync(m.Login, m.Host, m.mininetDir+"/", "mininet_run/")
	if err != nil {
		dbg.Fatal(err)
	}
	dbg.Lvl2("Done copying")

	return nil
}

func (m *MiniNet) Start(args ...string) error {
	// setup port forwarding for viewing log server
	m.started = true
	// Remote tunneling : the sink port is used both for the sink and for the
	// proxy => the proxy redirects packets to the same port the sink is
	// listening.
	// -n = stdout == /Dev/null, -N => no command stream, -T => no tty
	redirection := strconv.Itoa(m.MonitorPort) + ":" + m.ProxyAddress + ":" + strconv.Itoa(m.MonitorPort)
	cmd := []string{"-nNTf", "-o", "StrictHostKeyChecking=no", "-o", "ExitOnForwardFailure=yes", "-R",
		redirection, fmt.Sprintf("%s@%s", m.Login, m.Host)}
	exCmd := exec.Command("ssh", cmd...)
	if err := exCmd.Start(); err != nil {
		dbg.Fatal("Failed to start the ssh port forwarding:", err)
	}
	if err := exCmd.Wait(); err != nil {
		dbg.Fatal("ssh port forwarding exited in failure:", err)
	}
	dbg.Lvl3("Setup remote port forwarding", cmd)
	go func() {
		dbg.LLvl3("Starting simulation over mininet")
		startcli := "echo -e \"sync\\nstart\\n\\nquit\\n\" | python cli.py"
		_, err := cliutils.SshRun(m.Login, m.Host, "cd mininet/conodes/sites/icsil1; "+startcli)
		if err != nil {
			dbg.Lvl3(err)
		}
		time.Sleep(time.Second * 60)
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
		dbg.Lvl3("Simulation is started")
		select {
		case msg := <-m.sshMininet:
			if msg == "finished" {
				dbg.Lvl3("Received finished-message, not killing users")
				return nil
			} else {
				dbg.Lvl1("Received out-of-line message", msg)
			}
		case <-time.After(time.Second * time.Duration(wait)):
			dbg.Lvl1("Quitting after ", wait/60,
				" minutes of waiting")
			m.started = false
		}
		m.started = false
	}
	return nil
}

/*
* connect to the MiniNet server and check how many servers we got attributed
 */
func (m *MiniNet) readHosts() error {
	// Updating the available servers
	_, err := cliutils.SshRun(m.Login, m.Host, "cd mininet; ./icsil1_search_server.py icsil1.servers.json")
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("cd mininet/conodes && ./dispatched.py %d %s %d && "+
		"cat sites/icsil1/nodes.txt", m.Debug, m.Simulation, monitor.DefaultSinkPort)
	nodesSlice, err := cliutils.SshRun(m.Login, m.Host, cmd)
	if err != nil {
		return err
	}
	nodes := strings.Split(string(nodesSlice), "\n")
	num_servers := len(nodes) - 2

	m.HostIPs = make([]string, num_servers)
	copy(m.HostIPs, nodes[2:])
	dbg.Lvl4("Nodes are:", m.HostIPs)
	return nil
}
