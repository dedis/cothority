package platform

import (
	"fmt"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/lib/monitor"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Localhost is responsible for launching the app with the specified number of nodes
// directly on your machine, for local testing.

var defaultConfigName = "localhost.toml"

// Localhost is the platform for launching thee apps locally
type Localhost struct {

	// Address of the logger (can be local or not)
	Logger string

	// App to run [shamir,coll_sign..]
	App string
	// where the app is located
	AppDir string

	// Where is the Localhost package located
	LocalDir string
	// Where to build the executables +
	// where to read the config file
	// it will be assembled like LocalDir/RunDir
	RunDir string

	// Debug level 1 - 5
	Debug int

	// Number of machines - so we can use the same
	// configuration-files
	Machines int
	// This gives the number of hosts per node (machine)
	Ppm int
	// hosts used with the applications
	// example: localhost:2000, ...:2010 , ...
	Hosts []string

	// Whether we started a simulation
	running bool
	// WaitGroup for running processes
	wg_run sync.WaitGroup
}

// Configure various
func (d *Localhost) Configure() {
	pwd, _ := os.Getwd()
	d.AppDir = pwd + "/../app"
	d.RunDir = pwd + "/platform/localhost"
	d.LocalDir = pwd
	d.Debug = dbg.DebugVisible
	d.running = false
	if d.App == "" {
		dbg.Fatal("No app defined in simulation")
	}
	dbg.Lvl3(fmt.Sprintf("Localhost dirs : AppDir %s, RunDir %s", d.AppDir, d.RunDir))
	dbg.Lvl3("Localhost configured ...")
}

// Will build the application
func (d *Localhost) Build(build string) error {
	src, _ := filepath.Rel(d.LocalDir, d.AppDir+"/"+d.App)
	dst := d.RunDir + "/" + d.App
	start := time.Now()
	// build for the local machine
	res, err := cliutils.Build(src, dst, runtime.GOARCH, runtime.GOOS)
	if err != nil {
		dbg.Fatal("Error while building for localhost (src ", src, ", dst ", dst, " : ", res)
	}
	dbg.Lvl3("Localhost : Build src", src, ", dst", dst)
	dbg.Lvl3("Localhost : Results of localhost build :", res)
	dbg.Lvl1("Localhost : build finished in", time.Since(start))
	return err
}

func (d *Localhost) Cleanup() error {
	ex := d.RunDir + "/" + d.App
	err := exec.Command("pkill", "-f", ex).Run()
	if err != nil {
		dbg.Lvl3("Error stopping localhost", err)
	}

	// Wait for eventual connections to clean up
	time.Sleep(time.Second)
	return nil
}

func (d *Localhost) Deploy(rc RunConfig) error {
	dbg.Lvl1("Localhost : Writing config-files")

	// Initialize the deter-struct with our current structure (for debug-levels
	// and such), then read in the app-configuration to overwrite eventual
	// 'Machines', 'Ppm', 'Loggers' or other fields
	appConfig := d.RunDir + "/app.toml"
	localConfig := d.RunDir + "/" + defaultConfigName
	ioutil.WriteFile(appConfig, rc.Toml(), 0666)
	d.ReadConfig(appConfig)
	d.GenerateHosts()

	app.WriteTomlConfig(d, localConfig)

	// Prepare special configuration preparation for each application - the
	// reading in twice of the configuration file, once for the deterConfig,
	// then for the appConfig, sets the deterConfig as defaults and overwrites
	// everything else with the actual appConfig (which comes from the
	// runconfig-file)
	switch d.App {
	case "sign", "stamp":
		conf := app.ConfigColl{}
		app.ReadTomlConfig(&conf, localConfig)
		app.ReadTomlConfig(&conf, appConfig)
		// Calculates a tree that is used for the timestampers
		// ppm = 1
		conf.Tree = graphs.CreateLocalTree(d.Hosts, conf.Bf)
		conf.Hosts = d.Hosts

		dbg.Lvl2("Depth:", graphs.Depth(conf.Tree))
		dbg.Lvl2("Total hosts:", len(conf.Hosts))
		total := d.Machines * d.Ppm
		if len(conf.Hosts) != total {
			dbg.Fatal("Only calculated", len(conf.Hosts), "out of", total, "hosts - try changing number of",
				"machines or hosts per node")
		}
		d.Hosts = conf.Hosts
		// re-write the new configuration-file
		app.WriteTomlConfig(conf, appConfig)
	case "shamir":
		conf := app.ConfigShamir{}
		app.ReadTomlConfig(&conf, localConfig)
		app.ReadTomlConfig(&conf, appConfig)
		//_, conf.Hosts, _, _ = graphs.TreeFromList(d.Hosts, len(d.Hosts), 1)
		//d.Hosts = conf.Hosts
		dbg.Lvl4("Localhost : graphs.Tree for shamir ", conf.Hosts)
		// re-write the new configuration-file
		app.WriteTomlConfig(conf, appConfig)
	case "naive":
		conf := app.NaiveConfig{}
		app.ReadTomlConfig(&conf, localConfig)
		app.ReadTomlConfig(&conf, appConfig)
		dbg.Lvl4("Localhost : naive applications :", conf.Hosts)
		app.WriteTomlConfig(conf, appConfig)
	case "ntree":
		conf := app.NTreeConfig{}
		app.ReadTomlConfig(&conf, localConfig)
		app.ReadTomlConfig(&conf, appConfig)
		conf.Tree = graphs.CreateLocalTree(d.Hosts, conf.Bf)
		conf.Hosts = d.Hosts
		dbg.Lvl3("Localhost : naive Tree applications :", conf.Hosts)
		d.Hosts = conf.Hosts
		app.WriteTomlConfig(conf, appConfig)
	case "randhound":
	}
	//app.WriteTomlConfig(d, defaultConfigName, d.RunDir)
	debug := reflect.ValueOf(d).Elem().FieldByName("Debug")
	if debug.IsValid() {
		dbg.DebugVisible = debug.Interface().(int)
	}
	dbg.Lvl1("Localhost : Done deploying")

	return nil

}

func (d *Localhost) Start() error {
	os.Chdir(d.RunDir)
	dbg.Lvl4("Localhost : chdir into ", d.RunDir)
	ex := d.RunDir + "/" + d.App
	dbg.Lvl4("Localhost: in Start() => hosts ", d.Hosts)
	d.running = true
	dbg.Lvl1("Starting", len(d.Hosts), "applications of", ex)
	for index, host := range d.Hosts {
		args := []string{"-hostname", host, "-mode", "server", "-logger", "localhost:" + monitor.SinkPort}
		cmd := exec.Command(ex, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		go func(i int, h string) {
			dbg.Lvl3("Localhost : will start host ", host)
			d.wg_run.Add(1)
			err := cmd.Run()
			if err != nil {
				dbg.Lvl3("Error running localhost ", h, " : ", err)
			}
			d.wg_run.Done()
			dbg.Lvl3("host (index ", i, ") ", h, "done")
		}(index, host)

	}
	return nil
}

// Waits for all processes to finish
func (d *Localhost) Wait() error {
	dbg.Lvl3("Waiting for processes to finish")
	d.wg_run.Wait()
	dbg.Lvl2("Processes finished")
	return nil
}

// Reads in the localhost-config and drops out if there is an error
func (d *Localhost) ReadConfig(name ...string) {
	configName := defaultConfigName
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
	dbg.Lvl4("Localhost : read the config, Hosts ", d.Hosts)
}

// GenerateHosts will generate the list of hosts
// with a new port each
func (d *Localhost) GenerateHosts() {
	nrhosts := d.Machines * d.Ppm
	d.Hosts = make([]string, nrhosts)
	port := 2000
	inc := 5
	for i := 0; i < nrhosts; i++ {
		s := "127.0.0.1:" + strconv.Itoa(port+inc*i)
		d.Hosts[i] = s
	}
	dbg.Lvl4("Localhost: Generated hosts list ", d.Hosts)
}
