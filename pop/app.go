// This is the command line interface to communicate with the pop service.
//
// More details can be found here -
// https://github.com/dedis/cothority/blob/master/pop/README.md.
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ftcosi/check"
	_ "github.com/dedis/cothority/ftcosi/protocol"
	_ "github.com/dedis/cothority/ftcosi/service"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/protobuf"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(Config{})
}

// Config represents either a manager or an attendee configuration.
type Config struct {
	// Public key of org. Used for linking and
	// org authentication
	OrgPublic kyber.Point
	// Private key of org. Used for authentication
	OrgPrivate kyber.Scalar
	// Address of the linked conode.
	Address network.Address
	// Map of Final statements of the parties.
	// indexed by hash of party desciption
	Parties map[string]*PartyConfig
	// config-file name
	name string
}

// PartyConfig represents local configuration of party
type PartyConfig struct {
	// Private key of attendee or organizer, depending on value
	// of Index.
	Private kyber.Scalar
	// Public key of attendee or organizer, depending on value of
	// index.
	Public kyber.Point
	// Index of the attendee in the final statement. If the index
	// is -1, then this pop holds an organizer.
	Index int
	// Final statement of the party.
	Final *service.FinalStatement
}

func main() {
	appCli := cli.NewApp()
	appCli.Name = "Proof-of-personhood party"
	appCli.Usage = "Handles party-creation, finalizing, pop-token creation, and verification"
	appCli.Version = "0.1"
	appCli.Commands = []cli.Command{}
	appCli.Commands = []cli.Command{
		commandOmniledger,
		commandOrg,
		commandAttendee,
		commandAuth,
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action: func(c *cli.Context) error {
				return check.Config(c.Args().First(), false)
			},
		},
	}
	appCli.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug,d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name: "config,c",
			// we use GetDataPath because only non-human-readable
			// data files are stored here
			Value: cfgpath.GetDataPath("pop"),
			Usage: "The configuration-directory of pop",
		},
	}
	appCli.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(appCli.Run(os.Args))
}

// links this pop to a cothority
func orgLink(c *cli.Context) error {
	log.Lvl3("Org: Link")
	if c.NArg() == 0 {
		log.Fatal("Please give an IP and optionally a pin")
	}
	cfg, client := getConfigClient(c)

	addr := network.NewAddress(network.PlainTCP, c.Args().First())
	pin := c.Args().Get(1)
	if err := client.PinRequest(addr, pin, cfg.OrgPublic); err != nil {
		// Compare by string because this comes over the network.
		if strings.Contains(err.Error(), service.ErrorReadPIN.Error()) {
			log.Info("Please read PIN in server-log")
			return nil
		}
		return err
	}
	cfg.Address = addr
	log.Info("Successfully linked with", addr)
	cfg.write()
	return nil
}

// sets up a configuration
func orgConfig(c *cli.Context) error {
	log.Lvl3("Org: Config")
	if c.NArg() < 1 {
		log.Fatal("Please give pop_desc.toml and (optionally) merge_party.toml")
	}
	cfg, client := getConfigClient(c)
	if cfg.Address.String() == "" {
		return errors.New("No address found - please link first")
	}
	desc := &service.PopDesc{}
	pdFile := c.Args().First()
	buf, err := ioutil.ReadFile(pdFile)
	if err != nil {
		return fmt.Errorf("error: %s - while reading pop-description: %s", err, pdFile)
	}
	err = decodePopDesc(string(buf), desc)
	if err != nil {
		return fmt.Errorf("error: %s - while decoding pop-description: %s", err, pdFile)
	}
	if c.NArg() == 2 {
		mergeFile := c.Args().Get(1)
		buf, err = ioutil.ReadFile(mergeFile)
		if err != nil {
			return fmt.Errorf("error: %s - while reading merge_party: %s", err, mergeFile)
		}
		desc.Parties, err = decodeGroups(string(buf))
		if err != nil {
			return fmt.Errorf("error: %s - while decoding merge_party: %s", err, mergeFile)
		}

		// Check that current party is included in merge config
		found := false
		for _, party := range desc.Parties {
			if service.Equal(desc.Roster, party.Roster) {
				found = true
				break
			}
		}
		if !found {
			log.Fatal("party is not included in merge config")
		}
	}
	hash := hex.EncodeToString(desc.Hash())
	log.Lvlf2("Hash of config: %s", hash)
	err = client.StoreConfig(cfg.Address, desc, cfg.OrgPrivate)
	if err != nil {
		return err
	}
	if val, ok := cfg.Parties[hash]; !ok {
		kp := key.NewKeyPair(cothority.Suite)
		cfg.Parties[hash] = &PartyConfig{
			Index: -1,
			Final: &service.FinalStatement{
				Desc:      desc,
				Attendees: []kyber.Point{},
				Signature: []byte{},
			},
			Public:  kp.Public,
			Private: kp.Private,
		}
	} else {
		val.Final.Desc = desc
	}
	log.Infof("Stored new config with hash %x", desc.Hash())
	cfg.write()
	return nil
}

