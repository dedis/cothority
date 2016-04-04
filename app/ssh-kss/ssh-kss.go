// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import ()
import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"github.com/dedis/crypto/config"
	"io/ioutil"
	"os"
)

func init() {
	network.RegisterMessageType(ServerConfig{})
}

// ServerConfig represents one server that communicates with other servers
// and clients
type ServerConfig struct {
	// DirSSHD holds the directory for the SSHD-files
	DirSSHD string
	// DirSSH for the ssh directory
	DirSSH string
	// Server holds a list of servers and clients
	Server *ssh_ks.Server
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
		config, err := ReadServerConfig(c.String("config"))
		if err != nil {
			dbg.Fatal("Couldn't get config:", err)
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
		fmt.Print("Please enter an IP:port where this server has to be reached [localhost:2000] ")
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

func CreateServerConfig(ip string) *ServerConfig {
	if ip == "" {
		ip = "localhost:2000"
	}
	pair := config.NewKeyPair(network.Suite)
	return &ServerConfig{
		Server:  ssh_ks.NewServer(pair, ip),
		DirSSHD: "/etc/sshd",
		DirSSH:  "/root/.ssh",
	}
}

func (sc *ServerConfig) ReadSSH() error {

	return nil
}

func (sc *ServerConfig) Start() error {
	return sc.Server.Start()
}

func (sc *ServerConfig) Stop() error {
	return sc.Server.Stop()
}

func (sc *ServerConfig) WriteConfig(file string) error {
	b, err := network.MarshalRegisteredType(sc)
	if err != nil {
		return err
	}
	ioutil.WriteFile(file, b, 0660)
	return nil
}
