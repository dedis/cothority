package schnorr_sign

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/deploy"
	"github.com/dedis/cothority/lib/config"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io/ioutil"
	//"os"
)

// Dispatch-function for running either client or server (mode-parameter)
func Run(app *config.AppConfig, depl *deploy.Config) {

	// we must know who we are
	if app.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	dbg.Lvl2(app.Hostname, "Starting to run as ", app.Mode)
	var err error
	hosts, err := ReadHostsJson("tree.json")
	if err != nil {
		log.Fatal("Error while reading JSON hosts file on", app.Hostname, ". Abort")
	}
	switch app.Mode {
	case "client":
		RunClient(depl)
	case "server":
		RunServer(hosts, app, depl)
	}
}

// Read the tree json file and return the configFileold containing every hosts name
func ReadHostsJson(file string) (*config.HostsConfig, error) {
	var cf config.ConfigFileOld
	bFile, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bFile, &cf)
	if err != nil {
		return nil, err
	}
	return &config.HostsConfig{
		Conn:  cf.Conn,
		Hosts: cf.Hosts,
	}, nil
}
