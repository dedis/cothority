package server

import (
	"bufio"
	"bytes"
	"errors"
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

	c "github.com/dedis/cothority/lib/config"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"gopkg.in/codegangsta/cli.v1"
	// Empty imports to have the init-functions called which should
	// register the protocol

	"regexp"

	"github.com/dedis/cosi/lib"
	_ "github.com/dedis/cosi/protocol"
	_ "github.com/dedis/cosi/service"
	"github.com/dedis/cothority/lib/oi"
	"github.com/dedis/crypto/config"
)

// DefaultServerConfig is the name of the default file to lookup for server
// configuration file
const DefaultServerConfig = "config.toml"

// DefaultGroupFile is the name of the default file to lookup for group
// definition
const DefaultGroupFile = "group.toml"

// DefaultPort where to listen; At time of writing, this port is not listed in
// /etc/services
const DefaultPort = 6879

// DefaultAddress where to be contacted by other servers
const DefaultAddress = "127.0.0.1"

// Service used to get the port connection service
const whatsMyIP = "http://www.whatsmyip.org/"

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	if _, err := os.Stat(config); os.IsNotExist(err) {
		dbg.Fatalf("[-] Configuration file does not exist. %s. "+
			"Use `cosi server setup` to create one.", config)
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

// interactiveConfig will ask through the command line to create a Private / Public
// key, what is the listening address
func InteractiveConfig(binaryName string) {
	oi.Info("Setting up a cothority-server.")
	oi.Inputf("Please enter the [address:]PORT for incoming requests [%d]: ", DefaultPort)
	reader := bufio.NewReader(os.Stdin)
	var str = readString(reader)
	// let's dissect the port / IP
	var hostStr string
	var ipProvided = true
	var portStr string
	var serverBinding string
	splitted := strings.Split(str, ":")

	if str == "" {
		portStr = strconv.Itoa(DefaultPort)
		hostStr = "0.0.0.0"
		ipProvided = false
	} else if len(splitted) == 1 {
		// one element provided
		if _, err := strconv.Atoi(splitted[0]); err != nil {
			oi.Fatal("You have to provide a port number at least!")
		}
		// ip
		ipProvided = false
		hostStr = "0.0.0.0"
		portStr = splitted[0]
	} else if len(splitted) == 2 {
		hostStr = splitted[0]
		portStr = splitted[1]
	}
	// let's check if they are correct
	serverBinding = hostStr + ":" + portStr
	hostStr, portStr, err := net.SplitHostPort(serverBinding)
	if err != nil {
		oi.Fatalf("Invalid connection information for %s: %v", serverBinding, err)
	}
	if net.ParseIP(hostStr) == nil {
		oi.Fatal("Invalid connection  information for", serverBinding)
	}

	oi.Info("We now need to get a reachable address for other CoSi servers")
	oi.Info("and clients to contact you. This address will be put in a group definition")
	oi.Info("file that you can share and combine with others to form a Cothority roster.")

	var publicAddress string
	var failedPublic bool
	// if IP was not provided then let's get the public IP address
	if !ipProvided {
		resp, err := http.Get("http://myexternalip.com/raw")
		// cant get the public ip then ask the user for a reachable one
		if err != nil {
			oi.Error("Could not get your public IP address")
			failedPublic = true
		} else {
			buff, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				oi.Error("Could not parse your public IP address", err)
				failedPublic = true
			} else {
				publicAddress = strings.TrimSpace(string(buff)) + ":" + portStr
			}
		}
	} else {
		publicAddress = serverBinding
	}

	// Let's directly ask the user for a reachable address
	if failedPublic {
		publicAddress = askReachableAddress(reader, portStr)
	} else {
		if isPublicIP(publicAddress) {
			// try  to connect to ipfound:portgiven
			tryIP := publicAddress
			oi.Info("Check if the address", tryIP, "is reachable from Internet...")
			if err := tryConnect(tryIP, serverBinding); err != nil {
				oi.Error("Could not connect to your public IP")
				publicAddress = askReachableAddress(reader, portStr)
			} else {
				publicAddress = tryIP
				oi.Info("Address", publicAddress, "is publicly available from Internet.")
			}
		}
	}

	// create the keys
	privStr, pubStr := createKeyPair()
	conf := &c.CothoritydConfig{
		Public:    pubStr,
		Private:   privStr,
		Addresses: []string{serverBinding},
	}

	var configDone bool
	var configFolder string
	var defaultFolder = path.Dir(getDefaultConfigFile(binaryName))
	var configFile string
	var groupFile string

	for !configDone {
		// get name of config file and write to config file
		oi.Input("Please enter a folder for the configuration files [ " +
			defaultFolder + " ] :")
		configFolder = readString(reader)
		if configFolder == "" {
			configFolder = defaultFolder
		}
		configFile = path.Join(configFolder, DefaultServerConfig)
		groupFile = path.Join(configFolder, DefaultGroupFile)

		// check if the directory exists
		if _, err := os.Stat(configFolder); os.IsNotExist(err) {
			oi.Info("Creating inexistant directory configuration", configFolder)
			if err = os.MkdirAll(configFolder, 0744); err != nil {
				oi.Fatalf("Could not create directory configuration %s %v", configFolder, err)
			}
		}

		if checkOverwrite(configFile, reader) && checkOverwrite(groupFile, reader) {
			break
		}
	}

	public, err := crypto.ReadPubHex(network.Suite, pubStr)
	if err != nil {
		oi.Fatal("Impossible to parse public key:", err)
	}

	server := c.NewServerToml(network.Suite, public, publicAddress)
	group := c.NewGroupToml(server)

	saveFiles(conf, configFile, group, groupFile)
	oi.Info("All configurations saved, ready to serve signatures now.")
}

