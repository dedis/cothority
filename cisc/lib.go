package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"

	"strings"

	"path/filepath"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/identity"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(ciscConfig{})
}

type ciscConfig struct {
	// Identities is a slice of all identities we have the
	// private key for.
	Identities []*identity.Identity
	// Follow is the identities we're following and where we search for new
	// ssh public keys to include in our authorized_keys.
	Follow []*identity.Identity
	// admin key pairs. Key of map is address of conode
	KeyPairs map[string]*key.Pair
}

func newCiscConfig(i *identity.Identity) *ciscConfig {
	return &ciscConfig{Identities: []*identity.Identity{i},
		KeyPairs: make(map[string]*key.Pair)}
}

// loadConfig will try to load the configuration and `fatal` if it is there but
// not valid. If the config-file is missing altogether, loaded will be false and
// an empty config-file will be returned.
func loadConfig(c *cli.Context) (cfg *ciscConfig, loaded bool) {
	cfg = &ciscConfig{KeyPairs: make(map[string]*key.Pair)}
	loaded = true

	configFile := getConfig(c)
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.ErrFatal(err)
	}
	_, msg, err := network.Unmarshal(buf, cothority.Suite)
	log.ErrFatal(err)
	cfg, loaded = msg.(*ciscConfig)
	for _, i := range cfg.Identities {
		i.Client = onet.NewClient(cothority.Suite, identity.ServiceName)
	}
	for _, f := range cfg.Follow {
		f.Client = onet.NewClient(cothority.Suite, identity.ServiceName)
	}
	if !loaded {
		log.Fatal("Wrong message-type in config-file")
	}
	if len(cfg.KeyPairs) == 0 {
		cfg.KeyPairs = map[string]*key.Pair{}
	}
	return
}

// loadConfigOrFail tries to load the config and fails if it doesn't succeed.
// If a configuration has been loaded, it will update the config and propose
// part of the identity.
func loadConfigOrFail(c *cli.Context) *ciscConfig {
	cfg, loaded := loadConfig(c)
	if !loaded {
		log.Fatal("Couldn't load configuration-file")
	}
	return cfg
}

// loadConfigAdminOrFail tries to load the config and fails if it doesn't succeed.
// it doesn't load data and propose updates unlike loadConfigOrFail
func loadConfigAdminOrFail(c *cli.Context) *ciscConfig {
	cfg, loaded := loadConfig(c)
	if !loaded {
		log.Fatal("Couldn't load configuration-file")
	}
	return cfg
}

