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
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/codegangsta/cli"
	c "github.com/dedis/cothority/lib/config"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cothority/protocols"
	_ "github.com/dedis/cothority/services"
	"github.com/dedis/crypto/config"
)

const BIN = "cothorityd"
const SERVER_CONFIG = "config.toml"
const GROUP_DEF = "group.toml"
const VERSION = "1.1"

const CONNECTION_CHECKER = "http://www.canyouseeme.org/"

/*
Cothority is a general node that can be used for all available protocols.
*/

// Main starts the host and will setup the protocol.
func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "Cothorityd server"
	cliApp.Usage = "Serve a cothority"
	cliApp.Version = VERSION
	cliApp.Commands = []cli.Command{
		{
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "Setup the configuration for the server (interactive)",
			Action: func(c *cli.Context) {
				if c.String("config") != "" {
					stderrExit("[-] Configuration file option can't be used for the 'setup' command")
				}
				if c.String("debug") != "" {
					stderrExit("[-] Debug option can't be used for the 'setup' command")
				}
				interactiveConfig()
			},
		},
		{
			Name:  "server",
			Usage: "Run the cothority server",
			Action: func(c *cli.Context) {
				runServer(c)
			},
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: getDefaultConfigFile(),
			Usage: "Configuration file of the server",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	// default action
	cliApp.Action = func(c *cli.Context) {
		runServer(c)
	}

	cliApp.Run(os.Args)

}

func runServer(ctx *cli.Context) {
	// first check the options
	dbg.SetDebugVisible(ctx.Int("debug"))
	config := ctx.String("config")

	if _, err := os.Stat(config); os.IsNotExist(err) {
		dbg.Fatalf("[-] Configuration file does not exists. %s", config)
	}
	// Let's read the config
	_, host, err := c.ParseCothorityd(config)
	if err != nil {
		dbg.Fatal("Couldn't parse config:", err)
	}
	host.ListenAndBind()
	host.StartProcessMessages()
	host.WaitForClose()

}

// CreateCothoritydConfig will ask through the command line to create a Private / Public
// key, what is the listening address
func interactiveConfig() {
	fmt.Println("[+] Welcome ! Let's setup the configuration file for a cothority server...")

	fmt.Print("[*] We need to know on which [address:]PORT you want your server to listen to: ")
	reader := bufio.NewReader(os.Stdin)
	var str = readString(reader)

	// let's dissect the port / IP
	var hostStr string
	var ipProvided bool = true
	var portStr string
	var serverBinding string
	splitted := strings.Split(str, ":")

	// one element provided
	if len(splitted) == 1 {
		if _, err := strconv.Atoi(splitted[0]); err != nil {
			stderrExit("[-] You have to provide a port number at least!")
		}
		// ip
		ipProvided = false
		hostStr = "0.0.0.0"
		portStr = splitted[0]
	} else {
		hostStr = splitted[0]
		portStr = splitted[1]
	}

	// let's check if they are correct
	serverBinding = hostStr + ":" + portStr
	hostStr, portStr, err := net.SplitHostPort(serverBinding)
	if err != nil {
		stderrExit("[-] Invalid connection information for", serverBinding, " :", err)
	}
	if net.ParseIP(hostStr) == nil {
		stderrExit("[-] Invalid connection  information for", serverBinding)
	}

	fmt.Println("[+] We now need to get a reachable address for other cothority servers")
	fmt.Println("    and clients to contact you. This address will be put in a group definition")
	fmt.Println("    file that you can share and combine with others to form a Cothority roster.")

	var publicAddress string
	var failedPublic bool
	// if IP was not provided then let's get the public IP address
	if !ipProvided {
		resp, err := http.Get("http://myexternalip.com/raw")
		// cant get the public ip then ask the user for a reachable one
		if err != nil {
			stderr("[-] Could not get your public IP address")
			failedPublic = true
		} else {
			buff, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				stderr("[-] Could not parse your public IP address", err)
				failedPublic = true
			} else {
				publicAddress = strings.TrimSpace(string(buff)) + ":" + portStr
			}
		}
	} else {
	}

	var reachableAddress string
	// Let's directly ask the user for a reachable address
	if failedPublic {
		reachableAddress = askReachableAddress(reader, portStr)
	} else {
		// try  to connect to ipfound:portgiven
		tryIp := publicAddress
		fmt.Println("[+] Check if the address", tryIp, " is reachable from Internet...")
		if err := tryConnect(tryIp); err != nil {
			stderr("[-] Could not connect to your public IP")
			reachableAddress = askReachableAddress(reader, portStr)
		} else {
			reachableAddress = tryIp
			fmt.Println("[+] Address", reachableAddress, " publicly available from Internet!")
		}
	}

	// create the keys
	fmt.Println("\n[+] Creation of the ed25519 private and public keys...")
	kp := config.NewKeyPair(network.Suite)
	privStr, err := crypto.SecretHex(network.Suite, kp.Secret)
	if err != nil {
		stderrExit("[-] Error formating private key to hexadecimal. Abort.")
	}

	pubStr, err := crypto.PubHex(network.Suite, kp.Public)
	if err != nil {
		stderrExit("[-] Could not parse public key. Abort.")
	}

	fmt.Println("[+] Private:\t", privStr)
	fmt.Println("[+] Public: \t", pubStr, "\n")

	var configDone bool
	var configFile string
	var defaultFile = getDefaultConfigFile()
	for !configDone {
		// get name of config file and write to config file
		fmt.Println("[*] Name of the config file [", defaultFile, "]. Type <Enter> to use the default: ")
		configFile = readString(reader)
		if configFile == "" {
			configFile = defaultFile
		}

		// check if the directory exists
		var dirName = path.Dir(configFile)
		if _, err := os.Stat(dirName); os.IsNotExist(err) {
			fmt.Println("[+] Creating inexistant directory configuration", dirName)
			if err = os.MkdirAll(dirName, 0744); err != nil {
				stderrExit("[-] Could not create directory configuration", dirName, err)
			}
		}
		// check if the file exists and ask for override
		if _, err := os.Stat(configFile); err == nil {
			fmt.Println("[*] Configuration file already exists. Override ? (y/n) : ")
			var answer = readString(reader)
			answer = strings.ToLower(answer)
			if answer == "y" {
				configDone = true
				continue
			} else if answer == "n" {
				// let's try again
				continue
			} else {
				stderrExit("[-] Could not interpret your response. Abort.")
			}
		}
		configDone = true
	}

	conf := &c.CothoritydConfig{
		Public:    pubStr,
		Private:   privStr,
		Addresses: []string{serverBinding},
	}
	if err = conf.Save(configFile); err != nil {
		stderrExit("[-] Unable to write the config to file:", err)
	}
	fmt.Println("[+] Sucess ! You can now use the cothority server with the config file", configFile)

	// group definition part
	var dirName = path.Dir(configFile)
	var groupFile = path.Join(dirName, GROUP_DEF)
	serverToml := c.NewServerToml(network.Suite, kp.Public, reachableAddress)
	groupToml := c.NewGroupToml(serverToml)

	if err := groupToml.Save(groupFile); err != nil {
		stderrExit("[-] Could not write your group file snippet:", err)
	}

	fmt.Println("[+] Saved a group definition snippet for your server at", groupFile)
	fmt.Println(groupToml.String() + "\n")

	fmt.Println("[+] We're done ! Have good time using cothority :)")
}

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, fmt.Sprint(a...)+"\n")
}
func stderrExit(format string, a ...interface{}) {
	stderr(format, a...)
	os.Exit(1)
}

