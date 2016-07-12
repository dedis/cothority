package main

import (
	"os"

	"encoding/hex"

	"path"

	"io/ioutil"
	"strings"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/services/identity"
	"gopkg.in/codegangsta/cli.v1"
)

/*
Cisc is the Cisc Identity SkipChain to store information in a skipchain and
being able to retrieve it.

This is only one part of the system - the other part being the cothority that
holds the skipchain and answers to requests from the cisc-binary.
*/

var configFile string
var sshDir string
var sshConfig string
var clientApp *identity.Identity

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
		{
			Name:  "id",
			Usage: "working on the identity",
			Subcommands: []cli.Command{
				{
					Name:      "create",
					Aliases:   []string{"cr"},
					Usage:     "start a new identity",
					ArgsUsage: "group [id-name]",
					Action:    idCreate,
				},
				{
					Name:      "connect",
					Aliases:   []string{"co"},
					Usage:     "connect to an existing identity",
					ArgsUsage: "group id [id-name]",
					Action:    idConnect,
				},
				{
					Name:    "remove",
					Aliases: []string{"rm"},
					Usage:   "remove an identity",
					Action:  idRemove,
				},
				{
					Name:    "follow",
					Aliases: []string{"f"},
					Usage:   "follow an existing identity",
					Action:  idFollow,
				},
				{
					Name:    "check",
					Aliases: []string{"ch"},
					Usage:   "check the health of the cothority",
					Action:  idCheck,
				},
			},
		},
		{
			Name:  "config",
			Usage: "updating and voting on config",
			Subcommands: []cli.Command{
				{
					Name:    "propose",
					Aliases: []string{"l"},
					Usage:   "propose the new config",
					Action:  configPropose,
				},
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "fetch the latest config",
					Action:  configUpdate,
				},
				{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "list existing config and proposed",
					Action:  configList,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "p,propose",
							Usage: "will also show proposed config",
						},
					},
				},
				{
					Name:    "vote",
					Aliases: []string{"v"},
					Usage:   "vote on existing config",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "r,reject",
							Usage: "reject the proposition",
						},
					},
					Action: configVote,
				},
			},
		},
		{
			Name:    "keyvalue",
			Aliases: []string{"kv"},
			Usage:   "storing and retrieving key/value pairs",
			Subcommands: []cli.Command{
				{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "list all values",
					Action:  kvList,
				},
				{
					Name:      "value",
					Aliases:   []string{"v"},
					Usage:     "return the value of a key",
					ArgsUsage: "key",
					Action:    kvValue,
				},
				{
					Name:      "add",
					Aliases:   []string{"a"},
					Usage:     "add a new key/value pair",
					ArgsUsage: "key value",
					Action:    kvAdd,
				},
				{
					Name:      "rm",
					Aliases:   []string{"ls"},
					Usage:     "list all values",
					ArgsUsage: "key",
					Action:    kvRm,
				},
			},
		},
		{
			Name:  "ssh",
			Usage: "handling your ssh-keys",
			Subcommands: []cli.Command{
				{
					Name:    "add",
					Aliases: []string{"a"},
					Usage:   "adds a new entry to the config",
					Action:  sshAdd,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "a,alias",
							Usage: "alias to use for that entry",
						},
						cli.StringFlag{
							Name:  "u,user",
							Usage: "user for that connection",
						},
						cli.StringFlag{
							Name:  "p,port",
							Usage: "port for the connection",
						},
					},
				},
				{
					Name:      "del",
					Aliases:   []string{"rm"},
					Usage:     "deletes an entry from the config",
					ArgsUsage: "alias_or_host",
					Action:    sshDel,
				},
				{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "shows all entries for this device",
					Action:  sshLs,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "a,all",
							Usage: "show entries for all devices",
						},
					},
				},
				//{
				//	Name:    "rotate",
				//	Aliases: []string{"r"},
				//	Usage:   "renews all keys - only active once the vote passed",
				//	Action:  sshRotate,
				//},
				//{
				//	Name:    "sync",
				//	Aliases: []string{"tc"},
				//	Usage:   "sync config and blockchain - interactive",
				//	Flags: []cli.Flag{
				//		cli.StringFlag{
				//			Name:  "tob,toblockchain",
				//			Usage: "force copy of config-file to blockchain",
				//		},
				//		cli.StringFlag{
				//			Name:  "toc,toconfig",
				//			Usage: "force copy of blockchain to config-file",
				//		},
				//	},
				//	Action: sshSync,
				//},
			},
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
			Value: "~/.cisc",
			Usage: "The configuration-directory of cisc",
		},
		cli.StringFlag{
			Name:  "config-ssh, cs",
			Value: "~/.ssh",
			Usage: "The configuration-directory of the ssh-directory",
		},
	}
	app.Before = func(c *cli.Context) error {
		configDir := config.TildeToHome(c.String("config"))
		os.Mkdir(configDir, 0660)
		log.SetDebugVisible(c.Int("debug"))
		configFile = configDir + "/config.bin"
		if err := loadConfig(); err != nil {
			log.Error("Problems reading config-file. Most probably you\n",
				"should start a new one by running with the 'setup'\n",
				"argument.")
		}
		sshDir = config.TildeToHome(c.String("cs"))
		sshConfig = sshDir + "/config"
		return nil
	}
	app.After = func(c *cli.Context) error {
		if clientApp != nil {
			err := saveConfig()
			log.ErrFatal(err, "Error while creating config-file", configFile)
		}
		return nil
	}
	app.Run(os.Args)

}

