package main

import (
	"os"

	"golang.org/x/crypto/ssh"

	"io/ioutil"

	"errors"

	"encoding/hex"

	"fmt"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"path"

	"strings"

	"sort"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
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
			Name:  "data",
			Usage: "updating and voting on data",
			Subcommands: []cli.Command{
				{
					Name:    "propose",
					Aliases: []string{"l"},
					Usage:   "propose the new data",
					Action:  dataPropose,
				},
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "fetch the latest data",
					Action:  dataUpdate,
				},
				{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "list existing data and proposed",
					Action:  dataList,
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
					Usage:   "vote on existing data",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "r,reject",
							Usage: "reject the proposition",
						},
					},
					Action: dataVote,
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
					Name:    "del",
					Aliases: []string{"rm"},
					Usage:   "deletes an entry from the config",
					Action:  sshDel,
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

// loadConfig will return nil if the config-file doesn't exist. It tries to
// load the file given in configFile.
func loadConfig() error {
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	if err != nil {
		return err
	}
	var ok bool
	clientApp, ok = msg.(*identity.Identity)
	if !ok {
		return errors.New("Wrong message-type in config-file")
	}
	return nil
}

func saveConfig() error {
	buf, err := network.MarshalRegisteredType(clientApp)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, buf, 0660)
}

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

func dataUpdate(c *cli.Context) error {
	assertCA()
	log.ErrFatal(clientApp.ConfigUpdate())
	log.ErrFatal(clientApp.ProposeFetch())
	log.Info("Successfully updated")
	return dataList(c)
}
func dataList(c *cli.Context) error {
	assertCA()
	log.Info("Account name:", clientApp.ManagerStr)
	log.Infof("Identity-ID: %x", clientApp.ID)
	log.Infof("Current config: %s", clientApp.Config)
	if c.Bool("p") {
		if clientApp.Proposed != nil {
			log.Infof("Proposed config: %s", clientApp.Proposed)
		} else {
			log.Info("No proposed data")
		}
	}
	return nil
}
func dataPropose(c *cli.Context) error {
	assertCA()
	log.Fatal("Not yet implemented")
	return nil
}
func dataVote(c *cli.Context) error {
	assertCA()
	log.ErrFatal(clientApp.ProposeVote(!c.Bool("r")))
	return nil
}

func kvList(c *cli.Context) error {
	assertCA()
	log.Infof("Data for id %x", clientApp.ID)
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
		log.Fatal("Didn't find key", key, "in the data")
	}
	delete(prop.Data, key)
	log.ErrFatal(clientApp.ProposeSend(prop))
	return nil
}
func sshAdd(c *cli.Context) error {
	assertCA()
	if c.NArg() != 1 {
		log.Fatal("Please give the hostname as argument")
	}
	f, err := os.OpenFile(sshConfig, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	hostname := c.Args().First()
	alias := c.String("a")
	if alias == "" {
		alias = hostname
	}
	filePub := path.Join(sshDir, "key_"+alias+".pub")
	idPriv := "key_" + alias
	filePriv := path.Join(sshDir, idPriv)
	log.ErrFatal(makeSSHKeyPair(filePub, filePriv))
	text := fmt.Sprintf("Host %s\n\tHostName %s\n\tIdentityFile %s\n",
		alias, hostname, idPriv)
	if port := c.String("p"); port != "" {
		text += fmt.Sprintf("\tPort %s\n", port)
	}
	if user := c.String("u"); user != "" {
		text += fmt.Sprintf("\tUser %s\n", user)
	}
	if _, err = f.WriteString(text); err != nil {
		log.Fatal(err)
	}
	return nil
}
func sshLs(c *cli.Context) error {
	assertCA()
	all := c.Bool("a")
	if all {

	}
	log.Print("")
	return nil
}
func printSSH(dev string) {

}
func sshDel(c *cli.Context) error {
	log.Fatal("Not yet implemented")
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
func assertCA() {
	if clientApp == nil {
		log.Fatal("Couldn't load config-file", configFile, "or it was empty.")
	}
}

func addSSH(c *cli.Context) {
	pubFileName := config.TildeToHome(c.GlobalString("config-ssh") + "/id_rsa.pub")
	pubFile, err := os.Open(pubFileName)
	log.ErrFatal(err, "Couldn't open public-ssh: ", pubFileName)
	pub, err := ioutil.ReadAll(pubFile)
	pubFile.Close()
	log.ErrFatal(err, "Couldn't read public-ssh: ", pubFileName)
	log.Print(pub)
}

func getGroup(c *cli.Context) *config.Group {
	gfile := c.Args().Get(0)
	gr, err := os.Open(gfile)
	log.ErrFatal(err)
	groups, err := config.ReadGroupDescToml(gr)
	gr.Close()
	if groups == nil || groups.Roster == nil || len(groups.Roster.List) == 0 {
		log.Fatal("No servers found in roster from", gfile)
	}
	return groups
}

// getKeys returns the keys up to the next ":". If given a slice of keys, it
// will return sub-keys.
func getKeys(data map[string]string, keys ...string) []string {
	var ret []string
	start := strings.Join(keys, ":")
	if len(start) > 0 {
		start += ":"
	}
	for k, _ := range data {
		if strings.HasPrefix(k, start) {
			// Create subkey
			subkey := strings.TrimPrefix(k, start)
			subkey = strings.SplitN(subkey, ":", 2)[0]
			// Check if it's already in there
			isNew := true
			for _, s := range ret {
				isNew = isNew && (s != subkey)
			}
			if isNew {
				ret = append(ret, subkey)
			}
		}
	}
	sort.Strings(ret)
	return ret
}

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
// StackOverflow: Greg http://stackoverflow.com/users/328645/greg in
// http://stackoverflow.com/questions/21151714/go-generate-an-ssh-public-key
// No licence added
func makeSSHKeyPair(pubKeyPath, privateKeyPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE, 0600)
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0600)
}
