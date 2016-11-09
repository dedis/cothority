package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	"time"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"

	// CoSi-protocol is not part of the cothority.
	"github.com/dedis/crypto/cosi"
	// Empty imports to have the init-functions called which should
	// register the protocol.
	_ "github.com/dedis/cothority/protocols/cosi"
	// For the moment, the server only serves CoSi requests
	s "github.com/dedis/cothority/services/cosi"
	"github.com/dedis/crypto/abstract"
	crypconf "github.com/dedis/crypto/config"
)

// DefaultServerConfig is the default server configuration file-name.
const DefaultServerConfig = "config.toml"

// DefaultGroupFile is the default group definition file-name.
const DefaultGroupFile = "group.toml"

// DefaultPort to listen and connect to. As of this writing, this port is not listed in
// /etc/services
const DefaultPort = 6879

// DefaultAddress where to be contacted by other servers.
const DefaultAddress = "127.0.0.1"

// Service used to get the public IP-address.
const whatsMyIP = "http://www.whatsmyip.org/"

// RequestTimeOut is how long we're willing to wait for a signature.
var RequestTimeOut = time.Second * 1

// InteractiveConfig uses stdin to get the [address:]PORT of the server.
// If no address is given, whatsMyIP is used to find the public IP. In case
// no public IP can be configured, localhost will be used.
// If everything is OK, the configuration-files will be written.
// In case of an error this method Fatals.
func InteractiveConfig(binaryName string) {
	log.Info("Setting up a cothority-server.")
	str := config.Inputf(strconv.Itoa(DefaultPort), "Please enter the [address:]PORT for incoming requests")
	// let's dissect the port / IP
	var hostStr string
	var ipProvided = true
	var portStr string
	var serverBinding network.Address
	if !strings.Contains(str, ":") {
		str = ":" + str
	}
	host, port, err := net.SplitHostPort(str)
	log.ErrFatal(err, "Couldn't interpret", str)

	if str == "" {
		portStr = strconv.Itoa(DefaultPort)
		hostStr = "0.0.0.0"
		ipProvided = false
	} else if host == "" {
		// one element provided
		// ip
		ipProvided = false
		hostStr = "0.0.0.0"
		portStr = port
	} else {
		hostStr = host
		portStr = port
	}

	serverBinding = network.NewTCPAddress(hostStr + ":" + portStr)
	if !serverBinding.Valid() {
		log.Error("Unable to validate address given", serverBinding)
		return
	}

	log.Info("We now need to get a reachable address for other CoSi servers")
	log.Info("and clients to contact you. This address will be put in a group definition")
	log.Info("file that you can share and combine with others to form a Cothority roster.")

	var publicAddress network.Address
	var failedPublic bool
	// if IP was not provided then let's get the public IP address
	if !ipProvided {
		resp, err := http.Get("http://myexternalip.com/raw")
		// cant get the public ip then ask the user for a reachable one
		if err != nil {
			log.Error("Could not get your public IP address")
			failedPublic = true
		} else {
			buff, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Error("Could not parse your public IP address", err)
				failedPublic = true
			} else {
				publicAddress = network.NewTCPAddress(strings.TrimSpace(string(buff)) + ":" + portStr)
			}
		}
	} else {
		publicAddress = serverBinding
	}

	// Let's directly ask the user for a reachable address
	if failedPublic {
		publicAddress = askReachableAddress(portStr)
	} else {
		if publicAddress.Public() {
			// try  to connect to ipfound:portgiven
			tryIP := publicAddress
			log.Info("Check if the address", tryIP, "is reachable from Internet...")
			if err := tryConnect(tryIP, serverBinding); err != nil {
				log.Error("Could not connect to your public IP")
				publicAddress = askReachableAddress(portStr)
			} else {
				publicAddress = tryIP
				log.Info("Address", publicAddress, "is publicly available from Internet.")
			}
		}
	}

	if !publicAddress.Valid() {
		log.Fatal("Could not validate public ip address:", publicAddress)
	}

	// create the keys
	privStr, pubStr := createKeyPair()
	conf := &config.CothoritydConfig{
		Public:  pubStr,
		Private: privStr,
		Address: serverBinding,
	}

	var configDone bool
	var configFolder string
	var defaultFolder = path.Dir(GetDefaultConfigFile(binaryName))
	var configFile string
	var groupFile string

	for !configDone {
		// get name of config file and write to config file
		configFolder = config.Input(defaultFolder, "Please enter a folder for the configuration files")
		configFile = path.Join(configFolder, DefaultServerConfig)
		groupFile = path.Join(configFolder, DefaultGroupFile)

		// check if the directory exists
		if _, err := os.Stat(configFolder); os.IsNotExist(err) {
			log.Info("Creating inexistant directory configuration", configFolder)
			if err = os.MkdirAll(configFolder, 0744); err != nil {
				log.Fatalf("Could not create directory configuration %s %v", configFolder, err)
			}
		}

		if checkOverwrite(configFile) && checkOverwrite(groupFile) {
			break
		}
	}

	public, err := crypto.ReadPubHex(network.Suite, pubStr)
	if err != nil {
		log.Fatal("Impossible to parse public key:", err)
	}

	server := config.NewServerToml(network.Suite, public, publicAddress)
	group := config.NewGroupToml(server)

	saveFiles(conf, configFile, group, groupFile)
	log.Info("All configurations saved, ready to serve signatures now.")
}