// read all newly proposed configs
func orgProposed(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("Please give IP:Port of the conode you want to read proposed configs from")
	}
	client := service.NewClient()
	proposed, err := client.GetProposals(network.Address("tls://" + c.Args().First()))
	if err != nil {
		return err
	}
	if len(proposed) == 0 {
		log.Info("Didn't find any proposed configurations")
		return nil
	}
	if !c.Bool("quiet") {
		log.Info("Found", len(proposed), "configurations:")
	}
	for i, pd := range proposed {
		if !c.Bool("quiet") {
			log.Infof("Configuration #%d", i)
		}
		p := PopDescGroupToml{
			Name:     pd.Name,
			DateTime: pd.DateTime,
			Location: pd.Location,
		}
		for _, s := range pd.Roster.List {
			st := &app.ServerToml{
				Address:     s.Address,
				Suite:       cothority.Suite.String(),
				Public:      s.Public.String(),
				Description: s.Description,
			}
			p.Servers = append(p.Servers, st)
		}
		var buf bytes.Buffer
		err = toml.NewEncoder(&buf).Encode(p)
		if err != nil {
			return err
		}
		// Here we use fmt.Print because this toml should be copy/pastable
		// or redirectable into a file.
		fmt.Print(strings.Replace(buf.String(), "\n\n", "\n", -1))
	}
	return nil
}

// adds a public key to the list
func orgPublic(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("Please give a public key and hash of a party")
	}
	log.Lvl3("Org: Adding public keys", c.Args().First())
	str := c.Args().First()
	if !strings.HasPrefix(str, "[") {
		str = "[" + str + "]"
	}
	// TODO: better cleanup rules
	str = strings.Replace(str, "\"", "", -1)
	str = strings.Replace(str, "[", "", -1)
	str = strings.Replace(str, "]", "", -1)
	str = strings.Replace(str, "\\", "", -1)
	log.Lvl3("Niceified public keys are:\n", str)
	keys := strings.Split(str, ",")
	cfg, _ := getConfigClient(c)
	party, err := cfg.getPartybyHash(c.Args().Get(1))
	if err != nil {
		return err
	}
	for _, k := range keys {
		pub, err := encoding.StringHexToPoint(cothority.Suite, k)
		if err != nil {
			log.Fatal("Couldn't parse public key:", k, err)
		}
		for _, p := range party.Final.Attendees {
			if p.Equal(pub) {
				log.Fatal("This key already exists")
			}
		}
		party.Final.Attendees = append(party.Final.Attendees, pub)
	}
	cfg.write()
	return nil
}

// finalizes the statement
func orgFinal(c *cli.Context) error {
	log.Lvl3("Org: Final")
	if c.NArg() < 1 {
		log.Fatal("Please give hash of pop-party")
	}
	cfg, client := getConfigClient(c)

	if len(cfg.Parties) == 0 {
		log.Fatal("No configs stored - first store at least one")
	}
	if cfg.Address == "" {
		log.Fatal("Not linked")
	}
	party, err := cfg.getPartybyHash(c.Args().First())
	if err != nil {
		return err
	}
	if len(party.Final.Signature) > 0 {
		var finst []byte
		finst, err = party.Final.ToToml()
		if err != nil {
			return err
		}
		log.Lvl2("Final statement already here:\n", "\n"+string(finst))
		return nil
	}
	fs, err := client.Finalize(cfg.Address, party.Final.Desc,
		party.Final.Attendees, cfg.OrgPrivate)
	if err != nil {
		return err
	}
	party.Final = fs
	cfg.write()
	finst, err := fs.ToToml()
	if err != nil {
		return err
	}
	log.Lvl2("Created final statement:\n", "\n"+string(finst))
	return nil
}

