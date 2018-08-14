// This is a command line interface for communicating with the evoting service.
package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/network"
)

var (
	argRoster     = flag.String("roster", "", "path to roster toml file")
	argAdmins     = flag.String("admins", "", "list of admin users")
	argPin        = flag.String("pin", "", "service pin")
	argKey        = flag.String("key", "", "public key of authentication server")
	argID         = flag.String("id", "", "ID of the master chain to modify (optional)")
	argUser       = flag.Int("user", 0, "The SCIPER of an existing admin of this chain")
	argSig        = flag.String("sig", "", "A signature proving that you can login to Tequila with the given SCIPER.")
	argShow       = flag.Bool("show", false, "Show the current Master config")
	argDump       = flag.String("dump", "", "dump the election events from the given file")
	argDumpMaster = flag.Bool("dumpMaster", false, "dump the master chain")
	argFrom       = flag.Int("from", 0, "Dump starting at this block index")
)

func main() {
	flag.Parse()
	if *argDump != "" {
		err := dump(*argDump)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if *argRoster == "" {
		log.Fatal("Roster argument (-roster) is required for create, update, or show.")
	}
	roster, err := parseRoster(*argRoster)
	if err != nil {
		log.Fatal("cannot parse roster: ", err)
	}

	if *argShow {
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		request := &evoting.GetElections{Master: id}
		reply := &evoting.GetElectionsReply{}
		client := onet.NewClient(cothority.Suite, evoting.ServiceName)
		if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
			log.Fatal("get elections request: ", err)
		}
		m := reply.Master
		fmt.Printf(" Admins: %v\n", m.Admins)
		fmt.Printf(" Roster: %v\n", m.Roster.List)
		fmt.Printf("    Key: %v\n", m.Key)
		return
	}

	if *argAdmins == "" {
		log.Fatal("Admin list (-admins) must have at least one id.")
	}

	admins, err := parseAdmins(*argAdmins)
	if err != nil {
		log.Fatal("cannot parse admins: ", err)
	}

	if *argPin == "" {
		log.Fatal("pin must be set for create and update operations.")
	}

	var pub kyber.Point
	if *argKey != "" {
		pub, err = parseKey(*argKey)
		if err != nil {
			log.Fatal("cannot parse key: ", err)
		}
	} else {
		kp := key.NewKeyPair(cothority.Suite)
		log.Printf("Auth-server private key: %v", kp.Private)
		pub = kp.Public
	}

	request := &evoting.Link{Pin: *argPin, Roster: roster, Key: pub, Admins: admins}
	if *argID != "" {
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}

		if *argSig == "" {
			log.Fatal("-sig required when updating")
		}
		sig, err := hex.DecodeString(*argSig)
		if err != nil {
			log.Fatal("sig decode", err)
		}
		var sbid skipchain.SkipBlockID = id
		request.ID = &sbid
		var u = uint32(*argUser)
		request.User = &u
		request.Signature = &sig
	}
	reply := &evoting.LinkReply{}

	client := onet.NewClient(cothority.Suite, evoting.ServiceName)
	if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
		log.Fatal("link request: ", err)
	}

	log.Printf("Auth-server public  key: %v", pub)
	log.Printf("Master ID: %x", reply.ID)
}

// parseRoster reads a Dedis group toml file a converts it to a cothority roster.
func parseRoster(path string) (*onet.Roster, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	group, err := app.ReadGroupDescToml(file)
	if err != nil {
		return nil, err
	}
	return group.Roster, nil
}

// parseAdmins converts a string of comma-separated sciper numbers in
// the format sciper1,sciper2,sciper3 to a list of integers.
func parseAdmins(scipers string) ([]uint32, error) {
	if scipers == "" {
		return nil, nil
	}

	admins := make([]uint32, 0)
	for _, admin := range strings.Split(scipers, ",") {
		sciper, err := strconv.Atoi(admin)
		if err != nil {
			return nil, err
		}
		admins = append(admins, uint32(sciper))
	}
	return admins, nil
}

// parseKey unmarshals a Ed25519 point given in hexadecimal form.
func parseKey(key string) (kyber.Point, error) {
	b, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	point := cothority.Suite.Point()
	if err = point.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return point, nil
}