// CheckConfig contacts all servers and verifies if it receives a valid
// signature from each.
// If the roster is empty it will return an error.
// If a server doesn't reply in time, it will return an error.
func CheckConfig(tomlFileName string) error {
	f, err := os.Open(tomlFileName)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := config.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			tomlFileName)
	}
	log.Info("Checking the availability and responsiveness of the servers in the group...")
	return CheckServers(group)
}

// CheckServers contacts all servers in the entity-list and then makes checks
// on each pair. If server-descriptions are available, it will print them
// along with the IP-address of the server.
// In case a server doesn't reply in time or there is an error in the
// signature, an error is returned.
func CheckServers(g *config.Group) error {
	success := true
	// First check all servers individually
	for _, e := range g.Roster.List {
		desc := []string{"none", "none"}
		if d := g.GetDescription(e); d != "" {
			desc = []string{d, d}
		}
		el := sda.NewRoster([]*network.ServerIdentity{e})
		success = success && checkList(el, desc) == nil
	}
	if len(g.Roster.List) > 1 {
		// Then check pairs of servers
		for i, first := range g.Roster.List {
			for _, second := range g.Roster.List[i+1:] {
				desc := []string{"none", "none"}
				if d1 := g.GetDescription(first); d1 != "" {
					desc = []string{d1, g.GetDescription(second)}
				}
				es := []*network.ServerIdentity{first, second}
				success = success && checkList(sda.NewRoster(es), desc) == nil
				es[0], es[1] = es[1], es[0]
				desc[0], desc[1] = desc[1], desc[0]
				success = success && checkList(sda.NewRoster(es), desc) == nil
			}
		}
	}

	if !success {
		return errors.New("At least one of the tests failed")
	}
	return nil
}

// checkList sends a message to the cothority defined by list and
// waits for the reply.
// If the reply doesn't arrive in time, it will return an
// error.
func checkList(list *sda.Roster, descs []string) error {
	serverStr := ""
	for i, s := range list.List {
		name := strings.Split(descs[i], " ")[0]
		serverStr += fmt.Sprintf("%s_%s ", s.Address, name)
	}
	log.Lvl3("Sending message to: " + serverStr)
	msg := "verification"
	log.Info("Checking server(s) ", serverStr, ": ")
	sig, err := signStatement(strings.NewReader(msg), list)
	if err != nil {
		log.Error(err)
		return err
	}
	err = verifySignatureHash([]byte(msg), sig, list)
	if err != nil {
		log.Errorf("Invalid signature: %v", err)
		return err
	}
	log.Info("Success")
	return nil
}

// signStatement signs the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings). It uses
// the roster el to create the collective signature.
// In case the signature fails, an error is returned.
func signStatement(read io.Reader, el *sda.Roster) (*s.SignatureResponse,
	error) {
	//publics := entityListToPublics(el)
	client := s.NewClient()
	msg, _ := crypto.HashStream(network.Suite.Hash(), read)

	pchan := make(chan *s.SignatureResponse)
	var err error
	go func() {
		log.Lvl3("Waiting for the response on SignRequest")
		response, e := client.SignMsg(el, msg)
		if e != nil {
			err = e
			close(pchan)
			return
		}
		pchan <- response
	}()

	select {
	case response, ok := <-pchan:
		log.Lvl5("Response:", response)
		if !ok || err != nil {
			return nil, errors.New("Received an invalid repsonse.")
		}
		err = cosi.VerifySignature(network.Suite, el.Publics(), msg, response.Signature)
		if err != nil {
			return nil, err
		}
		return response, nil
	case <-time.After(RequestTimeOut):
		return nil, errors.New("timeout on signing request")
	}
}

// verifySignatureHash verifies if the message b is correctly signed by signature
// sig from roster el.
// If the signature-check fails for any reason, an error is returned.
func verifySignatureHash(b []byte, sig *s.SignatureResponse, el *sda.Roster) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	//publics := entityListToPublics(el)
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belonging to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	err := cosi.VerifySignature(network.Suite, el.Publics(), fHash, sig.Signature)
	if err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}

// entityListToPublics returns a slice of Points of all elements
// of the roster.
func entityListToPublics(el *sda.Roster) []abstract.Point {
	publics := make([]abstract.Point, len(el.List))
	for i, e := range el.List {
		publics[i] = e.Public
	}
	return publics
}

