package main

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/logread"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/app"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(wlrConfig{})
}

type wlrConfig struct {
	ACLBunch *logread.SkipBlockBunch
	WLRBunch *logread.SkipBlockBunch
	Roles    *logread.Credentials
}

// loadConfig will try to load the configuration and `fatal` if it is there but
// not valid. If the config-file is missing altogether, loaded will be false and
// an empty config-file will be returned.
func loadConfig(c *cli.Context) (cfg *wlrConfig, loaded bool) {
	cfg = &wlrConfig{}
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
	cfg, loaded = msg.(*wlrConfig)
	if !loaded {
		log.Fatal("Wrong message-type in config-file")
	}
	if cfg.ACLBunch == nil || cfg.WLRBunch == nil {
		log.Fatal("Identity doesn't hold skipblock")
	}
	return
}

// loadConfigOrFail tries to load the config and fails if it doesn't succeed.
// If a configuration has been loaded, it will update the config and propose
// part of the identity.
func loadConfigOrFail(c *cli.Context) *wlrConfig {
	cfg, loaded := loadConfig(c)
	if !loaded {
		log.Fatal("Couldn't load configuration-file")
	}
	return cfg
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func (cfg *wlrConfig) saveConfig(c *cli.Context) error {
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
func (cfg *wlrConfig) Admin() *logread.Credential {
	acls := cfg.Acls()
	for _, c := range cfg.Roles.List {
		if a := acls.Admins.SearchPseudo(c.Pseudonym); a != nil && a.Public.Equal(c.Public) {
			return c
		}
	}
	return nil
}

// Gets the latest acls
func (cfg *wlrConfig) Acls() *logread.DataACL {
	_, aclI, err := network.Unmarshal(cfg.ACLBunch.Latest.Data)
	if err != nil {
		return nil
	}
	aclsE, ok := aclI.(*logread.DataACLEvolve)
	if !ok {
		return nil
	}
	acls := aclsE.ACL
	if acls.Admins == nil {
		acls.Admins = &logread.Credentials{}
	}
	if acls.Writers == nil {
		acls.Writers = &logread.Credentials{}
	}
	if acls.Readers == nil {
		acls.Readers = &logread.Credentials{}
	}
	return acls
}

// StoreFile asks the skipchain to store the given file.
func (cfg *wlrConfig) StoreFile(writer, file string) (sb *skipchain.SkipBlock, err error) {
	cred := cfg.Roles.SearchPseudo(writer)
	if cred == nil {
		return nil, errors.New("Didn't find writer: " + writer)
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	log.ErrFatal(err)
	sb, err = logread.NewClient().EncryptAndWriteRequest(cfg.WLRBunch.Latest, data, cred)
	return
}

// CreateWLRBunch returns the WLR-bunch from a slice of skipblocks and does some basic
// tests.
func CreateWLRBunch(roster *onet.Roster, sid skipchain.SkipBlockID) (*logread.SkipBlockBunch, error) {
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
	if genesis.VerifierIDs[1] != logread.VerificationLogreadWLR[1] {
		return nil, errors.New("This is not a WLR-skipchain")
	}
	if genesis.Index != 0 {
		return nil, errors.New("This is not the genesis-block")
	}
	dataWlr := logread.NewDataWlr(genesis.Data)
	if dataWlr == nil {
		return nil, errors.New("Data is not dataWLR")
	}
	if dataWlr.Config == nil {
		return nil, errors.New("Configuration of genesis-block should be non-nil")
	}
	bunch := logread.NewSkipBlockBunch(genesis)
	for _, sb := range sbs[1:] {
		if bunch.Store(sb) == nil {
			return nil, errors.New("Error in Skipchain")
		}
	}
	return bunch, nil
}

// CreateACLBunch returns the ACL-bunch from a slice of skipblocks and does some basic
// tests.
func CreateACLBunch(roster *onet.Roster, sid skipchain.SkipBlockID) (*logread.SkipBlockBunch, error) {
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
	if genesis.VerifierIDs[1] != logread.VerificationLogreadACL[1] {
		return nil, errors.New("This is not a ACL-skipchain")
	}
	if genesis.Index != 0 {
		return nil, errors.New("This is not the genesis-block")
	}
	bunch := logread.NewSkipBlockBunch(genesis)
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
