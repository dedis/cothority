// This is a command line interface for communicating with the evoting service.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
)

var (
	argRoster = flag.String("roster", "", "path to roster toml file")
	argKey    = flag.String("key", "", "client-side public key")
	argAdmins = flag.String("admins", "", "list of admin users")
	argPin    = flag.String("pin", "", "service pin")
	argID     = flag.String("id", "", "ID of the master chain to modify (optional)")
	argShow   = flag.Bool("show", false, "Show the current Master config")
)

func main() {
	flag.Parse()

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

	key, err := parseKey(*argKey)
	if err != nil {
		log.Fatal("cannot parse key: ", err)
	}

	admins, err := parseAdmins(*argAdmins)
	if err != nil {
		log.Fatal("cannot parse admins: ", err)
	}

	request := &evoting.Link{Pin: *argPin, Roster: roster, Key: key, Admins: admins}
	if *argID != "" {
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		var sbid skipchain.SkipBlockID = id
		request.ID = &sbid
	}
	reply := &evoting.LinkReply{}

	client := onet.NewClient(cothority.Suite, evoting.ServiceName)
	if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
		log.Fatal("link request: ", err)
	}

	// Do not change this: the load-testing system counts on it.
	log.Info("Master ID:", reply.ID)

	// This is more useful for putting back into the -id argument to evolve master.
	log.Infof("Master ID in hex: %x", reply.ID)
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
