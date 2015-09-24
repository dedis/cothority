package main

import
(
	"flag"
	"os/exec"
	"strconv"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"os"
	"github.com/dedis/cothority/lib/cliutils"
	"net"
	"github.com/dedis/cothority/lib/config"
	"encoding/json"
	"io/ioutil"
	"github.com/dedis/cothority/lib/graphs"
	"sync"
	"github.com/dedis/cothority/lib/deploy"
)

// Wrapper around exec.go to enable measuring of cpu time

var deter *deploy.Deter
var conf *deploy.Config
var logger string
var physaddr string
var testConnect bool

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
}

func main() {
	conf := deploy.Config{}
	err := deploy.ReadConfig(&deter, "deploy.toml")
	if err != nil {
		log.Fatal("Couldn't load config-file in forkexec:", err)
	}
	dbg.DebugVisible = conf.Debug

	flag.Parse()

	// connect with the logging server
	if logger != "" {
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(logger, physaddr, conf.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
	}

	setup_deter()

	i := 0
	var wg sync.WaitGroup
	virts := physToServer[physaddr]
	if len(virts) > 0 {
		dbg.Lvl3("starting timestampers for", len(virts), "client(s)", virts)
		i = (i + 1) % len(loggerports)
		for _, name := range virts {
			dbg.Lvl4("Starting", name, "on", physaddr)
			wg.Add(1)
			go func(nameport string) {
				dbg.Lvl3("Running on", physaddr, "starting", nameport)
				defer wg.Done()

				args := []string{
					"-hostname=" + nameport,
					"-logger=" + logger,
					"-physaddr=" + physaddr,
					"-amroot=" + strconv.FormatBool(nameport == rootname),
					"-test_connect=" + strconv.FormatBool(testConnect),
					"-mode=server",
					"-app=" + conf.App,
				}

				dbg.Lvl3("Starting on", physaddr, "with args", args)
				cmdApp := exec.Command("./app", args...)
				//cmd.Stdout = log.StandardLogger().Writer()
				//cmd.Stderr = log.StandardLogger().Writer()
				cmdApp.Stdout = os.Stdout
				cmdApp.Stderr = os.Stderr
				dbg.Lvl3("fork-exec is running command:", args)
				err = cmdApp.Run()
				if err != nil {
					dbg.Lvl2("cmd run:", err)
				}

				// get CPU usage stats
				st := cmdApp.ProcessState.SystemTime()
				ut := cmdApp.ProcessState.UserTime()
				log.WithFields(log.Fields{
					"file":     logutils.File(),
					"type":     "forkexec",
					"systime":  st,
					"usertime": ut,
				}).Info("")

				dbg.Lvl2("Finished with Timestamper", physaddr)
			}(name)
		}
		dbg.Lvl3(physaddr, "Finished starting timestampers")
		wg.Wait()
	} else {
		dbg.Lvl2("No timestampers for", physaddr)
	}
	dbg.Lvl2(physaddr, "timestampers exited")
}

var physToServer map[string][]string
var loggerports []string
var random_leaf string
var rootname string

func setup_deter() {
	virt, err := cliutils.ReadLines("virt.txt")
	if err != nil {
		log.Fatal(err)
	}
	phys, err := cliutils.ReadLines("phys.txt")
	if err != nil {
		log.Fatal(err)
	}
	vpmap := make(map[string]string)
	for i := range virt {
		vpmap[virt[i]] = phys[i]
	}
	nloggers := conf.Nloggers
	masterLogger := phys[0]
	loggers := []string{masterLogger}
	for n := 1; n <= nloggers; n++ {
		loggers = append(loggers, phys[n])
	}

	phys = phys[nloggers:]
	virt = virt[nloggers:]

	// Read in and parse the configuration file
	file, err := ioutil.ReadFile("tree.json")
	if err != nil {
		log.Fatal("deter.go: error reading configuration file: %v\n", err)
	}
	dbg.Lvl4("cfg file:", string(file))
	var cf config.ConfigFile
	err = json.Unmarshal(file, &cf)
	if err != nil {
		log.Fatal("unable to unmarshal config.ConfigFile:", err)
	}

	hostnames := cf.Hosts
	dbg.Lvl4("hostnames:", hostnames)

	depth := graphs.Depth(cf.Tree)
	cf.Tree.TraverseTree(func(t *graphs.Tree) {
		if random_leaf != "" {
			return
		}
		if len(t.Children) == 0 {
			random_leaf = t.Name
		}
	})

	rootname = hostnames[0]

	dbg.Lvl4("depth of tree:", depth)

	// mapping from physical node name to the timestamp servers that are running there
	// essentially a reverse mapping of vpmap except ports are also used
	physToServer = make(map[string][]string)
	for _, virt := range hostnames {
		v, _, _ := net.SplitHostPort(virt)
		p := vpmap[v]
		ss := physToServer[p]
		ss = append(ss, virt)
		physToServer[p] = ss
	}
	dbg.Lvl3("PhysToServer is", physToServer)

	loggerports = make([]string, len(loggers))
	for i, logger := range loggers {
		loggerports[i] = logger + ":10000"
	}

}