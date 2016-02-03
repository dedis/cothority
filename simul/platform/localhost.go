package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	_ "github.com/dedis/cothority/protocols"
)

// Localhost is responsible for launching the app with the specified number of nodes
// directly on your machine, for local testing.

var defaultConfigName = "localhost.toml"

// Localhost is the platform for launching thee apps locally
type Localhost struct {

	// Address of the logger (can be local or not)
	Logger string

	// The simulation to run
	Simulation string

	// Where is the Localhost package located
	LocalDir string
	// Where to build the executables +
	// where to read the config file
	// it will be assembled like LocalDir/RunDir
	RunDir string

	// Debug level 1 - 5
	Debug int

	// The number of deployed hosts
	Hosts int
	// Addresses used with the applications
	// example: localhost:2000, ...:2010 , ...
	Addresses []string

	// Whether we started a simulation
	running bool
	// WaitGroup for running processes
	wg_run sync.WaitGroup

	// errors go here:
	errChan chan error

	// SimulationConfig holds all things necessary for the run
	Sc *sda.SimulationConfig
}

// Configure various
func (d *Localhost) Configure() {
	pwd, _ := os.Getwd()
	d.RunDir = pwd + "/platform/localhost"
	d.LocalDir = pwd
	d.Debug = dbg.DebugVisible
	d.running = false
	d.errChan = make(chan error)
	if d.Simulation == "" {
		dbg.Fatal("No simulation defined in simulation")
	}
	dbg.Lvl3(fmt.Sprintf("Localhost dirs: RunDir %s", d.RunDir))
	dbg.Lvl3("Localhost configured ...")
}

// Will build the application
func (d *Localhost) Build(build string) error {
	src, _ := filepath.Rel(d.LocalDir, d.LocalDir+"/..")
	dst := d.RunDir + "/" + d.Simulation
	start := time.Now()
	// build for the local machine
	res, err := cliutils.Build(src, dst, runtime.GOARCH, runtime.GOOS)
	if err != nil {
		dbg.Fatal("Error while building for localhost (src", src, ", dst", dst, ":", res)
	}
	dbg.Lvl3("Localhost: Build src", src, ", dst", dst)
	dbg.Lvl4("Localhost: Results of localhost build:", res)
	dbg.Lvl2("Localhost: build finished in", time.Since(start))
	return err
}

func (d *Localhost) Cleanup() error {
	dbg.Lvl3("Cleaning up")
	ex := d.RunDir + "/" + d.Simulation
	err := exec.Command("pkill", "-f", ex).Run()
	if err != nil {
		dbg.Lvl3("Error stopping localhost", err)
	}

	// Wait for eventual connections to clean up
	time.Sleep(time.Second)
	return nil
}

func (d *Localhost) Deploy(rc RunConfig) error {
	dbg.Lvl2("Localhost: Deploying and writing config-files")
	sim, err := sda.NewSimulation(d.Simulation, string(rc.Toml()))
	if err != nil {
		return err
	}
	d.Sc, err = sim.Setup(d.RunDir, []string{"localhost"})
	if err != nil {
		return err
	}
	d.Sc.Config = string(rc.Toml())
	d.Sc.Save(d.RunDir)
	dbg.Lvl2("Localhost: Done deploying")
	return nil

}

func (d *Localhost) Start(args ...string) error {
	os.Chdir(d.RunDir)
	dbg.Lvl4("Localhost: chdir into", d.RunDir)
	ex := d.RunDir + "/" + d.Simulation
	dbg.Lvl4("Localhost: in Start() => hosts", d.Hosts)
	d.running = true
	dbg.Lvl1("Starting", len(d.Sc.EntityList.List), "applications of", ex)
	for index, entity := range d.Sc.EntityList.List {
		d.wg_run.Add(1)
		address := entity.Addresses[0]
		dbg.Lvl3("Starting", index, "=", address)
		cmdArgs := []string{"-address", address, "-monitor",
			"localhost:" + strconv.Itoa(monitor.SinkPort),
			"-simul", d.Simulation,
			"-start=" + strconv.FormatBool(index == 0),
			"-debug", strconv.Itoa(dbg.DebugVisible),
		}
		cmdArgs = append(args, cmdArgs...)
		dbg.Lvl3("CmdArgs are", cmdArgs)
		cmd := exec.Command(ex, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		go func(i int, h string) {
			dbg.Lvl3("Localhost: will start host", address)
			err := cmd.Run()
			if err != nil {
				dbg.Error("Error running localhost", h, ":", err)
				d.errChan <- err
			}
			d.wg_run.Done()
			dbg.Lvl3("host (index", i, ")", h, "done")
		}(index, address)
	}
	return nil
}

// Waits for all processes to finish
func (d *Localhost) Wait() error {
	dbg.Lvl3("Waiting for processes to finish")

	var err error
	go func() {
		d.wg_run.Wait()
		dbg.Lvl3("WaitGroup is 0")
		// write to error channel when done:
		d.errChan <- nil
	}()

	// if one of the hosts fails, stop waiting and return the error:
	select {
	case e := <-d.errChan:
		dbg.Lvl3("Finished waiting for hosts:", e)
		if e != nil {
			d.Cleanup()
			err = e
		}
	}

	dbg.Lvl2("Processes finished")
	return err
}
