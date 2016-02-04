package platform

import (
	"fmt"
	"os"
	"os/exec"
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
	logger string

	// The simulation to run
	simulation string

	// Where is the Localhost package located
	localDir string
	// Where to build the executables +
	// where to read the config file
	// it will be assembled like LocalDir/RunDir
	runDir string

	// Debug level 1 - 5
	debug int

	// The number of deployed hosts
	hosts int
	// Addresses used with the applications
	// example: localhost:2000, ...:2010 , ...
	addresses []string

	// Whether we started a simulation
	running bool
	// WaitGroup for running processes
	wg_run sync.WaitGroup

	// errors go here:
	errChan chan error

	// SimulationConfig holds all things necessary for the run
	sc *sda.SimulationConfig
}

// Configure various
func (d *Localhost) Configure() {
	pwd, _ := os.Getwd()
	d.runDir = pwd + "/platform/localhost"
	d.localDir = pwd
	d.debug = dbg.DebugVisible
	d.running = false
	d.errChan = make(chan error)
	if d.simulation == "" {
		dbg.Fatal("No simulation defined in simulation")
	}
	dbg.Lvl3(fmt.Sprintf("Localhost dirs: RunDir %s", d.runDir))
	dbg.Lvl3("Localhost configured ...")
}

// Will build the application
func (d *Localhost) Build(build string) error {
	src := "./cothority"
	dst := d.runDir + "/" + d.simulation
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
	ex := d.runDir + "/" + d.simulation
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
	sim, err := sda.NewSimulation(d.simulation, string(rc.Toml()))
	if err != nil {
		return err
	}
	d.sc, err = sim.Setup(d.runDir, []string{"localhost"})
	if err != nil {
		return err
	}
	d.sc.Config = string(rc.Toml())
	d.sc.Save(d.runDir)
	dbg.Lvl2("Localhost: Done deploying")
	return nil

}

func (d *Localhost) Start(args ...string) error {
	os.Chdir(d.runDir)
	dbg.Lvl4("Localhost: chdir into", d.runDir)
	ex := d.runDir + "/" + d.simulation
	dbg.Lvl4("Localhost: in Start() => hosts", d.hosts)
	d.running = true
	dbg.Lvl1("Starting", len(d.sc.EntityList.List), "applications of", ex)
	for index, entity := range d.sc.EntityList.List {
		d.wg_run.Add(1)
		address := entity.Addresses[0]
		dbg.Lvl3("Starting", index, "=", address)
		cmdArgs := []string{"-address", address, "-monitor",
			"localhost:" + strconv.Itoa(monitor.SinkPort),
			"-simul", d.simulation,
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