/*
 * Identity-related commands
 */
func idCreate(c *cli.Context) error {
	log.Info("Creating id")
	if c.NArg() == 0 {
		log.Fatal("Please give at least a group-definition")
	}

	group := getGroup(c)

	name, err := os.Hostname()
	log.ErrFatal(err)
	if c.NArg() > 1 {
		name = c.Args().Get(1)
	}
	log.Info("Creating new blockchain-identity for", name)

	clientApp = identity.NewIdentity(group.Roster, 2, name)
	log.ErrFatal(clientApp.CreateIdentity())
	err = saveConfig()
	log.ErrFatal(err)
	return nil
}

func idConnect(c *cli.Context) error {
	log.Info("Connecting")
	name, err := os.Hostname()
	log.ErrFatal(err)
	switch c.NArg() {
	case 2:
		// We'll get all arguments after
	case 3:
		name = c.Args().Get(2)
	default:
		log.Fatal("Please give the following arguments: group.toml id [hostname]", c.NArg())
	}
	group := getGroup(c)
	idBytes, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	id := identity.ID(idBytes)
	clientApp = identity.NewIdentity(group.Roster, 2, name)
	clientApp.AttachToIdentity(id)
	log.ErrFatal(saveConfig())
	return nil
}
func idRemove(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func idFollow(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func idCheck(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}

/*
 * Commands related to the config in general
 */
func configUpdate(c *cli.Context) error {
	assertCA()
	log.ErrFatal(clientApp.ConfigUpdate())
	log.ErrFatal(clientApp.ProposeFetch())
	log.Info("Successfully updated")
	return configList(c)
}
func configList(c *cli.Context) error {
	assertCA()
	log.Info("Account name:", clientApp.ManagerStr)
	log.Infof("Identity-ID: %x", clientApp.ID)
	log.Infof("Current config: %s", clientApp.Config)
	if c.Bool("p") {
		if clientApp.Proposed != nil {
			log.Infof("Proposed config: %s", clientApp.Proposed)
		} else {
			log.Info("No proposed config")
		}
	}
	return nil
}
func configPropose(c *cli.Context) error {
	assertCA()
	log.Fatal("Not yet implemented")
	return nil
}
func configVote(c *cli.Context) error {
	assertCA()
	log.ErrFatal(clientApp.ProposeVote(!c.Bool("r")))
	return nil
}

/*
 * Commands related to the key/value storage and retrieval
 */
func kvList(c *cli.Context) error {
	assertCA()
	log.Infof("config for id %x", clientApp.ID)
	for k, v := range clientApp.Config.Data {
		log.Infof("%s: %s", k, v)
	}
	return nil
}
func kvValue(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func kvAdd(c *cli.Context) error {
	assertCA()
	if c.NArg() < 2 {
		log.Fatal("Please give a key value pair")
	}
	key := c.Args().Get(0)
	value := c.Args().Get(1)
	prop := clientApp.GetProposed()
	prop.Data[key] = value
	log.ErrFatal(clientApp.ProposeSend(prop))
	return nil
}
func kvRm(c *cli.Context) error {
	assertCA()
	if c.NArg() != 1 {
		log.Fatal("Please give a key to delete")
	}
	key := c.Args().First()
	prop := clientApp.GetProposed()
	if _, ok := prop.Data[key]; !ok {
		log.Fatal("Didn't find key", key, "in the config")
	}
	delete(prop.Data, key)
	log.ErrFatal(clientApp.ProposeSend(prop))
	return nil
}

/*
 * Commands related to the ssh-handling
 */
func sshAdd(c *cli.Context) error {
	assertCA()
	if c.NArg() != 1 {
		log.Fatal("Please give the hostname as argument")
	}

	// Get the current configuration
	sc, err := NewSSHConfigFromFile(sshConfig)
	log.ErrFatal(err)

	// Add a new host-entry
	hostname := c.Args().First()
	alias := c.String("a")
	if alias == "" {
		alias = hostname
	}
	filePub := path.Join(sshDir, "key_"+alias+".pub")
	idPriv := "key_" + alias
	filePriv := path.Join(sshDir, idPriv)
	log.ErrFatal(makeSSHKeyPair(filePub, filePriv))
	host := NewSSHHost(alias, "HostName"+hostname,
		"IdentityFile "+idPriv)
	if port := c.String("p"); port != "" {
		host.AddConfig("Port " + port)
	}
	if user := c.String("u"); user != "" {
		host.AddConfig("User " + user)
	}
	sc.AddHost(host)
	err = ioutil.WriteFile(sshConfig, []byte(sc.String()), 0600)
	log.ErrFatal(err)

	// Propose the new configuration
	prop := clientApp.GetProposed()
	key := strings.Join([]string{"ssh", clientApp.ManagerStr, alias}, ":")
	pub, err := ioutil.ReadFile(filePub)
	log.ErrFatal(err)
	prop.Data[key] = string(pub)
	proposeSendVoteUpdate(prop)
	return nil
}
func sshLs(c *cli.Context) error {
	assertCA()
	var devs []string
	if c.Bool("a") {
		devs = kvGetKeys("ssh")
	} else {
		devs = []string{clientApp.ManagerStr}
	}
	for _, dev := range devs {
		for _, pub := range kvGetKeys("ssh", dev) {
			log.Printf("SSH-key for device %s: %s", dev, pub)
		}
	}
	return nil
}
func sshDel(c *cli.Context) error {
	assertCA()
	if c.NArg() == 0 {
		log.Fatal("Please give alias or host to delete from ssh")
	}
	ah := c.Args().First()
	if len(kvGetValue("ssh", clientApp.ManagerStr, ah)) == 0 {
		log.Print("Didn't find alias or host", ah)
		sshLs(c)
		log.Fatal("Aboring")
	}
	sc, err := NewSSHConfigFromFile(sshConfig)
	log.ErrFatal(err)
	sc.DelHost(ah)
	err = ioutil.WriteFile(sshConfig, []byte(sc.String()), 0600)
	log.ErrFatal(err)
	prop := clientApp.GetProposed()
	delete(prop.Data, "ssh:"+clientApp.ManagerStr+":"+ah)
	proposeSendVoteUpdate(prop)
	return nil
}
func sshRotate(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func sshSync(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func proposeSendVoteUpdate(p *identity.Config) {
	log.ErrFatal(clientApp.ProposeSend(p))
	log.ErrFatal(clientApp.ProposeVote(true))
	log.ErrFatal(clientApp.ConfigUpdate())
}
