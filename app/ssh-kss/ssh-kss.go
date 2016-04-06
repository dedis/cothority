// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import ()
import (
	"bufio"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"github.com/dedis/crypto/config"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func init() {
	network.RegisterMessageType(ServerConfig{})
}

// ServerConfig represents one server that communicates with other servers
// and clients
type ServerConfig struct {
	// DirSSHD holds the directory for the SSHD-files
	DirSSHD string
	// DirSSH holds the ssh-directory for storing the authorized_keys
	DirSSH string
	// CoNode represents one conode plus the configuration for the whole
	// Cothority
	CoNode *ssh_ks.ServerApp
}

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore server"
	app.Usage = "Serves as a server to listen to requests"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "/etc/ssh-ks",
			Usage: "The configuration-file of ssh-keystore",
		},
	}
	app.Action = func(c *cli.Context) {
		dbg.SetDebugVisible(c.Int("debug"))
		file := c.String("config")
		config, err := ReadServerConfig(file)
		if err != nil {
			config, err = AskServerConfig(os.Stdin, os.Stdout)
			if err != nil {
				dbg.Fatal("While creating new config:", err)
			}
			err = config.WriteConfig(file)
			if err != nil {
				dbg.Fatal("Couldn't write config:", err)
			}
		}
		err = config.Start()
		if err != nil {
			dbg.Fatal("Couldn't start server:", err)
		}
	}
	app.Run(os.Args)
}

func ReadServerConfig(file string) (*ServerConfig, error) {
	if file == "" {
		return nil, errors.New("Need a name for the configuration-file")
	}
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return nil, errors.New("Didn't find file " + file)
	}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	conf := msg.(ServerConfig)
	return &conf, err
}

func AskServerConfig(in io.Reader, out io.Writer) (*ServerConfig, error) {
	inb := bufio.NewReader(in)
	ip := getArg(inb, out, "Please enter an IP:port where this server has to be reached",
		"localhost:2000")
	ssh := getArg(inb, out, "Where should the authorized_keys be stored",
		"/root/.ssh")
	sshd := getArg(inb, out, "Where is the system-ssh-directory located",
		"/etc/sshd")
	return CreateServerConfig(ip, ssh, sshd)
}

func CreateServerConfig(ip, dirSSH, dirSSHD string) (*ServerConfig, error) {
	if ip == "" {
		ip = "localhost:2000"
	}
	pair := config.NewKeyPair(network.Suite)
	sshPub, err := ioutil.ReadFile(dirSSHD + "/ssh_host_rsa_key.pub")
	if err != nil {
		return nil, errors.New("While reading public key: " + err.Error())
	}
	return &ServerConfig{
		CoNode:  ssh_ks.NewCoNode(pair, ip, string(sshPub)),
		DirSSHD: dirSSHD,
	}, nil
}

func (sc *ServerConfig) Start() error {
	return sc.CoNode.Start()
}

func (sc *ServerConfig) Stop() error {
	return sc.CoNode.Stop()
}

func (sc *ServerConfig) WriteConfig(file string) error {
	b, err := network.MarshalRegisteredType(sc)
	if err != nil {
		return err
	}
	ioutil.WriteFile(file, b, 0660)
	return nil
}

func getArg(in *bufio.Reader, out io.Writer, question, def string) string {
	fmt.Fprintf(out, "%s [%s]: ", question, def)
	b, _ := in.ReadString('\n')
	str := strings.TrimSpace(string(b))
	if str == "" {
		return def
	} else {
		return str
	}
}