// Returns true if file exists and user confirms overwriting, or if file doesn't exist.
// Returns false if file exists and user doesn't confirm overwriting.
func checkOverwrite(file string) bool {
	// check if the file exists and ask for override
	if _, err := os.Stat(file); err == nil {
		return config.InputYN(true, "Configuration file "+file+" already exists. Override?")
	}
	return true
}

// createKeyPair returns the private and public key in hexadecimal representation.
func createKeyPair() (string, string) {
	log.Info("Creating ed25519 private and public keys.")
	kp := crypconf.NewKeyPair(network.Suite)
	privStr, err := crypto.ScalarHex(network.Suite, kp.Secret)
	if err != nil {
		log.Fatal("Error formating private key to hexadecimal. Abort.")
	}
	var point abstract.Point
	// use the transformation for EdDSA signatures
	//point = cosi.Ed25519Public(network.Suite, kp.Secret)
	point = kp.Public
	pubStr, err := crypto.PubHex(network.Suite, point)
	if err != nil {
		log.Fatal("Could not parse public key. Abort.")
	}

	log.Info("Public key: ", pubStr, "\n")
	return privStr, pubStr
}

// saveFiles takes a CothoritydConfig and its filename, and a GroupToml and its filename,
// and saves the data to these files.
// In case of a failure it Fatals.
func saveFiles(conf *config.CothoritydConfig, fileConf string, group *config.GroupToml, fileGroup string) {
	if err := conf.Save(fileConf); err != nil {
		log.Fatal("Unable to write the config to file:", err)
	}
	log.Info("Sucess! You can now use the CoSi server with the config file", fileConf)
	// group definition part
	if err := group.Save(fileGroup); err != nil {
		log.Fatal("Could not write your group file snippet:", err)
	}

	log.Info("Saved a group definition snippet for your server at", fileGroup,
		group.String())

}

// GetDefaultConfigFile returns the default path to the configuration-path, which
// is ~/.config/binaryName for Unix and ~/Library/binaryName for MacOSX.
// In case of an error it Fatals.
func GetDefaultConfigFile(binaryName string) string {
	u, err := user.Current()
	// can't get the user dir, so fallback to current working dir
	if err != nil {
		log.Error("Could not get your home-directory (", err.Error(), "). Switching back to current dir.")
		if curr, err := os.Getwd(); err != nil {
			log.Fatal("Impossible to get the current directory:", err)
		} else {
			return path.Join(curr, DefaultServerConfig)
		}
	}
	// Fetch standard folders.
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", binaryName, DefaultServerConfig)
	default:
		return path.Join(u.HomeDir, ".config", binaryName, DefaultServerConfig)
		// TODO Windows? FreeBSD?
	}
}

// askReachableAddress uses stdin to get the contactable IP-address of the server
// and adding port if necessary.
// In case of an error, it will Fatal.
func askReachableAddress(port string) network.Address {
	ipStr := config.Input(DefaultAddress, "IP-address where your server can be reached")

	splitted := strings.Split(ipStr, ":")
	if len(splitted) == 2 && splitted[1] != port {
		// if the client gave a port number, it must be the same
		log.Fatal("The port you gave is not the same as the one your server will be listening. Abort.")
	} else if len(splitted) == 2 && net.ParseIP(splitted[0]) == nil {
		// of if the IP address is wrong
		log.Fatal("Invalid IP:port address given:", ipStr)
	} else if len(splitted) == 1 {
		// check if the ip is valid
		if net.ParseIP(ipStr) == nil {
			log.Fatal("Invalid IP address given:", ipStr)
		}
		// add the port
		ipStr = ipStr + ":" + port
	}
	return network.NewTCPAddress(ipStr)
}

// tryConnect binds to the given IP address and ask an internet service to
// connect to it. binding is the address where we must listen (needed because
// the reachable address might not be the same as the binding address => NAT, ip
// rules etc).
// In case anything goes wrong, an error is returned.
func tryConnect(ip, binding network.Address) error {

	stopCh := make(chan bool, 1)
	// let's bind
	go func() {
		ln, err := net.Listen("tcp", binding.NetworkAddress())
		if err != nil {
			log.Error("Trouble with binding to the address:", err)
			return
		}
		con, _ := ln.Accept()
		<-stopCh
		con.Close()
	}()
	defer func() { stopCh <- true }()

	_, port, err := net.SplitHostPort(ip.NetworkAddress())
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
		return errors.New("Address unreachable")
	}
	return nil
}

// RunServer starts a cothority server with the given config file name. It can
// be used by different apps (like CoSi, for example)
func RunServer(configFilename string) {
	if _, err := os.Stat(configFilename); os.IsNotExist(err) {
		log.Fatalf("[-] Configuration file does not exists. %s", configFilename)
	}
	// Let's read the config
	_, conode, err := config.ParseCothorityd(configFilename)
	if err != nil {
		log.Fatal("Couldn't parse config:", err)
	}
	conode.Start()
}