// To use this:
//
// 1. dump the master chain in order to find out which election chains there are to dump.
//     ./evoting-admin -dump f4fc9718c096f9bfc76bd23a7056a4102770e4450a8f00f3c363bce804e61509.db -dumpMaster
// 2. for each election chain that you are interested in, dump that thain:
//     ./evoting-admin -dump f4fc9718c096f9bfc76bd23a7056a4102770e4450a8f00f3c363bce804e61509.db -id 0c4901f078637bf2b5ba0f903e6ee6279a6032f36f9f8d8819e7782ada3ed480
//
// Use the -from argument to start dumping from a certain block index.
//
// Dump format is one line per block, tab separated, first column tells the block type, other
// columns depend on the block type.
func dump(path string) error {
	db, err := openDb(path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Dump master chain.
	if *argDumpMaster {
		// The skipchain ID of the master chain from the production e-voting run for 2018.
		// I found it via: console.log(localStorage.master)
		var key []byte
		for _, x := range strings.Split("121,29,215,113,94,96,73,229,180,105,187,51,107,148,27,123,97,14,233,180,237,134,32,254,51,151,5,232,159,93,176,69", ",") {
			x2, _ := strconv.Atoi(x)
			key = append(key, byte(x2))
		}
		id := skipchain.SkipBlockID(key)

		for !id.IsNull() {
			bl, err := loadBlock(db, []byte(id))
			if err != nil {
				return err
			}
			tx := lib.UnmarshalTransaction(bl.Data)
			switch {
			case bl.Index == 0:
				fmt.Printf("genesis\n")
			case tx.Link != nil:
				bl2, err := loadBlock(db, tx.Link.ID)
				if err != nil {
					return err
				}
				bl2, err = loadBlock(db, bl2.ForwardLink[0].To)
				if err != nil {
					return err
				}
				tx2 := lib.UnmarshalTransaction(bl2.Data)
				if tx == nil {
					panic("should not happen")
				} else {
					fmt.Printf("link	%x	%v	%v\n", tx.Link.ID, tx2.Election.Name["en"], len(tx2.Election.Users))
				}
			case tx.Master != nil:
				fmt.Printf("master	%x	%v\n", tx.Master.Key, joinUsers(tx.Master.Admins))
			default:
				fmt.Println("other")
			}
			if len(bl.ForwardLink) > 0 {
				id = bl.ForwardLink[0].To
			} else {
				id = nil
			}
		}
		return nil
	}

	// Dump election chain.
	if *argID == "" {
		return errors.New("need election chain id")
	}
	id2, err := hex.DecodeString(*argID)
	if err != nil {
		return err
	}
	id := skipchain.SkipBlockID(id2)

	for !id.IsNull() {
		bl, err := loadBlock(db, []byte(id))
		if err != nil {
			return err
		}
		if bl == nil {
			return errors.New("cannot lookup election")
		}

		if bl.Index >= *argFrom {
			tx := lib.UnmarshalTransaction(bl.Data)
			if tx == nil {
				log.Fatalf("Block %d: could not find an evoting tx", bl.Index)
			} else {
				switch {
				case len(bl.Data) == 0:
					fmt.Printf("genesis\n")
				case tx.Election != nil:
					fmt.Printf("election	\"%v\"	voters	%v\n", tx.Election.Name, joinUsers(tx.Election.Users))
				case tx.Ballot != nil:
					fmt.Printf("ballot	%v	%v	%v\n", tx.Ballot.User, tx.Ballot.Alpha, tx.Ballot.Beta)
				case tx.Mix != nil:
					fmt.Printf("mix	%v\n", tx.Mix.NodeID)
				case tx.Partial != nil:
					fmt.Printf("partial	%v\n", tx.Partial.NodeID)
				default:
					fmt.Printf("other\n")
				}
			}
		}

		if len(bl.ForwardLink) > 0 {
			id = bl.ForwardLink[0].To
		} else {
			id = nil
		}
	}

	return nil
}

func joinUsers(u []uint32) string {
	var out []string
	for _, x := range u {
		out = append(out, fmt.Sprintf("%d", x))
	}
	return strings.Join(out, ",")
}

// openDb opens a database at `path`. It creates the database if it does not exist.
// The caller must ensure that all parent directories exist.
func openDb(path string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func loadBlock(db *bolt.DB, key []byte) (*skipchain.SkipBlock, error) {
	bucket := []byte("Skipchain_skipblocks")
	var buf []byte
	db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucket).Get(key)
		if v == nil {
			return nil
		}

		buf = make([]byte, len(v))
		copy(buf, v)
		return nil
	})

	if buf == nil {
		return nil, nil
	}

	_, ret, err := network.Unmarshal(buf, cothority.Suite)
	if bl, ok := ret.(*skipchain.SkipBlock); !ok {
		return nil, fmt.Errorf("data is type %T", ret)
	} else {
		return bl, err
	}
}