// sends Merge request
func orgMerge(c *cli.Context) error {
	log.Lvl3("Org:Merge")
	if c.NArg() < 1 {
		log.Fatal("Please give party-hash")
	}
	cfg, client := getConfigClient(c)
	if cfg.Address == "" {
		log.Fatal("Not linked")
	}
	party, err := cfg.getPartybyHash(c.Args().First())
	if err != nil {
		return err
	}
	if len(party.Final.Signature) <= 0 || party.Final.Verify() != nil {
		log.Lvl2("The local config is not finished yet")
		log.Lvl2("Fetching final statement")
		fs, err := client.FetchFinal(cfg.Address, party.Final.Desc.Hash())
		if err != nil {
			return err
		}
		if len(fs.Signature) <= 0 || fs.Verify() != nil {
			log.Fatal("Fetched final statement is invalid")
		}
		party.Final = fs
		cfg.write()
	}
	if party.Final.Merged {
		var finst []byte
		finst, err = party.Final.ToToml()
		if err != nil {
			return err
		}
		log.Lvl1("Merged final statement:\n", "\n"+string(finst))
		return nil
	}
	if len(party.Final.Desc.Parties) <= 0 {
		log.Fatal("there is no parties to merge")
	}

	fs, err := client.Merge(cfg.Address, party.Final.Desc, cfg.OrgPrivate)
	if err != nil {
		return err
	}
	party.Final = fs
	cfg.write()
	finst, err := fs.ToToml()
	if err != nil {
		return err
	}
	log.Lvl1("Created merged final statement:\n", "\n"+string(finst))
	return nil
}

// creates a new private/public pair
func attCreate(c *cli.Context) error {
	kp := key.NewKeyPair(cothority.Suite)
	secStr, err := encoding.ScalarToStringHex(nil, kp.Private)
	if err != nil {
		return err
	}
	pubStr, err := encoding.PointToStringHex(nil, kp.Public)
	if err != nil {
		return err
	}
	log.Infof("Private: %s\nPublic: %s", secStr, pubStr)
	return nil
}

// joins a poparty
func attJoin(c *cli.Context) error {
	log.Lvl3("att: join")
	if c.NArg() < 2 {
		log.Fatal("Please give private key and final.toml")
	}
	priv, err := encoding.StringHexToScalar(cothority.Suite, c.Args().First())
	if err != nil {
		return err
	}
	cfg, client := getConfigClient(c)

	finalName := c.Args().Get(1)
	buf, err := ioutil.ReadFile(finalName)
	if err != nil {
		return err
	}
	final, err := service.NewFinalStatementFromToml(buf)
	if err != nil {
		return err
	}
	if len(final.Signature) <= 0 || final.Verify() != nil {
		log.Lvl2("The local config is not finished yet")
		if cfg.Address != "" {
			log.Lvl2("Fetching final statement")
			// Need to get the updated version of party config
			// Cause attendee doesn't know,
			// whether it has finished successfully or not
			fs, err := client.FetchFinal(cfg.Address, final.Desc.Hash())
			if err != nil {
				return err
			}
			if len(fs.Signature) <= 0 || fs.Verify() != nil {
				log.Fatal("Fetched final statement is invalid")
			}
			final = fs
		} else {
			log.Fatal("No address of conode to download final statement from")
		}
	}

	if len(final.Desc.Parties) > 0 && !final.Merged {
		log.Lvl2("The local party is not merged yet")
		if cfg.Address != "" {
			log.Lvl2("Fetching final statement")
			fs, err := client.FetchFinal(cfg.Address, final.Desc.Hash())
			if err != nil {
				return err
			}
			if !fs.Merged {
				log.Fatal("Global party is not merged")
			}
			if len(fs.Signature) <= 0 || fs.Verify() != nil {
				log.Fatal("Fetched final statement is invalid")
			}
			final = fs
		} else {
			log.Fatal("No address of conode to download final statement from")
		}

	}
	party := &PartyConfig{}
	party.Final = final
	party.Private = priv
	party.Public = cothority.Suite.Point().Mul(priv, nil)
	index := -1
	for i, p := range party.Final.Attendees {
		if p.Equal(party.Public) {
			log.Lvl1("Found public key at index", i)
			index = i
		}
	}
	if index == -1 {
		log.Fatal("Didn't find our public key in the final statement!")
	}
	party.Index = index
	hash := hex.EncodeToString(final.Desc.Hash())
	log.Lvlf2("Final statement hash: %s", hash)
	if !c.Bool("yes") {
		fmt.Printf("Is it correct hash(y/n)")
		for {
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			c := strings.ToLower(string([]byte(input)[0]))
			if c == "n" {
				return nil
			} else if c != "y" {
				fmt.Printf("Please type (y/n)")
			} else {
				break
			}
		}
	}
	cfg.Parties[hash] = party
	cfg.write()
	log.Lvl3("Stored final statement")
	return nil
}

