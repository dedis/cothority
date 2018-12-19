// This is the command line interface to communicate with the pop service.
//
// More details can be found here -
// https://github.com/dedis/cothority/blob/master/pop/README.md.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/blscosi/blscosi/check"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/bcadmin/lib"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	ph "github.com/dedis/cothority/personhood"
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
	"github.com/dedis/protobuf"
	cli "gopkg.in/urfave/cli.v1"
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
	// Map of Final statements or configutations of the parties.
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
		commandBC,
		commandOrg,
		commandAttendee,
		commandAuth,
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action: func(c *cli.Context) error {
				return check.CothorityCheck(c.Args().First(), false)
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
	err := cfg.write()
	if err != nil {
		return errors.New("couldn't write configuration file: " + err.Error())
	}
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
			if desc.Roster.IsRotation(party.Roster) {
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
	return cfg.write()
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
		grp, err := (&app.Group{Roster: pd.Roster}).Toml(cothority.Suite)
		if err != nil {
			return err
		}
		p.Servers = grp.Servers

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
	return cfg.write()
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
	err = cfg.write()
	if err != nil {
		return errors.New("couldn't write configuration file: " + err.Error())
	}
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
		err = cfg.write()
		if err != nil {
			return errors.New("couldn't write configuration file: " + err.Error())
		}
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
	err = cfg.write()
	if err != nil {
		return errors.New("couldn't write configuration file: " + err.Error())
	}
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
	err = cfg.write()
	if err != nil {
		return errors.New("couldn't write configuration file: " + err.Error())
	}
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
	sig := sigtag[:len(sigtag)-service.SignatureSize/2]
	tag := sigtag[len(sigtag)-service.SignatureSize/2:]
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
	err = cfg.write()
	if err != nil {
		return errors.New("couldn't write configuration file: " + err.Error())
	}
	log.Lvlf1("Stored final statement, hash: %s", hash)
	return nil
}

// bcStore creates a new PopParty instance together with a darc that is
// allowed to invoke methods on it.
func bcStore(c *cli.Context) error {
	if c.NArg() < 2 || c.NArg() > 3 {
		return errors.New("please give: bc.cfg key-xxx.cfg [final-id]")
	}

	// Load the configuration
	cfg, _, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}

	// Load the signer
	signer, err := lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		return err
	}

	log.Info("Finished loading configuration and signer.")

	// And get the final statement - either we get a hint of the first bytes of
	// the final statement on the command line, or we will display all final
	// statements to the user.
	var finalID []byte
	if c.NArg() == 3 {
		finalID, err = hex.DecodeString(c.Args().Get(2))
		if err != nil {
			return err
		}
	}

	// Fetch all final statements from all nodes in the roster.
	// This supposes that there is at least an overlap of the servers who were
	// involved in the pop-party and the servers holding the ledger.
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

	if len(finalID) > 0 {
		// Filter using finalId
		for k := range finalStatements {
			if bytes.Compare([]byte(k[0:len(finalID)]), finalID) != 0 {
				delete(finalStatements, k)
			}
		}
	}

	if len(finalStatements) == 0 {
		if len(finalID) > 0 {
			return errors.New("didn't find a final statement starting with the bytes given. Try no search")
		}
		return errors.New("none of the conodes has any party stored")
	}

	if len(finalStatements) > 1 {
		for k, v := range finalStatements {
			log.Infof("%x: '%s' in '%s' at '%s'", k, v.Desc.Name, v.Desc.Location, v.Desc.DateTime)
		}
		return errors.New("found more than one proposed configuration - please chose by giving the first (or more) bytes")
	}

	var finalStatement *service.FinalStatement
	for k, v := range finalStatements {
		finalID = []byte(k)
		finalStatement = v
	}
	if err != nil {
		return errors.New("error while creating hash of proposed configuration: " + err.Error())
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
	identities = append(identities, signer.Identity())

	log.Info("Creating byzcoin client and getting signer counters")
	bccl := byzcoin.NewClient(cfg.ByzCoinID, cfg.Roster)
	signerCtrs, err := bccl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return err
	}
	if len(signerCtrs.Counters) != 1 {
		return errors.New("incorrect signer counter length")
	}

	log.Info("Creating darc for the organizers")
	rules := darc.InitRules(identities, identities)
	var exprSlice []string
	for _, id := range identities {
		exprSlice = append(exprSlice, id.String())
	}
	// The master signer has the right to create a new party.
	rules.AddRule("spawn:popParty", expression.Expr(signer.Identity().String()))
	// We allow any of the organizers to update the proposed configuration. The contract
	// will make sure that it is correctly signed.
	rules.AddRule("invoke:Finalize", expression.Expr(strings.Join(exprSlice, " | ")))
	orgDarc := darc.NewDarc(rules, []byte("For party "+fsString))
	orgDarcBuf, err := orgDarc.ToProto()
	if err != nil {
		return err
	}
	inst := byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(cfg.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDarcID,
			Args: byzcoin.Arguments{{
				Name:  "darc",
				Value: orgDarcBuf,
			}},
		},
		SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
	}
	ct := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{inst},
	}
	err = ct.SignWith(*signer)
	if err != nil {
		return err
	}
	log.Info("Storing the new darc on byzcoin")
	_, err = bccl.AddTransactionAndWait(ct, 10)
	if err != nil {
		return err
	}

	partyConfigBuf, err := protobuf.Encode(finalStatement)
	if err != nil {
		return err
	}
	inst = byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(orgDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: service.ContractPopParty,
			Args: byzcoin.Arguments{{
				Name:  "FinalStatement",
				Value: partyConfigBuf,
			}},
		},
		SignerCounter: []uint64{signerCtrs.Counters[0] + 2},
	}
	ct = byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{inst},
	}
	err = ct.SignWith(*signer)
	if err != nil {
		return err
	}
	fsID, err := finalStatement.Hash()
	if err != nil {
		return err
	}
	log.Infof("Contacting ByzCoin to spawn the new party with id %x", fsID)
	_, err = bccl.AddTransactionAndWait(ct, 10)
	if err != nil {
		return err
	}

	log.Info("Storing InstanceID in pop-service")
	iid := ct.Instructions[0].DeriveID("")
	for _, c := range cfg.Roster.List {
		err = service.NewClient().StoreInstanceID(c.Address, finalID, iid, orgDarc.GetBaseID())
		if err != nil {
			log.Error("couldn't store instanceID: " + err.Error())
		}
	}

	log.Info("New party spawned with instance-id:", iid)
	return nil
}