// update gets new data for all identities
func (cfg *ciscConfig) update() error {
	for _, id := range cfg.Identities {
		if err := id.DataUpdate(); err != nil {
			return err
		}
		if err := id.ProposeUpdate(); err != nil {
			return err
		}
	}
	return nil
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func (cfg *ciscConfig) saveConfig(c *cli.Context) error {
	configFile := getConfig(c)
	if cfg == nil {
		return errors.New("Cannot save empty clientApp")
	}
	buf, err := network.Marshal(cfg)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Lvl2("Saving to", configFile)
	return ioutil.WriteFile(configFile, buf, 0660)
}

// convenience function to send and vote a proposition and update.
func (cfg *ciscConfig) proposeSendVoteUpdate(id *identity.Identity, p *identity.Data) {
	log.ErrFatal(id.ProposeSend(p))
	log.ErrFatal(id.ProposeVote(true))
	log.ErrFatal(id.DataUpdate())
}

// writes the ssh-keys to an 'authorized_keys.cisc'-file. If
// `authorized_keys` doesn't exist, it will be created as a
// soft-link pointing to `authorized_keys.cisc`.
func (cfg *ciscConfig) writeAuthorizedKeys(c *cli.Context) {
	var keys []string
	dir, _ := sshDirConfig(c)
	authKeys := filepath.Join(dir, "authorized_keys")
	authKeysCisc := authKeys + ".cisc"
	if _, err := os.Stat(authKeys); os.IsNotExist(err) {
		log.Info("Making link from authorized_keys to authorized_keys.cisc")
		os.Symlink(authKeysCisc, authKeys)
	}
	for _, f := range cfg.Follow {
		log.Lvlf2("Parsing IC %x", f.ID)
		for _, s := range f.Data.GetIntermediateColumn("ssh", f.DeviceName) {
			pub := f.Data.GetValue("ssh", s, f.DeviceName)
			log.Lvlf2("Value of %s is %s", s, pub)
			log.Info("Writing key for", s, "to authorized_keys")
			keys = append(keys, pub+" "+s+"@"+f.DeviceName)
		}
	}
	err := ioutil.WriteFile(authKeysCisc,
		[]byte(strings.Join(keys, "\n")), 0600)
	log.ErrFatal(err)
}

// showDifference compares the propose and the config-part
func (cfg *ciscConfig) showDifference(id *identity.Identity) {
	if id.Proposed == nil {
		log.Info("No proposed config found")
		return
	}
	for k, v := range id.Proposed.Storage {
		orig, ok := id.Data.Storage[k]
		if !ok || v != orig {
			log.Infof("New or changed key: %s/%s", k, v)
		}
	}
	for k := range id.Data.Storage {
		_, ok := id.Proposed.Storage[k]
		if !ok {
			log.Info("Deleted key:", k)
		}
	}
	for dev, pub := range id.Proposed.Device {
		if _, exists := id.Data.Device[dev]; !exists {
			log.Infof("New device: %s / %s", dev,
				pub.Point.String())
		}
	}
	for dev := range id.Data.Device {
		if _, exists := id.Proposed.Device[dev]; !exists {
			log.Info("Deleted device:", dev)
		}
	}
	if id.Proposed.Roster != nil {
		log.Info("Changing roster:")
		log.Info("Old:", id.Data.Roster.List)
		log.Info("New:", id.Proposed.Roster.List)
	}
}

// shows only the keys, but not the data
func (cfg *ciscConfig) showKeys(id *identity.Identity) {
	for d := range id.Data.Device {
		log.Info("Connected device", d)
	}
	for k := range id.Data.Storage {
		log.Info("Key set", k)
	}
}

func (cfg *ciscConfig) findSC(idHex string) (*identity.Identity, error) {
	id, err := hex.DecodeString(idHex)
	if err != nil {
		return nil, errors.New("hex-decoding error: " + err.Error())
	}
	for _, i := range cfg.Identities {
		if i.ID.FuzzyEqual(id) {
			if err := i.DataUpdate(); err != nil {
				return nil, err
			}
			if err := i.ProposeUpdate(); err != nil {
				return nil, err
			}
			return i, nil
		}
	}
	return nil, nil
}

// Returns the config-file from the configuration
func getConfig(c *cli.Context) string {
	configDir := app.TildeToHome(c.GlobalString("config"))
	log.ErrFatal(mkdir(configDir, 0770))
	return configDir + "/config.bin"
}

// Returns the config-file for admins containing key-pair
func getKeyConfig(c *cli.Context) string {
	configDir := app.TildeToHome(c.GlobalString("config"))
	log.ErrFatal(mkdir(configDir, 0770))
	return configDir + "/admin_key.bin"
}

// Reads the group-file and returns it
func getGroup(c *cli.Context) *app.Group {
	gfile := c.Args().Get(0)
	gr, err := os.Open(gfile)
	log.ErrFatal(err)
	defer gr.Close()
	groups, err := app.ReadGroupDescToml(gr)
	log.ErrFatal(err)
	if groups == nil || groups.Roster == nil || len(groups.Roster.List) == 0 {
		log.Fatal("No servers found in roster from", gfile)
	}
	return groups
}

// retrieves ssh-directory and ssh-config-name.
func sshDirConfig(c *cli.Context) (sshDir string, sshConfig string) {
	sshDir = app.TildeToHome(c.GlobalString("cs"))
	log.ErrFatal(mkdir(sshDir, 0700))
	sshConfig = sshDir + "/config"
	return
}

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
// StackOverflow: Greg http://stackoverflow.com/users/328645/greg in
// http://stackoverflow.com/questions/21151714/go-generate-an-ssh-public-key
// No licence added
func makeSSHKeyPair(bits int, pubKeyPath, privateKeyPath string) error {
	if bits < 1024 {
		return errors.New("Reject using too few bits for key")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
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

// mkDir fails only if it is another error than an existing directory
func mkdir(n string, p os.FileMode) error {
	err := os.Mkdir(n, p)
	if !os.IsExist(err) {
		return err
	}
	return nil
}