// signs a message + context
func attSign(c *cli.Context) error {
	log.Lvl3("att: sign")
	cfg, _ := getConfigClient(c)
	if c.NArg() < 3 {
		log.Fatal("Please give msg, context and party hash")
	}
	log.Lvl3("hash:", c.Args().Get(2))
	party, err := cfg.getPartybyHash(c.Args().Get(2))
	if err != nil {
		return err
	}

	if party.Index == -1 || party.Private == nil || party.Public == nil ||
		!cothority.Suite.Point().Mul(party.Private, nil).Equal(party.Public) {
		log.Fatal("No public key stored. Please join a party")
	}

	if len(party.Final.Signature) < 0 || party.Final.Verify() != nil {
		log.Fatal("Party is not finalized or signature is not valid")
	}

	msg := []byte(c.Args().First())
	ctx := []byte(c.Args().Get(1))
	Set := anon.Set(party.Final.Attendees)
	sigtag := anon.Sign(cothority.Suite.(anon.Suite), msg,
		Set, ctx, party.Index, party.Private)
	sig := sigtag[:len(sigtag)-service.SIGSIZE/2]
	tag := sigtag[len(sigtag)-service.SIGSIZE/2:]
	log.Lvlf2("\nSignature: %x\nTag: %x", sig, tag)
	return nil
}

// verifies a signature and tag
func attVerify(c *cli.Context) error {
	log.Lvl3("att: verify")
	cfg, _ := getConfigClient(c)
	if c.NArg() < 5 {
		log.Fatal("Please give a msg, context, signature, a tag and party hash")
	}
	party, err := cfg.getPartybyHash(c.Args().Get(4))
	if err != nil {
		return err
	}

	if len(party.Final.Signature) < 0 || party.Final.Verify() != nil {
		return errors.New("Party is not finalized or signature is not valid")
	}

	msg := []byte(c.Args().First())
	ctx := []byte(c.Args().Get(1))
	sig, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return err
	}
	tag, err := hex.DecodeString(c.Args().Get(3))
	if err != nil {
		return err
	}
	sigtag := append(sig, tag...)
	ctag, err := anon.Verify(cothority.Suite.(anon.Suite), msg,
		anon.Set(party.Final.Attendees), ctx, sigtag)
	if err != nil {
		return err
	}
	if !bytes.Equal(tag, ctag) {
		log.Fatalf("Tag and calculated tag are not equal:\n%x - %x", tag, ctag)
	}
	log.Lvl3("Successfully verified signature and tag")
	return nil
}

func authStore(c *cli.Context) error {
	log.Lvl3("auth: store")
	cfg, _ := getConfigClient(c)
	if c.NArg() < 1 {
		log.Fatal("Please give a final.toml")
	}

	finalName := c.Args().First()
	buf, err := ioutil.ReadFile(finalName)
	if err != nil {
		return err
	}
	final, err := service.NewFinalStatementFromToml(buf)
	if err != nil {
		return err
	}

	if len(final.Signature) <= 0 || final.Verify() != nil {
		log.Fatal("The local config is not finished yet")
	}

	if len(final.Desc.Parties) > 0 && !final.Merged {
		log.Fatal("The local party is not merged yet")
	}
	party := &PartyConfig{}
	party.Final = final
	hash := hex.EncodeToString(final.Desc.Hash())
	cfg.Parties[hash] = party
	cfg.write()
	log.Lvlf1("Stored final statement, hash: %s", hash)
	return nil
}

