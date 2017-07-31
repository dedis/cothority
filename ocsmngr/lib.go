package main

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/dedis/onchain-secrets"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/app"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(ocsConfig{})
}

type ocsConfig struct {
	ACLBunch *onchain_secrets.SkipBlockBunch
	DocBunch *onchain_secrets.SkipBlockBunch
	Roles    *onchain_secrets.Credentials
}

// loadConfig will try to load the configuration and `fatal` if it is there but
// not valid. If the config-file is missing altogether, loaded will be false and
// an empty config-file will be returned.
func loadConfig(c *cli.Context) (cfg *ocsConfig, loaded bool) {
	cfg = &ocsConfig{}
	loaded = true

	configFile := getConfig(c)
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		log.ErrFatal(err)
	}
	_, msg, err := network.Unmarshal(buf)
	log.ErrFatal(err)
	cfg, loaded = msg.(*ocsConfig)
	if !loaded {
		log.Fatal("Wrong message-type in config-file")
	}
	if cfg.ACLBunch == nil || cfg.DocBunch == nil {
		log.Fatal("Identity doesn't hold skipblock")
	}
	return
}

// loadConfigOrFail tries to load the config and fails if it doesn't succeed.
// If a configuration has been loaded, it will update the config and propose
// part of the identity.
func loadConfigOrFail(c *cli.Context) *ocsConfig {
	cfg, loaded := loadConfig(c)
	if !loaded {
		log.Fatal("Couldn't load configuration-file")
	}
	return cfg
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func (cfg *ocsConfig) saveConfig(c *cli.Context) error {
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

// Gets the admin-role
func (cfg *ocsConfig) Admin() *onchain_secrets.Credential {
	acls := cfg.Acls()
	for _, c := range cfg.Roles.List {
		if a := acls.Admins.SearchPseudo(c.Pseudonym); a != nil && a.Public.Equal(c.Public) {
			return c
		}
	}
	return nil
}

// Gets the latest acls
func (cfg *ocsConfig) Acls() *onchain_secrets.DataACL {
	_, aclI, err := network.Unmarshal(cfg.ACLBunch.Latest.Data)
	if err != nil {
		return nil
	}
	aclsE, ok := aclI.(*onchain_secrets.DataACLEvolve)
	if !ok {
		return nil
	}
	acls := aclsE.ACL
	if acls.Admins == nil {
		acls.Admins = &onchain_secrets.Credentials{}
	}
	if acls.Writers == nil {
		acls.Writers = &onchain_secrets.Credentials{}
	}
	if acls.Readers == nil {
		acls.Readers = &onchain_secrets.Credentials{}
	}
	return acls
}

// StoreFile asks the skipchain to store the given file.
func (cfg *ocsConfig) StoreFile(writer, file string) (sb *skipchain.SkipBlock, err error) {
	cred := cfg.Roles.SearchPseudo(writer)
	if cred == nil {
		return nil, errors.New("Didn't find writer: " + writer)
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	log.ErrFatal(err)
	sb, err = onchain_secrets.NewClient().EncryptAndWriteRequest(cfg.DocBunch.Latest, data, cred)
	return
}

// CreateDocBunch returns the Doc-bunch from a slice of skipblocks and does some basic
// tests.
func CreateDocBunch(roster *onet.Roster, sid skipchain.SkipBlockID) (*onchain_secrets.SkipBlockBunch, error) {
	cl := skipchain.NewClient()
	sbsReply, err := cl.GetUpdateChain(roster, sid)
	if err != nil {
		return nil, err
	}
	sbs := sbsReply.Update
	if len(sbs) == 0 {
		return nil, errors.New("Didn't find skipchain or it is empty")
	}
	genesis := sbs[0]
	if genesis.VerifierIDs[1] != onchain_secrets.VerificationOCSDoc[1] {
		return nil, errors.New("This is not a Doc-skipchain")
	}
	if genesis.Index != 0 {
		return nil, errors.New("This is not the genesis-block")
	}
	dataOCS := onchain_secrets.NewDataOCS(genesis.Data)
	if dataOCS == nil {
		return nil, errors.New("Data is not dataOCS")
	}
	if dataOCS.Config == nil {
		return nil, errors.New("Configuration of genesis-block should be non-nil")
	}
	bunch := onchain_secrets.NewSkipBlockBunch(genesis)
	for _, sb := range sbs[1:] {
		if bunch.Store(sb) == nil {
			return nil, errors.New("Error in Skipchain")
		}
	}
	return bunch, nil
}

// CreateACLBunch returns the ACL-bunch from a slice of skipblocks and does some basic
// tests.
func CreateACLBunch(roster *onet.Roster, sid skipchain.SkipBlockID) (*onchain_secrets.SkipBlockBunch, error) {
	cl := skipchain.NewClient()
	sbsReply, err := cl.GetUpdateChain(roster, sid)
	if err != nil {
		return nil, err
	}
	sbs := sbsReply.Update
	if len(sbs) == 0 {
		return nil, errors.New("Didn't find skipchain or it is empty")
	}
	genesis := sbs[0]
	if genesis.VerifierIDs[1] != onchain_secrets.VerificationOCSACL[1] {
		return nil, errors.New("This is not a ACL-skipchain")
	}
	if genesis.Index != 0 {
		return nil, errors.New("This is not the genesis-block")
	}
	bunch := onchain_secrets.NewSkipBlockBunch(genesis)
	for _, sb := range sbs[1:] {
		if bunch.Store(sb) == nil {
			return nil, errors.New("Error in Skipchain")
		}
	}
	return bunch, nil
}

// Returns the config-file from the configuration
func getConfig(c *cli.Context) string {
	configDir := app.TildeToHome(c.GlobalString("config"))
	log.ErrFatal(mkdir(configDir, 0770))
	return configDir + "/config.bin"
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

// mkDir fails only if it is another error than an existing directory
func mkdir(n string, p os.FileMode) error {
	err := os.Mkdir(n, p)
	if !os.IsExist(err) {
		return err
	}
	return nil
}
