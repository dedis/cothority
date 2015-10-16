package main

import (
	"errors"
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/app/conode/defs"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/abstract"
	"net"
	"strconv"
)

// which mode are we running
var key, validate bool
var check, build string

// Which suite to use
var suiteString string = "ed25519"
var suite abstract.Suite

//// 		Key part 		////
// The hostname / address of the host we generate key for
var address string = ""

// where to write the key file .priv + .pub
var out string = "key"

// Returns the name of the file for the private key
func namePriv() string {
	return out + ".priv"
}

// Returns the name of the file for the public key
func namePub() string {
	return out + ".pub"
}

///////////////////////

// Init function set up the flag reading
func init() {
	flag.StringVar(&suiteString, "suite", suiteString, "Suite to use throughout the process [ed25519]")
	flag.BoolVar(&key, "key", false, "Key will generate the keys that will be used in the cothority project and will write them to files\n")
	flag.StringVar(&address, "address", "", "External IP address so we know who you are. If not supplied when calling 'key' or in run mode, it will panic!")
	flag.BoolVar(&validate, "validate", false, "Validate waits for the connection of a verifier / checker from the head of the cothority project.\n"+
		"\tIt will send some systems stats and a signature on it in order to verify the public / private keys.\n")
	flag.StringVar(&check, "check", "", "ip_address:port [-public publicFile]\n"+
		"\tcheck will launch the check on the host. Basically, it requests some system stats, \n"+
		"\tand a signature in order to check the host system and the public / private keys of the host.\n"+
		"\tip_address:port is the address of the host we want to verify\n")
	flag.StringVar(&build, "build", "", "Builds the configuration file (included the tree) out of a file with hostnames")

}

func main() {
	// parse the flags
	flag.Parse()
	// setup the suite
	suite = app.GetSuite(suiteString)
	// Lets check everything is in order
	verifyArgs()

	switch {
	case key:
		KeyGeneration()
	case validate:
		Validation()
	case check != "":
		Check(check)
	case build != "":
		Build(build)
	default:
		dbg.Lvl1("Starting conode -> in run mode")
		conf := &app.ConfigConode{}
		if err := app.ReadTomlConfig(conf, configFile); err != nil {
			dbg.Fatal("Could not read toml config... : ", err)
		}
		dbg.Lvl1("Configuration file read")
		RunServer(conf)
	}
}

// verifyArgs will check if some arguments get the right value or is some are
// missing
func verifyArgs() {
	if suite == nil {
		dbg.Fatal("Suite could not be recognized. Use a proper suite [ed25519] ( given ", suiteString, ")")
	}
}

// KeyGeneration will generate a fresh public / private key pair
// and write those down into two separate files
func KeyGeneration() {
	if address == "" {
		dbg.Fatal("You must call key with -address [ipadress] !")
	}
	address, err := cliutils.UpsertPort(address, defs.DefaultPort)
	if err != nil {
		dbg.Fatal(err)
	}
	// gen keypair
	kp := cliutils.KeyPair(suite)
	// Write private
	if err := cliutils.WritePrivKey(kp.Secret, suite, namePriv()); err != nil {
		dbg.Fatal("Error writing private key file : ", err)
	}

	// Write public
	if err := cliutils.WritePubKey(kp.Public, suite, namePub(), address); err != nil {
		dbg.Fatal("Error writing public key file : ", err)
	}

	dbg.Lvl1("Keypair generated and written to ", namePriv(), " / ", namePub())
}

func RunServer(conf *app.ConfigConode) {

	var err error
	// make sure address has a port or insert default one
	address, err = cliutils.UpsertPort(address, defs.DefaultPort)
	if err != nil {
		dbg.Fatal(err)
	}
	// load the configuration
	//dbg.Lvl3("loading configuration")
	var hc *graphs.HostConfig
	opts := graphs.ConfigOptions{ConnType: "tcp", Host: address, Suite: suite}

	hc, err = graphs.LoadConfig(conf.Hosts, conf.Tree, opts)
	if err != nil {
		dbg.Fatal(err)
	}

	err = hc.Run(true, sign.MerkleTree, address)
	if err != nil {
		dbg.Fatal(err)
	}

	defer func(sn *sign.Node) {
		dbg.Lvl2("Program timestamper has terminated:", address)
		sn.Close()
	}(hc.SNodes[0])

	stampers, err := RunTimestamper(hc, 0, address)
	if err != nil {
		dbg.Fatal(err)
	}
	for _, s := range stampers {
		// only listen if this is the hostname specified
		if s.Name() == address {
			s.Hostname = address
			s.App = "stamp"
			if s.IsRoot(0) {
				dbg.Lvl1("Root timestamper at:", address)
				s.Run("root")

			} else {
				dbg.Lvl1("Running regular timestamper on:", address)
				s.Run("regular")
			}
		}
	}
}

// run each host in hostnameSlice with the number of clients given
func RunTimestamper(hc *graphs.HostConfig, nclients int, hostnameSlice ...string) ([]*Server, error) {
	dbg.Lvl3("RunTimestamper on", hc.Hosts)
	hostnames := make(map[string]*sign.Node)
	// make a list of hostnames we want to run
	if hostnameSlice == nil {
		hostnames = hc.Hosts
	} else {
		for _, h := range hostnameSlice {
			sn, ok := hc.Hosts[h]
			if !ok {
				return nil, errors.New("hostname given not in config file:" + h)
			}
			hostnames[h] = sn
		}
	}
	// for each client in
	stampers := make([]*Server, 0, len(hostnames))
	for _, sn := range hc.SNodes {
		if _, ok := hostnames[sn.Name()]; !ok {
			dbg.Lvl1("signing node not in hostnmaes")
			continue
		}
		stampers = append(stampers, NewServer(sn))
		if hc.Dir == nil {
			dbg.Lvl3(hc.Hosts, "listening for clients")
			stampers[len(stampers)-1].Listen()
		}
	}
	dbg.Lvl3("stampers:", stampers)
	for _, s := range stampers[1:] {

		_, p, err := net.SplitHostPort(s.Name())
		if err != nil {
			log.Fatal("RunTimestamper: bad Tcp host")
		}
		pn, err := strconv.Atoi(p)
		if hc.Dir != nil {
			pn = 0
		} else if err != nil {
			log.Fatal("port ", pn, "is not valid integer")
		}
		//dbg.Lvl4("client connecting to:", hp)

	}

	return stampers, nil
}