// omniStore creates a new PopParty instance together with a darc that is
// allowed to invoke methods on it.
func omniStore(c *cli.Context) error {
	if c.NArg() < 2 || c.NArg() > 3 {
		return errors.New("please give: omniledger.cfg key-xxx.cfg [final-id]")
	}

	// Load the omniledger configuration
	buf, err := ioutil.ReadFile(c.Args().First())
	if err != nil {
		return err
	}
	_, msgCfg, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return err
	}
	cfg := msgCfg.(*ol.Config)

	// Load the signer
	buf, err = ioutil.ReadFile(c.Args().Get(1))
	if err != nil {
		return err
	}
	_, msgSigner, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return err
	}
	signer := msgSigner.(*darc.Signer)

	log.Info("Finished loading configuration and signer.")

	// And get the final statement - either we get a hint of the first bytes of
	// the final statement on the command line, or we will display all final
	// statements to the user.
	var finalId []byte
	if c.NArg() == 3 {
		finalId, err = hex.DecodeString(c.Args().Get(2))
		if err != nil {
			return err
		}
	}

	// Fetch all final statements from all nodes in the roster of omniledger.
	// This supposes that there is at least an overlap of the servers who were
	// involved in the pop-party and the servers holding omniledger.
	finalStatements := map[string]*service.FinalStatement{}
	for _, s := range cfg.Roster.List {
		log.Info("Looking up final statements on host", s)
		fs, err := service.NewClient().GetFinalStatements(s.Address)
		if err != nil {
			log.Error("Error while fetching final statement:", err, "going on")
			continue
		}
		for k, v := range fs {
			finalStatements[k] = v
		}
	}

	if len(finalId) > 0 {
		// Filter using finalId
		for k := range finalStatements {
			if bytes.Compare([]byte(k[0:len(finalId)]), finalId) == 0 {
				delete(finalStatements, k)
			}
		}
	}

	if len(finalStatements) == 0 {
		if len(finalId) > 0 {
			return errors.New("didn't find a final statement starting with the bytes given. Try no search")
		}
		return errors.New("none of the conodes of omniledger has any party stored")
	}

	if len(finalStatements) > 1 {
		for k, v := range finalStatements {
			log.Infof("%x: '%s' in '%s' at '%s'", k, v.Desc.Name, v.Desc.Location, v.Desc.DateTime)
		}
		return errors.New("found more than one final statement - please chose by giving the first (or more) bytes")
	}

	var finalStatement *service.FinalStatement
	for _, v := range finalStatements {
		finalStatement = v
	}
	fsString := fmt.Sprintf("'%s' in '%s' at '%s'", finalStatement.Desc.Name,
		finalStatement.Desc.Location, finalStatement.Desc.DateTime)

	log.Info("Contacting nodes to get the public keys of the organizers")
	var identities []darc.Identity
	for _, s := range cfg.Roster.List {
		log.Info("Contacting", s)
		link, err := service.NewClient().GetLink(s.Address)
		if err != nil {
			return errors.New("need all public keys of all organizers: " + err.Error())
		}
		identities = append(identities, darc.NewIdentityEd25519(link))
	}

	log.Info("Creating darc for the organizers")
	rules := darc.InitRules(identities, identities)
	var exprSlice []string
	for _, id := range identities {
		exprSlice = append(exprSlice, id.String())
	}
	// The master signer has the right to create a new party.
	rules.AddRule("spawn:popParty", expression.Expr(signer.Identity().String()))
	// We allow any of the organizers to update the final statement. The contract
	// will make sure that it is correctly signed.
	rules.AddRule("invoke:Finalize", expression.Expr(strings.Join(exprSlice, " | ")))
	orgDarc := darc.NewDarc(rules, []byte("For party "+fsString))
	orgDarcBuf, err := orgDarc.ToProto()
	if err != nil {
		return err
	}
	inst := ol.Instruction{
		InstanceID: ol.NewInstanceID(cfg.GenesisDarc.GetBaseID()),
		Nonce:      ol.Nonce{},
		Index:      0,
		Length:     1,
		Spawn: &ol.Spawn{
			ContractID: ol.ContractDarcID,
			Args: ol.Arguments{{
				Name:  "darc",
				Value: orgDarcBuf,
			}},
		},
	}
	err = inst.SignBy(cfg.GenesisDarc.GetBaseID(), *signer)
	if err != nil {
		return err
	}
	ct := ol.ClientTransaction{
		Instructions: ol.Instructions{inst},
	}
	log.Info("Contacting omniledger to store the new darc")
	olc := ol.NewClientConfig(*cfg)
	_, err = olc.AddTransactionAndWait(ct, 10)
	if err != nil {
		return err
	}

	partyConfigBuf, err := protobuf.Encode(finalStatement)
	if err != nil {
		return err
	}
	inst = ol.Instruction{
		InstanceID: ol.NewInstanceID(orgDarc.GetBaseID()),
		Nonce:      ol.Nonce{},
		Index:      0,
		Length:     1,
		Spawn: &ol.Spawn{
			ContractID: service.ContractPopParty,
			Args: ol.Arguments{{
				Name:  "PartyConfig",
				Value: partyConfigBuf,
			}},
		},
	}
	err = inst.SignBy(orgDarc.GetBaseID(), *signer)
	if err != nil {
		return err
	}
	ct = ol.ClientTransaction{
		Instructions: ol.Instructions{inst},
	}
	log.Info("Contacting omniledger to spawn the new party")
	_, err = olc.AddTransactionAndWait(ct, 10)
	if err != nil {
		return err
	}

	return nil
}

