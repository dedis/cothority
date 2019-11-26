// This is a command line interface for communicating with the evoting service.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
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
	argDumpVoters = flag.Bool("dumpvoters", false, "Dump a list of voters for election skipchain specified with -id (ballot de-duplication has already been taken into account, order is preserved)")
)

func main() {
	flag.Parse()

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

	if *argDumpVoters {
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		reply := &evoting.GetBoxReply{}
		client := onet.NewClient(cothority.Suite, evoting.ServiceName)
		if err = client.SendProtobuf(roster.List[0], &evoting.GetBox{ID: id}, reply); err != nil {
			log.Fatal("get box request: ", err)
		}

		for _, b := range reply.Box.Ballots {
			fmt.Println(b.User)
		}
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
		log.Infof("Auth-server private key: %v", kp.Private)
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

	log.Infof("Auth-server public  key: %v", pub)
	log.Infof("Master ID: %x", reply.ID)
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