// bcFinalize stores a final statement of a party
func bcFinalize(c *cli.Context) error {
	if c.NArg() != 3 {
		return errors.New("please give: bc.cfg key-xxx.cfg partyID")
	}

	// Load the configuration
	cfg, ocl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}

	// Load the signer
	signer, err := lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		return err
	}

	// Load signer counter
	signerCtrs, err := ocl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return err
	}

	// Get the party-id
	partyID, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return errors.New("couldn't parse partyID: " + err.Error())
	}

	log.Info("Fetching final statement from conode", cfg.Roster.List[0])
	cl := service.NewClient()
	fsMap, err := cl.GetFinalStatements(cfg.Roster.List[0].Address)
	if err != nil {
		return errors.New("error while fetching final statement: " + err.Error())
	}
	fs, ok := fsMap[string(partyID)]
	if !ok {
		for k, fs := range fsMap {
			log.Infof("partyID: %x - %v", k, fs.Desc)
		}
		return errors.New("didn't find final statement")
	}
	if fs.Signature == nil || len(fs.Signature) == 0 || len(fs.Attendees) == 0 {
		log.Infof("%+v", fs)
		return errors.New("proposed configuration not finalized")
	}
	fsBuf, err := protobuf.Encode(fs)
	if err != nil {
		return errors.New("couldn't encode final statement: " + err.Error())
	}

	partyInstance, _, err := cl.GetInstanceID(cfg.Roster.List[0].Address, partyID)
	if err != nil {
		return errors.New("couldn't get instanceID: " + err.Error())
	}
	if partyInstance.Equal(byzcoin.InstanceID{}) {
		return errors.New("no instanceID stored")
	}

	sigBuf, err := signer.Ed25519.Point.MarshalBinary()
	if err != nil {
		return errors.New("Couldn't get point: " + err.Error())
	}

	log.Info("Sending finalize-instruction")
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{byzcoin.Instruction{
			InstanceID: partyInstance,
			Invoke: &byzcoin.Invoke{
				Command: "Finalize",
				Args: byzcoin.Arguments{
					byzcoin.Argument{
						Name:  "FinalStatement",
						Value: fsBuf,
					},
					{
						Name:  "Service",
						Value: sigBuf,
					}},
			},
			SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		}},
	}
	err = ctx.SignWith(*signer)
	if err != nil {
		return errors.New("couldn't sign instruction: " + err.Error())
	}

	_, err = ocl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("error while sending transaction: " + err.Error())
	}

	iid := sha256.New()
	iid.Write(ctx.Instructions[0].InstanceID.Slice())
	pubBuf, err := signer.Ed25519.Point.MarshalBinary()
	if err != nil {
		return errors.New("couldn't marshal public key: " + err.Error())
	}
	iid.Write(pubBuf)
	p, err := ocl.GetProof(iid.Sum(nil))
	if err != nil {
		return errors.New("couldn't calculate service coin address: " + err.Error())
	}
	_, _, _, dID, err := p.Proof.KeyValue()
	if err != nil {
		return errors.New("proof was invalid: " + err.Error())
	}
	p, err = ocl.GetProof(dID)
	if err != nil {
		return errors.New("couldn't get proof for service-darc: " + err.Error())
	}
	_, v0, _, _, err := p.Proof.KeyValue()
	if err != nil {
		return errors.New("service-darc proof is invalid: " + err.Error())
	}
	serviceDarc, err := darc.NewFromProtobuf(v0)
	if err != nil {
		return errors.New("got invalid service-darc: " + err.Error())
	}

	err = ph.NewClient().LinkPoP(fs.Desc.Roster.List[0], ph.Party{
		ByzCoinID:      cfg.ByzCoinID,
		FinalStatement: *fs,
		InstanceID:     partyInstance,
		Darc:           *serviceDarc,
		Signer:         *signer,
	})
	if err != nil {
		return errors.New("couldn't store in personhood service: " + err.Error())
	}

	log.Infof("Finalized party %x with instance-id %x", partyID, partyInstance)
	return nil
}