func isPublicIP(ip string) bool {
	public, err := regexp.MatchString("(^127\\.)|(^10\\.)|"+
		"(^172\\.1[6-9]\\.)|(^172\\.2[0-9]\\.)|"+
		"(^172\\.3[0-1]\\.)|(^192\\.168\\.)", ip)
	if err != nil {
		dbg.Error(err)
	}
	return !public
}

// Returns true if file exists and user is OK to overwrite, or file dont exists
// Return false if file exists and user is NOT OK to overwrite.
func checkOverwrite(file string, reader *bufio.Reader) bool {
	// check if the file exists and ask for override
	if _, err := os.Stat(file); err == nil {
		oi.Input("Configuration file " + file + " already exists. Override ? [Yn]: ")
		var answer = readString(reader)
		return strings.ToLower(answer) != "n"
	}
	return true
}

// createKeyPair returns the private and public key hexadecimal representation
func createKeyPair() (string, string) {
	oi.Info("Creating ed25519 private and public keys.")
	kp := config.NewKeyPair(network.Suite)
	privStr, err := crypto.SecretHex(network.Suite, kp.Secret)
	if err != nil {
		oi.Fatal("Error formating private key to hexadecimal. Abort.")
	}
	// use the transformation for ed25519 signatures
	point := cosi.Ed25519Public(network.Suite, kp.Secret)
	pubStr, err := crypto.PubHex(network.Suite, point)
	if err != nil {
		oi.Fatal("Could not parse public key. Abort.")
	}

	oi.Info("Public key: ", pubStr, "\n")
	return privStr, pubStr
}

func saveFiles(conf *c.CothoritydConfig, fileConf string, group *c.GroupToml, fileGroup string) {
	if err := conf.Save(fileConf); err != nil {
		oi.Fatal("Unable to write the config to file:", err)
	}
	oi.Info("Sucess! You can now use the CoSi server with the config file", fileConf)
	// group definition part
	if err := group.Save(fileGroup); err != nil {
		oi.Fatal("Could not write your group file snippet:", err)
	}

	oi.Info("Saved a group definition snippet for your server at", fileGroup,
		group.String())

}

func getDefaultConfigFile(binaryName string) string {
	u, err := user.Current()
	// can't get the user dir, so fallback to current working dir
	if err != nil {
		oi.Error("Could not get your home-directory (", err.Error(), "). Switching back to current dir.")
		if curr, err := os.Getwd(); err != nil {
			oi.Fatal("Impossible to get the current directory:", err)
		} else {
			return path.Join(curr, DefaultServerConfig)
		}
	}
	// let's try to stick to usual OS folders
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", binaryName, DefaultServerConfig)
	default:
		return path.Join(u.HomeDir, ".config", binaryName, DefaultServerConfig)
		// TODO WIndows ? FreeBSD ?
	}
}

func readString(reader *bufio.Reader) string {
	str, err := reader.ReadString('\n')
	if err != nil {
		oi.Fatal("Could not read input.")
	}
	return strings.TrimSpace(str)
}

func askReachableAddress(reader *bufio.Reader, port string) string {
	oi.Input("IP-address where your server can be reached [", DefaultAddress, "]:")
	ipStr := readString(reader)
	if ipStr == "" {
		return DefaultAddress + ":" + port
	}

	splitted := strings.Split(ipStr, ":")
	if len(splitted) == 2 && splitted[1] != port {
		// if the client gave a port number, it must be the same
		oi.Fatal("The port you gave is not the same as the one your server will be listening. Abort.")
	} else if len(splitted) == 2 && net.ParseIP(splitted[0]) == nil {
		// of if the IP address is wrong
		oi.Fatal("Invalid IP:port address given:", ipStr)
	} else if len(splitted) == 1 {
		// check if the ip is valid
		if net.ParseIP(ipStr) == nil {
			oi.Fatal("Invalid IP address given:", ipStr)
		}
		// add the port
		ipStr = ipStr + ":" + port
	}
	return ipStr
}

// tryConnect will bind to the ip address and ask a internet service to try to
// connect to it. binding is the address where we must listen (needed because
// the reachable address might not be the same as the binding address => NAT, ip
// rules etc).
func tryConnect(ip string, binding string) error {

	stopCh := make(chan bool, 1)
	// let's bind
	go func() {
		ln, err := net.Listen("tcp", binding)
		if err != nil {
			oi.Error("Trouble with binding to the address:", err)
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
	url := whatsMyIP + "port-scanner/scan.php"
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
