package lib

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ConfigPath points to where the files will be stored by default.
var ConfigPath = "."

// Config is the structure used by ol to save its configuration. It holds everything
// necessary to talk to a ByzCoin instance. The GenesisDarc and AdminIdentity
// can change over the lifetime of a ledger.
type Config struct {
	Roster        onet.Roster
	ByzCoinID     skipchain.SkipBlockID
	GenesisDarc   darc.Darc
	AdminIdentity darc.Identity
}

//BaseConfig is a smaller version of Config containing only the necessary material
//to export the Config using, for example, a QR code
type BaseConfig struct {
	ByzCoinID skipchain.SkipBlockID
}

//AAdminConfig is a smaller version of Config containing only the necessary material
//to export the Config with its admin credentials using, for example, a QR code
type AdminConfig struct {
	ByzCoinID skipchain.SkipBlockID
	Admin     string
}

// LoadKey returns the signer of a given identity. It searches it in the ConfigPath.
func LoadKey(id darc.Identity) (*darc.Signer, error) {
	// Find private key file.
	fn := fmt.Sprintf("key-%s.cfg", id)
	fn = filepath.Join(ConfigPath, fn)
	return LoadSigner(fn)
}

// LoadKeyFromString returns a signer based on a string representing the public identity of the signer
func LoadKeyFromString(id string) (*darc.Signer, error) {
	// Find private key file.
	fn := fmt.Sprintf("key-%s.cfg", id)
	fn = filepath.Join(ConfigPath, fn)
	return LoadSigner(fn)
}

// LoadSigner loads a signer from a file given by fn.
func LoadSigner(fn string) (*darc.Signer, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	var signer darc.Signer
	err = protobuf.DecodeWithConstructors(buf, &signer,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	return &signer, err
}

// SaveKey stores a signer in a file.
func SaveKey(signer darc.Signer) error {
	os.MkdirAll(ConfigPath, 0755)

	fn := fmt.Sprintf("key-%s.cfg", signer.Identity())
	fn = filepath.Join(ConfigPath, fn)

	// perms = 0400 because there is key material inside this file.
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0400)
	if err != nil {
		return fmt.Errorf("could not write %v: %v", fn, err)
	}

	buf, err := protobuf.Encode(&signer)
	if err != nil {
		return err
	}
	_, err = f.Write(buf)
	if err != nil {
		return err
	}
	return f.Close()
}

// SaveConfig stores the config in the ConfigPath directory. It returns the
// pathname of the stored file.
func SaveConfig(cfg Config) (string, error) {
	os.MkdirAll(ConfigPath, 0755)

	fn := fmt.Sprintf("bc-%x.cfg", cfg.ByzCoinID)
	fn = filepath.Join(ConfigPath, fn)

	buf, err := protobuf.Encode(&cfg)
	if err != nil {
		return fn, err
	}
	err = ioutil.WriteFile(fn, buf, 0644)
	if err != nil {
		return fn, err
	}

	return fn, nil
}

// LoadConfig returns a config read from the file and an initialized
// Client that can be used to communicate with ByzCoin.
func LoadConfig(file string) (cfg Config, cl *byzcoin.Client, err error) {
	var cfgBuf []byte
	cfgBuf, err = ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = protobuf.DecodeWithConstructors(cfgBuf, &cfg,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return
	}
	cl = byzcoin.NewClient(cfg.ByzCoinID, cfg.Roster)
	return
}

// ReadRoster reads a roster file from disk.
func ReadRoster(file string) (r *onet.Roster, err error) {
	in, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("Could not open roster %v: %v", file, err)
	}
	defer in.Close()

	group, err := app.ReadGroupDescToml(in)
	if err != nil {
		return nil, err
	}

	if len(group.Roster.List) == 0 {
		return nil, errors.New("empty roster")
	}
	return group.Roster, nil
}
