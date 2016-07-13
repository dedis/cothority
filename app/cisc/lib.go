package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/services/identity"
	"gopkg.in/codegangsta/cli.v1"
)

// loadConfig will try to load the configuration and fail if it can't load it.
func loadConfig(c *cli.Context) *identity.Identity {
	configFile := getConfig(c)
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &identity.Identity{}
		}
		log.Fatal(err)
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	log.ErrFatal(err)
	cfg, ok := msg.(*identity.Identity)
	if !ok {
		log.Fatal("Wrong message-type in config-file")
	}
	return cfg
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func saveConfig(c *cli.Context, cfg *identity.Identity) error {
	configFile := getConfig(c)
	if cfg == nil {
		return errors.New("Cannot save empty clientApp")
	}
	buf, err := network.MarshalRegisteredType(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, buf, 0660)
}

// Returns the config-file from the configuration
func getConfig(c *cli.Context) string {
	configDir := config.TildeToHome(c.GlobalString("config"))
	os.Mkdir(configDir, 0660)
	return configDir + "/config.bin"
}

// Reads the group-file and returns it
func getGroup(c *cli.Context) *config.Group {
	gfile := c.Args().Get(0)
	gr, err := os.Open(gfile)
	log.ErrFatal(err)
	defer gr.Close()
	groups, err := config.ReadGroupDescToml(gr)
	log.ErrFatal(err)
	if groups == nil || groups.Roster == nil || len(groups.Roster.List) == 0 {
		log.Fatal("No servers found in roster from", gfile)
	}
	return groups
}

// retrieves ssh-config-name and ssh-directory
func sshDirConfig(c *cli.Context) (string, string) {
	sshDir := config.TildeToHome(c.GlobalString("cs"))
	return sshDir, sshDir + "/config"
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