// getConfigClient returns the configuration and a client-structure.
func getConfigClient(c *cli.Context) (*Config, *service.Client) {
	cfg, err := newConfig(path.Join(c.GlobalString("config"), "config.bin"))
	log.ErrFatal(err)
	return cfg, service.NewClient()
}

// newConfig tries to read the config and returns an organizer-
// config if it doesn't find anything.
func newConfig(fileConfig string) (*Config, error) {
	name := app.TildeToHome(fileConfig)
	if _, err := os.Stat(name); err != nil {
		kp := key.NewKeyPair(cothority.Suite)
		return &Config{
			OrgPublic:  kp.Public,
			OrgPrivate: kp.Private,
			Parties:    make(map[string]*PartyConfig),
			name:       name,
		}, nil
	}
	buf, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("couldn't read %s: %s - please remove it",
			name, err)
	}
	_, msg, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return nil, fmt.Errorf("error while reading file %s: %s",
			name, err)
	}
	cfg, ok := msg.(*Config)
	if !ok {
		log.Fatal("Wrong data-structure in file", name)
	}
	if cfg.Parties == nil {
		cfg.Parties = make(map[string]*PartyConfig)
	}
	cfg.name = name
	return cfg, nil
}

// write saves the config to the given file.
func (cfg *Config) write() {
	buf, err := network.Marshal(cfg)
	log.ErrFatal(err)
	log.ErrFatal(ioutil.WriteFile(cfg.name, buf, 0660))
}

func (cfg *Config) getPartybyHash(hash string) (*PartyConfig, error) {
	if val, ok := cfg.Parties[hash]; ok {
		return val, nil
	}
	return nil, errors.New("No such party")
}

// readGroup fetches group definition file.
func readGroup(name string) *onet.Roster {
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	g, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(g.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s", name)
	}
	return g.Roster
}

// PopDescGroupToml represents serializable party description
type PopDescGroupToml struct {
	Name     string
	DateTime string
	Location string
	Servers  []*app.ServerToml `toml:"servers"`
}

func decodePopDesc(buf string, desc *service.PopDesc) error {
	descGroup := &PopDescGroupToml{}
	_, err := toml.Decode(buf, descGroup)
	if err != nil {
		return err
	}
	desc.Name = descGroup.Name
	desc.DateTime = descGroup.DateTime
	desc.Location = descGroup.Location
	entities := make([]*network.ServerIdentity, len(descGroup.Servers))
	for i, s := range descGroup.Servers {
		en, err := toServerIdentity(s, cothority.Suite)
		if err != nil {
			return err
		}
		entities[i] = en
	}
	desc.Roster = onet.NewRoster(entities)
	return nil
}

type shortDescGroupToml struct {
	Location string
	Servers  []*app.ServerToml `toml:"servers"`
}

// decode config of several groups into array of rosters
func decodeGroups(buf string) ([]*service.ShortDesc, error) {
	decodedGroups := make(map[string][]shortDescGroupToml)
	_, err := toml.Decode(buf, &decodedGroups)
	if err != nil {
		return []*service.ShortDesc{}, err
	}
	groups := decodedGroups["parties"]
	descs := []*service.ShortDesc{}
	for _, descGroup := range groups {
		desc := &service.ShortDesc{}
		desc.Location = descGroup.Location
		entities := make([]*network.ServerIdentity, len(descGroup.Servers))
		for j, s := range descGroup.Servers {
			en, err := toServerIdentity(s, cothority.Suite)
			if err != nil {
				return []*service.ShortDesc{}, err
			}
			entities[j] = en
		}
		desc.Roster = onet.NewRoster(entities)
		descs = append(descs, desc)
	}
	return descs, nil
}

// TODO: Needs to be public in app package!!!
// toServerIdentity converts this ServerToml struct to a ServerIdentity.
func toServerIdentity(s *app.ServerToml, suite kyber.Group) (*network.ServerIdentity, error) {
	public, err := encoding.StringHexToPoint(suite, s.Public)
	if err != nil {
		return nil, err
	}
	si := network.NewServerIdentity(public, s.Address)
	si.Description = s.Description
	return si, nil
}