func getDefaultConfigFile() string {
	u, err := user.Current()
	// can't get the user dir, so fallback to currere ynt one
	if err != nil {
		fmt.Print("[-] Could not get your home's directory. Switching back to current dir.")
		if curr, err := os.Getwd(); err != nil {
			stderrExit("[-] Impossible to get the current directory.", err)
		} else {
			return path.Join(curr, SERVER_CONFIG)
		}
	}
	// let's try to stick to usual OS folders
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", BIN, SERVER_CONFIG)
	default:
		return path.Join(u.HomeDir, ".config", BIN, SERVER_CONFIG)
		// TODO WIndows ? FreeBSD ?
	}
}

func readString(reader *bufio.Reader) string {
	str, err := reader.ReadString('\n')
	if err != nil {
		stderrExit("[-] Could not read input.")
	}
	return strings.TrimSpace(str)
}

func askReachableAddress(reader *bufio.Reader, port string) string {
	fmt.Print("[+] Enter the IP address you would like others cothority servers and client to contact you: ")
	ipStr := readString(reader)

	splitted := strings.Split(ipStr, ":")
	if len(splitted) == 2 && splitted[1] != port {
		// if the client gave a port number, it must be the same
		stderrExit("[-] The port you gave is not the same as the one your server will be listening. Abort.")
	} else if len(splitted) == 2 && net.ParseIP(splitted[0]) != nil {
		// of if the IP address is wrong
		stderrExit("[-] Invalid IP address given (", ipStr, ")")
	} else {
		// check if the ip is valid
		if net.ParseIP(ipStr) == nil {
			stderrExit("[-] Invalid IP address given (", ipStr, ")")
		}
		// add the port
		ipStr = ipStr + ":" + port
	}
	return ipStr
}

// Service used to get the port connection service
const WHATS_MY_IP = "http://www.whatsmyip.org/"

// tryConnect will bind to the ip address and ask a internet service to try to
// connect to it
func tryConnect(ip string) error {

	stopCh := make(chan bool, 1)
	// let's bind
	go func() {
		ln, err := net.Listen("tcp", ip)
		if err != nil {
			fmt.Println("[-] Trouble with binding to the address:", err)
			return
		}
		con, _ := ln.Accept()
		<-stopCh
		con.Close()
	}()
	defer func() { stopCh <- true }()

	_, port, err := net.SplitHostPort(ip)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("port", port)
	values.Set("timeout", "default")

	// ask the check
	url := WHATS_MY_IP + "port-scanner/scan.php"
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Host", "www.whatsmyip.org")
	req.Header.Set("Referer", "http://www.whatsmyip.org/port-scanner/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:46.0) Gecko/20100101 Firefox/46.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	buffer, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if !bytes.Contains(buffer, []byte("1")) {
		return errors.New("Address unrechable")
	}
	return nil
}
