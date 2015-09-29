package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/config"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io/ioutil"
	//"os"
	"github.com/dedis/cothority/lib/app"
)

// Dispatch-function for running either client or server (mode-parameter)
func main() {
	ac := app.ReadAppConfig()

	// we must know who we are
	if ac.Flags.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	dbg.Lvl2(ac.Flags.Hostname, "Starting to run as ", ac.Flags.Mode)
	var err error
	hosts, err := ReadHostsJson("tree.json")
	if err != nil {
		log.Fatal("Error while reading JSON hosts file on", ac.Flags.Hostname, ". Abort")
	}
	switch ac.Flags.Mode {
	case "client":
		RunClient(ac.Conf)
	case "server":
		RunServer(hosts, ac)
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
