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
	"strings"

	c "github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"gopkg.in/urfave/cli.v1"
	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cosi/protocol"
	_ "github.com/dedis/cosi/service"
)

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	if _, err := os.Stat(config); os.IsNotExist(err) {
		log.Fatalf("[-] Configuration file does not exist. %s. "+
			"Use `cosi server setup` to create one.", config)
	}
	// Let's read the config
	_, conode, err := c.ParseCothorityd(config)
	if err != nil {
		log.Fatal("Couldn't parse config:", err)
	}
	conode.Start()
}

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf(format, a...)+"\n")
}

func stderrExit(format string, a ...interface{}) {
	stderr(format, a...)
	os.Exit(1)
}

func getDefaultConfigFile() string {
	u, err := user.Current()
	// can't get the user dir, so fallback to current working dir
	if err != nil {
		fmt.Print("[-] Could not get your home-directory (", err.Error(), "). Switching back to current dir.\n")
		if curr, err := os.Getwd(); err != nil {
			stderrExit("[-] Impossible to get the current directory. %v", err)
		} else {
			return path.Join(curr, DefaultServerConfig)
		}
	}
	// let's try to stick to usual OS folders
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", BinaryName, DefaultServerConfig)
	default:
		return path.Join(u.HomeDir, ".config", BinaryName, DefaultServerConfig)
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

func askReachableAddress(reader *bufio.Reader, port string) network.Address {
	fmt.Println("[*] Enter the IP address you would like others cothority servers and client to contact you.")
	fmt.Print("[*] Type <Enter> to use the default address [ " + DefaultAddress + " ] if you plan to do local experiments:")
	ipStr := readString(reader)
	if ipStr == "" {
		return network.NewTCPAddress(DefaultAddress + ":" + port)
	}

	splitted := strings.Split(ipStr, ":")
	if len(splitted) == 2 && splitted[1] != port {
		// if the client gave a port number, it must be the same
		stderrExit("[-] The port you gave is not the same as the one your server will be listening. Abort.")
	} else if len(splitted) == 2 && net.ParseIP(splitted[0]) == nil {
		// of if the IP address is wrong
		stderrExit("[-] Invalid IP:port address given (%s)", ipStr)
	} else if len(splitted) == 1 {
		// check if the ip is valid
		if net.ParseIP(ipStr) == nil {
			stderrExit("[-] Invalid IP address given (%s)", ipStr)
		}
		// add the port
		ipStr = ipStr + ":" + port
	}
	return network.NewTCPAddress(ipStr)
}

// Service used to get the port connection service
const whatsMyIP = "http://www.whatsmyip.org/"

// tryConnect will bind to the ip address and ask a internet service to try to
// connect to it. binding is the address where we must listen (needed because
// the reachable address might not be the same as the binding address => NAT, ip
// rules etc).
func tryConnect(ip, binding network.Address) error {

	stopCh := make(chan bool, 1)
	// let's bind
	go func() {
		ln, err := net.Listen("tcp", binding.NetworkAddress())
		if err != nil {
			fmt.Println("[-] Trouble with binding to the address:", err)
			return
		}
		con, _ := ln.Accept()
		<-stopCh
		con.Close()
	}()
	defer func() { stopCh <- true }()

	port := ip.Port()
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
