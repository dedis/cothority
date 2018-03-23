// This is a command line interface for communicating with the evoting service.
package main

import (
	"encoding/hex"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
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
)

func main() {
	flag.Parse()

	roster, err := parseRoster(*argRoster)
	if err != nil {
		panic(err)
	}

	key, err := parseKey(*argKey)
	if err != nil {
		panic(err)
	}

	admins, err := parseAdmins(*argAdmins)
	if err != nil {
		panic(err)
	}

	var client struct {
		*onet.Client
	}

	request := &evoting.Link{Pin: *argPin, Roster: roster, Key: key, Admins: admins}
	reply := &evoting.LinkReply{}

	client.Client = onet.NewClient(cothority.Suite, evoting.ServiceName)
	if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
		panic(err)
	}

	log.Info("Master ID:", reply.ID)
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
