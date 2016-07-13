/*
Cisc is the Cisc Identity SkipChain to store information in a skipchain and
being able to retrieve it.

This is only one part of the system - the other part being the cothority that
holds the skipchain and answers to requests from the cisc-binary.
*/
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

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
		commandID,
		commandConfig,
		commandKeyvalue,
		commandSSH,
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
		log.SetDebugVisible(c.Int("debug"))
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

	cfg := identity.NewIdentity(group.Roster, 2, name)
	log.ErrFatal(cfg.CreateIdentity())
	return saveConfig(c, cfg)
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
	cfg := identity.NewIdentity(group.Roster, 2, name)
	cfg.AttachToIdentity(id)
	return saveConfig(c, cfg)
}
func idFollow(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func idRemove(c *cli.Context) error {
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
	cfg := loadConfig(c)
	log.ErrFatal(cfg.ConfigUpdate())
	log.ErrFatal(cfg.ProposeFetch())
	log.Info("Successfully updated")
	log.ErrFatal(saveConfig(c, cfg))
	return configList(c)
}
func configList(c *cli.Context) error {
	cfg := loadConfig(c)
	log.Info("Account name:", cfg.ManagerStr)
	log.Infof("Identity-ID: %x", cfg.ID)
	log.Infof("Current config: %s", cfg.Config)
	if c.Bool("p") {
		if cfg.Proposed != nil {
			log.Infof("Proposed config: %s", cfg.Proposed)
		} else {
			log.Info("No proposed config")
		}
	}
	return nil
}
func configPropose(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func configVote(c *cli.Context) error {
	cfg := loadConfig(c)
	if c.NArg() == 0 {
		configList(c)
		if !config.InputYN(true, "Do you want to accept the changes") {
			return nil
		}
	}
	if strings.ToLower(c.Args().First()) == "n" {
		return nil
	}
	log.ErrFatal(cfg.ProposeVote(true))
	return saveConfig(c, cfg)
}

/*
 * Commands related to the key/value storage and retrieval
 */
func kvList(c *cli.Context) error {
	cfg := loadConfig(c)
	log.Infof("config for id %x", cfg.ID)
	for k, v := range cfg.Config.Data {
		log.Infof("%s: %s", k, v)
	}
	return nil
}
func kvValue(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func kvAdd(c *cli.Context) error {
	cfg := loadConfig(c)
	if c.NArg() < 2 {
		log.Fatal("Please give a key value pair")
	}
	key := c.Args().Get(0)
	value := c.Args().Get(1)
	prop := cfg.GetProposed()
	prop.Data[key] = value
	log.ErrFatal(cfg.ProposeSend(prop))
	return saveConfig(c, cfg)
}
func kvDel(c *cli.Context) error {
	cfg := loadConfig(c)
	if c.NArg() != 1 {
		log.Fatal("Please give a key to delete")
	}
	key := c.Args().First()
	prop := cfg.GetProposed()
	if _, ok := prop.Data[key]; !ok {
		log.Fatal("Didn't find key", key, "in the config")
	}
	delete(prop.Data, key)
	log.ErrFatal(cfg.ProposeSend(prop))
	return saveConfig(c, cfg)
}

/*
 * Commands related to the ssh-handling
 */
func sshAdd(c *cli.Context) error {
	cfg := loadConfig(c)
	sshDir, sshConfig := sshDirConfig(c)
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
	log.ErrFatal(makeSSHKeyPair(c.Int("sec"), filePub, filePriv))
	host := NewSSHHost(alias, "HostName "+hostname,
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
	prop := cfg.GetProposed()
	key := strings.Join([]string{"ssh", cfg.ManagerStr, alias}, ":")
	pub, err := ioutil.ReadFile(filePub)
	log.ErrFatal(err)
	prop.Data[key] = string(pub)
	proposeSendVoteUpdate(cfg, prop)
	return saveConfig(c, cfg)
}
func sshLs(c *cli.Context) error {
	cfg := loadConfig(c)
	var devs []string
	if c.Bool("a") {
		devs = cfg.Config.GetKeys("ssh")
	} else {
		devs = []string{cfg.ManagerStr}
	}
	for _, dev := range devs {
		for _, pub := range cfg.Config.GetKeys("ssh", dev) {
			log.Printf("SSH-key for device %s: %s", dev, pub)
		}
	}
	return nil
}
func sshDel(c *cli.Context) error {
	cfg := loadConfig(c)
	_, sshConfig := sshDirConfig(c)
	if c.NArg() == 0 {
		log.Fatal("Please give alias or host to delete from ssh")
	}
	ah := c.Args().First()
	if len(cfg.Config.GetValue("ssh", cfg.ManagerStr, ah)) == 0 {
		log.Print("Didn't find alias or host", ah)
		sshLs(c)
		log.Fatal("Aborting")
	}
	sc, err := NewSSHConfigFromFile(sshConfig)
	log.ErrFatal(err)
	sc.DelHost(ah)
	err = ioutil.WriteFile(sshConfig, []byte(sc.String()), 0600)
	log.ErrFatal(err)
	prop := cfg.GetProposed()
	delete(prop.Data, "ssh:"+cfg.ManagerStr+":"+ah)
	proposeSendVoteUpdate(cfg, prop)
	return saveConfig(c, cfg)
}
func sshRotate(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func sshSync(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func proposeSendVoteUpdate(cfg *identity.Identity, p *identity.Config) {
	log.ErrFatal(cfg.ProposeSend(p))
	log.ErrFatal(cfg.ProposeVote(true))
	log.ErrFatal(cfg.ConfigUpdate())
}
