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

	"bytes"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/services/identity"
	"gopkg.in/codegangsta/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Version = "0.3"
	app.Commands = []cli.Command{
		commandID,
		commandConfig,
		commandKeyvalue,
		commandSSH,
		commandFollow,
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
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

	thr := c.Int("threshold")
	cfg := &ciscConfig{Identity: identity.NewIdentity(group.Roster, thr, name)}
	log.ErrFatal(cfg.CreateIdentity())
	log.Infof("IC is %x", cfg.ID)
	log.Infof("Config to be saved: %+v", cfg.Identity.Cothority)
	return cfg.saveConfig(c)
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
	cfg := &ciscConfig{Identity: identity.NewIdentity(group.Roster, 0, name)}
	cfg.AttachToIdentity(id)
	log.Infof("Public key: %s",
		cfg.Proposed.Device[cfg.DeviceName].Point.String())
	return cfg.saveConfig(c)
}
func idDel(c *cli.Context) error {
	if c.NArg() == 0 {
		log.Fatal("Please give device to delete")
	}
	cfg := loadConfigOrFail(c)
	dev := c.Args().First()
	if _, ok := cfg.Config.Device[dev]; !ok {
		log.Error("Didn't find", dev, "in config. Available devices:")
		configList(c)
		log.Fatal("Device not found in config.")
	}
	prop := cfg.GetProposed()
	delete(prop.Device, dev)
	for _, s := range cfg.Config.GetSuffixColumn("ssh", dev) {
		delete(prop.Data, "ssh:"+dev+":"+s)
	}
	cfg.proposeSendVoteUpdate(prop)
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
	cfg := loadConfigOrFail(c)
	log.ErrFatal(cfg.ConfigUpdate())
	log.ErrFatal(cfg.ProposeUpdate())
	log.Info("Successfully updated")
	log.ErrFatal(cfg.saveConfig(c))
	if cfg.Proposed != nil {
		cfg.showDifference()
	} else {
		cfg.showKeys()
	}
	return nil
}
func configList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	log.Info("Account name:", cfg.DeviceName)
	log.Infof("Identity-ID: %x", cfg.ID)
	if c.Bool("d") {
		log.Info(cfg.Config.Data)
	} else {
		cfg.showKeys()
	}
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
	cfg := loadConfigOrFail(c)
	log.ErrFatal(cfg.ConfigUpdate())
	log.ErrFatal(cfg.ProposeUpdate())
	if cfg.Proposed == nil {
		log.Info("No proposed config")
		return nil
	}
	if c.NArg() == 0 {
		cfg.showDifference()
		if !config.InputYN(true, "Do you want to accept the changes") {
			return nil
		}
	}
	if strings.ToLower(c.Args().First()) == "n" {
		return nil
	}
	log.ErrFatal(cfg.ProposeVote(true))
	return cfg.saveConfig(c)
}

/*
 * Commands related to the key/value storage and retrieval
 */
func kvList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
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
	cfg := loadConfigOrFail(c)
	if c.NArg() < 2 {
		log.Fatal("Please give a key value pair")
	}
	key := c.Args().Get(0)
	value := c.Args().Get(1)
	prop := cfg.GetProposed()
	prop.Data[key] = value
	log.ErrFatal(cfg.ProposeSend(prop))
	return cfg.saveConfig(c)
}
func kvDel(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
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
	return cfg.saveConfig(c)
}

/*
 * Commands related to the ssh-handling. All ssh-keys are stored in the
 * identity-sc as
 * ssh:device:server / ssh_public_key
 * where 'ssh' is a fixed string, 'device' is the device where the private
 * key is stored and 'server' the server that should add the public key to
 * its authorized_keys.
 */
