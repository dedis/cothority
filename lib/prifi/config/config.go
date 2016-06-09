package config

import (
	"github.com/dedis/cothority/lib/prifi/dcnet"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/suites"
)

//used to make sure everybody has the same version of the software. must be updated manually
const LLD_PROTOCOL_VERSION = 3

//sets the crypto suite used
var CryptoSuite = edwards.NewAES128SHA256Ed25519(false) //nist.NewAES128SHA256P256()

//sets the factory for the dcnet's cell encoder/decoder
var Factory = dcnet.SimpleCoderFactory

var configFile config.File

// Dissent config file format
type ConfigData struct {
	Keys config.Keys // Info on configured key-pairs
}

var configData ConfigData
var keyPairs []config.KeyPair

func ReadConfig() error {

	// Load the configuration file
	configFile.Load("dissent", &configData)

	// Read or create our public/private keypairs
	pairs, err := configFile.Keys(&configData.Keys, suites.All(), CryptoSuite)
	if err != nil {
		return err
	}
	keyPairs = pairs
	println("Loaded", len(pairs), "key-pairs")

	return nil
}
