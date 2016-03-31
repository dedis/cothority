package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cothority/protocols"
	"github.com/dedis/crypto/config"
)

/*
Cothority is a general node that can be used for all available protocols.
*/

// ConfigFile represents the configuration for a standalone run
var ConfigFile string
var debugVisible int

// DefaultConfName is the default configuration file-name for cosid (stores the
// generated private/public key and the host's address)
const DefaultConfName = "config.toml"

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&ConfigFile, "config", "config.toml", "which config-file to use")
	flag.IntVar(&debugVisible, "debug", 1, "verbosity: 0-5")
}

const releaseVersion = "1.0-alpha"

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	dbg.SetDebugVisible(debugVisible)
	dbg.Lvl1("Starting cothority daemon", releaseVersion)
	// We're in standalone mode and only start the node
	var host *sda.Host
	var err error
	host, err = sda.NewHostFromFile(ConfigFile)
	// if the file does not exists
	if os.IsNotExist(err) {
		// then ask informations to create one
		host = createHost()
	} else if err != nil {
		dbg.Lvl1("Config file is invalid. (", err, ") Fix it or delete it and re-start to create a new one")
		os.Exit(1)
	}

	dbg.Lvl1("Starting Host: You can copy the following lines in a servers.toml file to use by app/cosi client:")
	serverToml := app.NewServerToml(host.Suite(), host.Entity.Public, host.Entity.Addresses...)
	groupToml := app.NewGroupToml(serverToml)
	fmt.Println(groupToml.String())

	host.ListenNoblock()
	host.StartProcessMessages()
	host.WaitForClose()
}

// createHost will ask for the public IP:PORT of the host we want to create and
// the name of the file we want to output this config so it can be re-used
// later.
// The IP:PORT pair *must* be accessible from the Internet as other Hosts will
// try to contact it.
func createHost() *sda.Host {
	fmt.Println("[-] Configuration file does not exists")
	reader := bufio.NewReader(os.Stdin)
	var err error
	var str string
	// IP:PORT
	fmt.Println("[*] Type the IP:PORT (ipv4) address of this host (accessible from Internet):")
	str, err = reader.ReadString('\n')
	str = strings.TrimSpace(str)
	_, _, errStr := net.SplitHostPort(str)
	if err != nil || errStr != nil {
		fmt.Println("[-] Error reading IP:PORT (", str, ") ", errStr, " => Abort")
		os.Exit(1)
	}

	ip := str

	// File output
	fmt.Println("[*] Name of the file to output the configuration of this host (or default: [config.toml]):")
	str, err = reader.ReadString('\n')
	str = strings.TrimSpace(str)
	if str == "" {
		str = DefaultConfName
	}

	// create the public / private keys
	kp := config.NewKeyPair(network.Suite)
	entity := network.NewEntity(kp.Public, ip)
	host := sda.NewHost(entity, kp.Secret)

	// write to the file
	if err = host.SaveToFile(str); err != nil {
		fmt.Println("[-] Error writing config to file:", err, " => Abort.")
		os.Exit(1)
	}
	fmt.Println("[+] Host written to file", str)
	return host
}