func sshAdd(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
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
		"IdentityFile "+filePriv)
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
	key := strings.Join([]string{"ssh", cfg.DeviceName, hostname}, ":")
	pub, err := ioutil.ReadFile(filePub)
	log.ErrFatal(err)
	prop.Data[key] = strings.TrimSpace(string(pub))
	cfg.proposeSendVoteUpdate(prop)
	return cfg.saveConfig(c)
}
func sshLs(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	var devs []string
	if c.Bool("a") {
		devs = cfg.Config.GetSuffixColumn("ssh")
	} else {
		devs = []string{cfg.DeviceName}
	}
	for _, dev := range devs {
		for _, pub := range cfg.Config.GetSuffixColumn("ssh", dev) {
			log.Printf("SSH-key for device %s: %s", dev, pub)
		}
	}
	return nil
}
func sshDel(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	_, sshConfig := sshDirConfig(c)
	if c.NArg() == 0 {
		log.Fatal("Please give alias or host to delete from ssh")
	}
	sc, err := NewSSHConfigFromFile(sshConfig)
	log.ErrFatal(err)
	// Converting ah to a hostname if found in ssh-config
	host := sc.ConvertAliasToHostname(c.Args().First())
	if len(cfg.Config.GetValue("ssh", cfg.DeviceName, host)) == 0 {
		log.Error("Didn't find alias or host", host, "here is what I know:")
		sshLs(c)
		log.Fatal("Unknown alias or host.")
	}

	sc.DelHost(host)
	err = ioutil.WriteFile(sshConfig, []byte(sc.String()), 0600)
	log.ErrFatal(err)
	prop := cfg.GetProposed()
	delete(prop.Data, "ssh:"+cfg.DeviceName+":"+host)
	cfg.proposeSendVoteUpdate(prop)
	return cfg.saveConfig(c)
}
func sshRotate(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}
func sshSync(c *cli.Context) error {
	log.Fatal("Not yet implemented")
	return nil
}

func followAdd(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give a group-definition, an ID, and optionally a service-name of the skipchain to follow")
	}
	cfg, _ := loadConfig(c)
	group := getGroup(c)
	idBytes, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	id := identity.ID(idBytes)
	newID, err := identity.NewIdentityFromCothority(group.Roster, id)
	log.ErrFatal(err)
	if c.NArg() == 3 {
		newID.DeviceName = c.Args().Get(2)
	} else {
		var err error
		newID.DeviceName, err = os.Hostname()
		log.ErrFatal(err)
		log.Info("Using", newID.DeviceName, "as the device-name.")
	}
	cfg.Follow = append(cfg.Follow, newID)
	cfg.writeAuthorizedKeys(c)
	// Identity needs to exist, else saving/loading will fail. For
	// followers it doesn't matter if the identity will be overwritten,
	// as it is not used.
	cfg.Identity = newID
	return cfg.saveConfig(c)
}
func followDel(c *cli.Context) error {
	if c.NArg() != 1 {
		log.Fatal("Please give id of skipchain to unfollow")
	}
	cfg := loadConfigOrFail(c)
	idBytes, err := hex.DecodeString(c.Args().First())
	log.ErrFatal(err)
	idDel := identity.ID(idBytes)
	newSlice := cfg.Follow[:0]
	for _, id := range cfg.Follow {
		if !bytes.Equal(id.ID, idDel) {
			newSlice = append(newSlice, id)
		}
	}
	cfg.Follow = newSlice
	cfg.writeAuthorizedKeys(c)
	return cfg.saveConfig(c)
}
func followList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	for _, id := range cfg.Follow {
		log.Infof("SCID: %x", id.ID)
		server := id.DeviceName
		log.Infof("Server %s is asked to accept ssh-keys from %s:",
			server,
			id.Config.GetIntermediateColumn("ssh", server))
	}
	return nil
}
func followUpdate(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	for _, f := range cfg.Follow {
		log.ErrFatal(f.ConfigUpdate())
	}
	cfg.writeAuthorizedKeys(c)
	return cfg.saveConfig(c)
}
