// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"os"
)

func init() {
	network.RegisterMessageType(ssh_ks.Config{})
}

var clientApp *ssh_ks.ClientApp

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
		{
			Name:    "server",
			Aliases: []string{"s"},
			Usage:   "handle servers",
			Subcommands: []cli.Command{
				{
					Name:   "add",
					Usage:  "add a server",
					Action: serverAdd,
				}, {
					Name:   "del",
					Usage:  "deletes a server",
					Action: serverDel,
				}, {
					Name:   "check",
					Usage:  "check servers",
					Action: serverCheck,
				},
			},
		},
		{
			Name:    "client",
			Aliases: []string{"c"},
			Usage:   "handle clients",
			Subcommands: []cli.Command{
				{
					Name:   "add",
					Usage:  "add a client",
					Action: clientAdd,
				}, {
					Name:   "del",
					Usage:  "delete a client",
					Action: clientDel,
				},
			},
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "update to the latest list",
			Action:  update,
		},
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "~/.ssh-ks",
			Usage: "The configuration-directory of ssh-keystore",
		},
		cli.StringFlag{
			Name:  "config-ssh, cs",
			Value: "~/.ssh",
			Usage: "The configuration-directory of the ssh-directory",
		},
	}
	app.Before = func(c *cli.Context) error {
		dbg.SetDebugVisible(c.Int("debug"))
		var err error
		clientApp, err = ssh_ks.ReadClientApp(c.String("config") + "/config.bin")
		dbg.ErrFatal(err, "Couldn't read config-file")
		return nil
	}
	app.After = func(c *cli.Context) error {
		clientApp.Write(c.String("config") + "/config.bin")
		return nil
	}
	app.Run(os.Args)
}

func serverAdd(c *cli.Context) {
	srvAddr := c.Args().First()
	dbg.Print("Contacting server", srvAddr)
	ServerAdd(clientApp, srvAddr)
}

func ServerAdd(ca *ssh_ks.ClientApp, srvAddr string) {
	srv, err := ssh_ks.NetworkGetServer(srvAddr)
	dbg.ErrFatal(err)
	err = ca.NetworkAddServer(srv)
	dbg.ErrFatal(err)
	conf, err := ca.NetworkSign(srv)
	dbg.ErrFatal(err)
	ca.Config = conf
}
func serverDel(c *cli.Context) {
	srvAddr := c.Args().First()
	dbg.Print("Deleting server", srvAddr)
	err := ServerDel(clientApp, srvAddr)
	dbg.ErrFatal(err)
	if len(clientApp.Config.Servers) == 0 {
		dbg.Print("Deleted last server")
	}
}

func ServerDel(ca *ssh_ks.ClientApp, srvAddr string) error {
	srv, err := ssh_ks.NetworkGetServer(srvAddr)
	dbg.ErrFatal(err)
	err = ca.NetworkDelServer(srv)
	dbg.ErrFatal(err)
	if len(ca.Config.Servers) == 1 {
		dbg.Lvl2("Deleted last server")
		ca.Config = ssh_ks.NewConfig(0)
	} else {
		for _, s := range ca.Config.Servers {
			dbg.Lvl3(s.Entity.Addresses[0], srv.Entity.Addresses[0])
			if s.Entity.Addresses[0] != srv.Entity.Addresses[0] {
				conf, err := ca.NetworkSign(s)
				if err != nil {
					return err
				}
				dbg.Lvl3("New config:", conf)
				ca.Config = conf
			}
		}
	}
	dbg.Lvl3(ca.Config.Servers)
	return nil
}

func serverCheck(c *cli.Context) {
	err := ServerCheck(clientApp)
	if err != nil {
		dbg.Error(err)
	} else {
		dbg.Print("Correctly checkd servers")
	}
}

func ServerCheck(ca *ssh_ks.ClientApp) error {
	if ca.Config == nil {
		return errors.New("No config defined")
	}
	if len(ca.Config.Servers) == 0 {
		return errors.New("No servers defined")
	}
	var cnf *ssh_ks.Config
	for _, srv := range ca.Config.Servers {
		cnf_, err := ca.NetworkGetConfig(srv)
		dbg.ErrFatal(err)
		if cnf != nil {
			if bytes.Compare(cnf.Hash(), cnf_.Hash()) != 0 {
				return errors.New("Hashes should be the same\n" +
					"1st server has " + fmt.Sprintln(cnf_) +
					"Server" + fmt.Sprint(srv) + "has" + fmt.Sprintln(cnf_))
			} else {
				cnf = cnf_
			}
		}
	}
	return nil
}

func clientAdd(c *cli.Context) {}
func clientDel(c *cli.Context) {}
func update(c *cli.Context)    {}