// bcCoinShow returns the number of coins in the account of the user.
func bcCoinShow(c *cli.Context) error {
	if c.NArg() != 3 {
		return errors.New("please give: bc.cfg partyID (public-key | accountID)")
	}

	// Load the configuration
	_, ocl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}

	partyInstanceID, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't parse partyID: " + err.Error())
	}

	// Check if we got the public-key or the accountID. First suppose it's the accountID
	// and verify if that instance exists.
	accountID, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return errors.New("couldn't parse public-key or accountID: " + err.Error())
	}

	accountProof, err := ocl.GetProof(accountID)
	if err != nil {
		return errors.New("couldn't get proof for account: " + err.Error())
	}
	if !accountProof.Proof.InclusionProof.Match(accountID) {
		// This account doesn't exist - try with the account, supposing we got a
		// public key.
		log.Info("Interpreting argument as public key.")
		h := sha256.New()
		h.Write(partyInstanceID)
		h.Write(accountID)
		accountID = h.Sum(nil)
		accountProof, err = ocl.GetProof(accountID)
		if err != nil {
			return errors.New("couldn't get proof for account: " + err.Error())
		}
		if !accountProof.Proof.InclusionProof.Match(accountID) {
			return errors.New("didn't find this account - neither as accountID, nor as public key")
		}
	} else {
		log.Info("Interpreting argument as account ID")
	}

	_, v0, _, _, err := accountProof.Proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value from proof: " + err.Error())
	}
	ci := byzcoin.Coin{}
	err = protobuf.Decode(v0, &ci)
	if err != nil {
		return errors.New("couldn't unmarshal coin balance: " + err.Error())
	}
	log.Info("Coin balance is: ", ci.Value)
	return nil
}

