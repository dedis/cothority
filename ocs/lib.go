package main

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/service"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(ocsConfig{})
}

type ocsConfig struct {
	SkipChainURL *service.SkipChainURL
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
	_, msg, err := network.Unmarshal(buf, cothority.Suite)
	log.ErrFatal(err)
	cfg, loaded = msg.(*ocsConfig)
	if !loaded {
		log.Fatal("Wrong message-type in config-file")
	}
	if cfg.SkipChainURL == nil {
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
