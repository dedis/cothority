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
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"os"
)

// ServerConfig represents one server that communicates with other servers
// and clients
type ServerConfig struct {
	// DirSSHD holds the directory for the SSHD-files
	DirSSHD string
	// DirSSH for the ssh directory
	DirSSH string
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
	sc := &ServerConfig{}
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		fmt.Print("Please enter an IP:port where this server has to be reached [localhost:2000] ")
	}
	return sc, nil
}

func CreateServerConfig(ip string) *ServerConfig {
	if ip == "" {
		ip = "localhost:2000"
	}
	pair := config.NewKeyPair(network.Suite)
	return &ServerConfig{
		Entity:  network.NewEntity(pair.Public, ip),
		Private: pair.Secret,
		DirSSHD: "/etc/sshd",
		DirSSH:  "/root/.ssh",
	}
}

func (sc *ServerConfig) ReadSSH() error {

	return nil
}

func (sc *ServerConfig) Start() error {
	return nil
}

func (sc *ServerConfig) Stop() error {
	return nil
}

func (sc *ServerConfig) WriteConfig(file string) error {
	return nil
}