func bcCoinTransfer(c *cli.Context) error {
	if c.NArg() != 5 {
		return errors.New("please give: bc.cfg partyID source_private_key dst_public_key amount")
	}

	// Load the configuration
	_, ocl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}

	partyID, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't parse partyID: " + err.Error())
	}

	// Get the private key for the source
	srcPriv, err := encoding.StringHexToScalar(cothority.Suite, c.Args().Get(2))
	if err != nil {
		return errors.New("couldn't parse private key: " + err.Error())
	}
	srcPub := cothority.Suite.Point().Mul(srcPriv, nil)
	srcSigner := darc.NewSignerEd25519(srcPub, srcPriv)
	srcAddrHash := sha256.New()
	srcAddrHash.Write(partyID)
	srcPubBuf, err := srcPub.MarshalBinary()
	if err != nil {
		return errors.New("couldn't marshal public key: " + err.Error())
	}
	srcAddrHash.Write(srcPubBuf)
	srcAddr := srcAddrHash.Sum(nil)

	dstPub, err := encoding.StringHexToPoint(cothority.Suite, c.Args().Get(3))
	if err != nil {
		return errors.New("couldn't parse public key: " + err.Error())
	}
	dstAddrHash := sha256.New()
	dstAddrHash.Write(partyID)
	dstPubBuf, err := dstPub.MarshalBinary()
	if err != nil {
		return errors.New("couldn't marshal public key: " + err.Error())
	}
	dstAddrHash.Write(dstPubBuf)
	dstAddr := dstAddrHash.Sum(nil)

	amount, err := strconv.ParseUint(c.Args().Get(4), 10, 64)
	if err != nil {
		return errors.New("couldn't get amount")
	}

	log.Info("Getting account of source")
	srcInstanceProof, err := ocl.GetProof(srcAddr)
	if err != nil {
		return errors.New("couldn't get source instance: " + err.Error())
	}
	if !srcInstanceProof.Proof.InclusionProof.Match(srcAddr) {
		return errors.New("source instance doesn't exist")
	}

	log.Info("Getting darc for source account")
	_, _, _, _, err = srcInstanceProof.Proof.KeyValue()
	if err != nil {
		return errors.New("cannot get proof for source instance: " + err.Error())
	}

	log.Info("Getting signer counters")
	signerCtrs, err := ocl.GetSignerCounters(srcSigner.Identity().String())
	if err != nil {
		return errors.New("couldn't get signer counter: " + err.Error())
	}
	if len(signerCtrs.Counters) != 1 {
		return errors.New("incorrect signer counter length")
	}

	log.Info("Transferring coins")
	amountBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBuf, amount)
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(srcAddr),
			Invoke: &byzcoin.Invoke{
				Command: "transfer",
				Args: byzcoin.Arguments{{
					Name:  "coins",
					Value: amountBuf,
				},
					{
						Name:  "destination",
						Value: dstAddr,
					}},
			},
			SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		}},
	}
	err = ctx.SignWith(srcSigner)
	if err != nil {
		return errors.New("couldn't sign transaction: " + err.Error())
	}

	_, err = ocl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("couldn't add transaction: " + err.Error())
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
func (cfg *Config) write() error {
	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(cfg.name), os.ModePerm)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(cfg.name, buf, 0660)
	if err != nil {
		return err
	}
	return nil
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
		en, err := s.ToServerIdentity()
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
			en, err := s.ToServerIdentity()
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
