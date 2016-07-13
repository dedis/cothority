package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/services/identity"
	"gopkg.in/codegangsta/cli.v1"
)

type CA struct {
	*identity.Identity
}

// loadCA will return nil if the config-file doesn't exist. It tries to
// load the file given in configFile.
func loadCA(c *cli.Context) (*CA, error) {
	configFile := getConfig(c)
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &CA{&identity.Identity{}}, nil
		}
		return nil, err
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	if err != nil {
		return nil, err
	}
	ca, ok := msg.(*CA)
	if !ok {
		return nil, errors.New("Wrong message-type in config-file")
	}
	return ca, nil
}

// kvGetKeys returns the keys up to the next ":". If given a slice of keys, it
// will return sub-keys.
func (ca *CA) kvGetKeys(keys ...string) []string {
	var ret []string
	start := strings.Join(keys, ":")
	if len(start) > 0 {
		start += ":"
	}
	for k := range ca.Config.Data {
		if strings.HasPrefix(k, start) {
			// Create subkey
			subkey := strings.TrimPrefix(k, start)
			subkey = strings.SplitN(subkey, ":", 2)[0]
			ret = append(ret, subkey)
		}
	}
	return sortUniq(ret)
}

// kvGetValue returns the value of the key
func (ca *CA) kvGetValue(keys ...string) string {
	key := strings.Join(keys, ":")
	for k, v := range ca.Config.Data {
		if k == key {
			return v
		}
	}
	return ""
}

// kvGetIntKeys returns the keys in the middle of prefix and suffix
func (ca *CA) kvGetIntKeys(prefix, suffix string) []string {
	var ret []string
	if len(prefix) > 0 {
		prefix += ":"
	}
	if len(suffix) > 0 {
		suffix = ":" + suffix
	}
	for k := range ca.Config.Data {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			interm := strings.TrimPrefix(k, prefix)
			interm = strings.TrimSuffix(interm, suffix)
			if !strings.Contains(interm, ":") {
				ret = append(ret, interm)
			}
		}
	}
	return sortUniq(ret)
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func (ca *CA) saveConfig(c *cli.Context) error {
	configFile := getConfig(c)
	if ca == nil {
		return errors.New("Cannot save empty clientApp")
	}
	buf, err := network.MarshalRegisteredType(ca)
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

// Asserts that the clientApp exists, else fatals
func assertCA(c *cli.Context) *CA {
	ca, err := loadCA(c)
	log.ErrFatal(err, "Problems reading config-file. Most probably you\n",
		"should start a new one by running with the 'setup'\n",
		"argument.")
	if ca == nil || ca.ManagerStr == "" {
		log.Fatal("Couldn't load config-file or it was empty.")
	}
	return ca
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

// sortUniq sorts the slice of strings and deletes duplicates
func sortUniq(slice []string) []string {
	sorted := make([]string, len(slice))
	copy(sorted, slice)
	sort.Strings(sorted)
	var ret []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			ret = append(ret, s)
		}
	}
	return ret
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
