/*
Cothority-SDA is a framework that allows testing, simulating and
deploying crypto-related protocols.

# Running a simulation

Your best starting point is the simul/-directory where you can start different
protocols and test them on your local computer. After building, you can
start any protocol you like:
	cd simul
	go build
	./simul runfiles/test_cosi.toml
Once a simulation is done, you can look at the results in the test_data-directory:
	cat test_data/test_cosi.csv
To have a simple plot of the round-time, you need to have matplotlib installed
in version 1.5.1.
	matplotlib/plot.py test_data/test_cosi.csv
If plot.py complains about missing matplotlib-library, you can install it using
	sudo easy_install "matplotlib == 1.5.1"
at least on a Mac.

# Writing your own protocol

If you want to experiment with a protocol of your own, have a look at
the protocols-package-documentation.

# Deploying

Unfortunately it's not possible to deploy the Cothority as a stand-alon app.
Well, it is possible, but there is no way to start a protocol.
*/
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/dedis/cothority/lib/cliutils"
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

	dbg.Lvl1("Starting Host ...")
	dbg.Lvl1("	... Addresses:", host.Entity.Addresses)
	// print public key in hex
	var buff bytes.Buffer
	err = cliutils.WritePub64(network.Suite, &buff, host.Entity.Public)
	if err != nil {
		dbg.Fatal("Unknown error:", err)
	}
	dbg.Lvl1("	... Public key:", buff.String())

	host.Listen()
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
	str1 := strings.TrimSpace(str)
	_, _, errStr := net.SplitHostPort(str1)
	if err != nil || errStr != nil {
		fmt.Println("[-] Error reading IP:PORT (", str1, ") ", errStr, " => Abort")
		os.Exit(1)
	}

	ip := str1

	// File output
	fmt.Println("[*] Name of the file to output the configuration of this host:")
	str, err = reader.ReadString('\n')
	str = strings.TrimSpace(str)

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
