// This is a command line interface for communicating with the evoting service.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go.dedis.ch/cothority/v3/evoting/lib"
	"os"
	"strconv"
	"strings"
	"time"

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
	argRoster         = flag.String("roster", "", "path to roster toml file")
	argAdmins         = flag.String("admins", "", "list of admin users")
	argPin            = flag.String("pin", "", "service pin")
	argKey            = flag.String("key", "", "public key of authentication server")
	argPrivate        = flag.String("private", "", "private key of authentication server for voting")
	argID             = flag.String("id", "", "ID of the master chain to modify (optional)")
	argUser           = flag.Int("user", 0, "The SCIPER of an existing admin of this chain")
	argSig            = flag.String("sig", "", "A signature proving that you can login to Tequila with the given SCIPER.")
	argShow           = flag.Bool("show", false, "Show the current Master config")
	argDumpVoters     = flag.Bool("dumpvoters", false, "Dump a list of voters for election skipchain specified with -id (ballot de-duplication has already been taken into account, order is preserved)")
	argDumpElection   = flag.Bool("dumpelection", false, "Dump the current election config for the election specified with -id.")
	argJSON           = flag.Bool("json", false, "Dump in json mode.")
	argLoad           = flag.String("load", "", "Load the specified json file to modify the election specified with -id.")
	argVoteCandidates = flag.String("voteCandidates", "", "Coma delimited list of SCIPERs to vote for")
	argDebug          = flag.Int("debug", 0, "Debugging level")
)

func main() {
	flag.Parse()

	if *argDebug > 0 {
		log.SetDebugVisible(*argDebug)
	}

	if *argRoster == "" {
		log.Fatal("Roster argument (-roster) is required for create, update, or show.")
	}
	roster, err := parseRoster(*argRoster)
	if err != nil {
		log.Fatal("cannot parse roster: ", err)
	}
	client := onet.NewClient(cothority.Suite, evoting.ServiceName)
	cl := evoting.NewClient(roster)

	var pub kyber.Point
	var priv kyber.Scalar
	if *argPrivate != "" {
		// If we get the private key, calculate the corresponding public key
		b, err := hex.DecodeString(*argPrivate)
		log.ErrFatal(err, "while parsing private key")

		log.Lvl1("Setting the private key")
		priv = cothority.Suite.Scalar()
		err = priv.UnmarshalBinary(b)
		log.ErrFatal(err, "while unmarshalling private key")
		pub = cothority.Suite.Point().Mul(priv, nil)
	} else if *argKey != "" {
		// If we only get the public key
		pub, err = parseKey(*argKey)
		if err != nil {
			log.Fatal("cannot parse key: ", err)
		}
	} else {
		// If we get no key, create them again
		kp := key.NewKeyPair(cothority.Suite)
		priv = kp.Private
		pub = kp.Public
	}

	if *argShow {
		if *argID == "" {
			log.Fatal("Please give ID of master chain")
		}
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		request := &evoting.GetElections{Master: id}
		if priv != nil && *argUser > 0 {
			request.User = uint32(*argUser)
			request.Signature = lib.GenerateSignature(priv, id, request.User)
		} else {
			fmt.Println("You can give '-private PRIVATE_KEY -user ADMINID' for a list of available elections to this user")
		}
		reply := &evoting.GetElectionsReply{}
		if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
			log.Fatal("get elections request: ", err)
		}
		m := reply.Master
		fmt.Printf(" Admins: %v\n", m.Admins)
		fmt.Printf(" Roster: %v\n", m.Roster.List)
		fmt.Printf("    Key: %v\n", m.Key)
		for _, election := range reply.Elections {
			fmt.Printf("\nElection: %s\n", election.Name)
			fmt.Printf("      ID: %x\n", election.ID)
		}
		return
	}

	if *argDumpVoters {
		if *argID == "" {
			log.Fatal("Please give ID of master chain")
		}
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		reply := &evoting.GetBoxReply{}
		if err = client.SendProtobuf(roster.List[0], &evoting.GetBox{ID: id}, reply); err != nil {
			log.Fatal("get box request: ", err)
		}

		for _, b := range reply.Box.Ballots {
			fmt.Println(b.User)
		}
		return
	}

	if *argDumpElection {
		if *argID == "" {
			log.Fatal("Please give an ID")
		}
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}
		reply := &evoting.GetBoxReply{}
		if err = client.SendProtobuf(roster.List[0], &evoting.GetBox{ID: id}, reply); err != nil {
			log.Fatal("get box request: ", err)
		}

		if *argJSON {
			e := reply.Election
			j := &jsonElection{
				Name:         e.Name,
				Creator:      e.Creator,
				Users:        e.Users,
				Candidates:   e.Candidates,
				MaxChoices:   e.MaxChoices,
				Subtitle:     e.Subtitle,
				MoreInfo:     e.MoreInfo,
				MoreInfoLang: e.MoreInfoLang,
				Start:        time.Unix(e.Start, 0),
				End:          time.Unix(e.End, 0),
				Theme:        e.Theme,
				FooterText:   e.Footer.Text,
				FooterTitle:  e.Footer.ContactTitle,
				FooterPhone:  e.Footer.ContactPhone,
				FooterEmail:  e.Footer.ContactEmail,
			}
			b, err := json.Marshal(j)
			if err != nil {
				log.Fatal(err)
			}
			var out bytes.Buffer
			json.Indent(&out, b, "", "\t")
			fmt.Fprintln(&out)
			out.WriteTo(os.Stdout)
		} else {
			fmt.Println(reply.Election)
			if reply.Election.Stage == lib.Decrypted {
				reconstructReply, err := cl.Reconstruct(id)
				log.ErrFatal(err, "While getting reconstructed votes")
				for i, p := range reconstructReply.Points {
					d, _ := p.Data()
					fmt.Printf("Vote %d with data %x", i, d)
					for _, p := range reconstructReply.AdditionalPoints[i].AdditionalPoints {
						d, _ := p.Data()
						fmt.Printf(",%x", d)
					}
					fmt.Println()
				}
			}
		}
		return
	}

	if *argLoad != "" {
		if *argID == "" {
			log.Fatal("Please give ID of master chain")
		}
		id, err := hex.DecodeString(*argID)
		if err != nil {
			log.Fatal("id decode", err)
		}

		// Look up the election
		reply := &evoting.GetBoxReply{}
		if err = client.SendProtobuf(roster.List[0], &evoting.GetBox{ID: id}, reply); err != nil {
			log.Fatal("get box request:", err)
		}

		// Load the JSON
		f, err := os.Open(*argLoad)
		if err != nil {
			log.Fatal("cannot open json file:", err)
		}
		defer f.Close()
		dec := json.NewDecoder(f)
		j := new(jsonElection)
		err = dec.Decode(j)
		if err != nil {
			log.Fatal("cannot decode json file:", err)
		}

		// Copy the updatable things over
		e := reply.Election
		e.Name = j.Name
		e.Candidates = j.Candidates
		e.MaxChoices = j.MaxChoices
		e.Subtitle = j.Subtitle
		e.MoreInfo = j.MoreInfo
		e.MoreInfoLang = j.MoreInfoLang
		e.Start = j.Start.Unix()
		e.End = j.End.Unix()
		e.Theme = j.Theme
		e.Footer.Text = j.FooterText
		e.Footer.ContactTitle = j.FooterTitle
		e.Footer.ContactPhone = j.FooterPhone
		e.Footer.ContactEmail = j.FooterEmail

		request := &evoting.Open{ID: e.Master, Election: e}

		// Put the auth info onto the request
		if *argSig == "" {
			log.Fatal("-sig required when updating; get it from printing document.cookie in the browser console of a logged-in evoting app")
		}
		s := *argSig
		if strings.HasPrefix(s, "signature=") {
			s = s[10:]
		}
		sig, err := hex.DecodeString(s)
		if err != nil {
			log.Fatal("sig decode", err)
		}
		var u = uint32(*argUser)
		request.User = u
		request.Signature = sig

		reply2 := &evoting.OpenReply{}
		if err = client.SendProtobuf(roster.List[0], request, reply2); err != nil {
			log.Fatal("could not post the election update:", err)
		}
		return
	}

	if len(*argVoteCandidates) > 0 {
		voteUser := uint32(*argUser)
		if len(*argID) != 64 {
			log.Fatal("Need an ID of the election chain in hex form of 32 bytes")
		}
		voteID, err := hex.DecodeString(*argID)
		log.ErrFatal(err, "Wrong ID for election chain")

		log.Lvl1("Getting election information from the chain")
		reply := &evoting.GetBoxReply{}
		if err = client.SendProtobuf(roster.List[0], &evoting.GetBox{ID: voteID}, reply); err != nil {
			log.Fatal("get box request:", err)
		}

		if len(*argVoteCandidates) < 6 {
			log.Fatal("Need at least one candidate to vote for")
		}
		voteCandidatesStrings := strings.Split(*argVoteCandidates, ",")
		voteCandidates := make([]uint32, len(voteCandidatesStrings))
		for i, c := range voteCandidatesStrings {
			cand, err := strconv.Atoi(c)
			log.ErrFatal(err, "Wrong SCIPER ID for candidate")
			voteCandidates[i] = uint32(cand)
		}

		if priv == nil {
			log.Fatal("Need the private key to vote")
		}

		fmt.Printf("Votes are: %+v\n", voteCandidates)

		ballot := lib.CreateBallot(reply.Election.MaxChoices, reply.Election.Key, voteUser, voteCandidates)
		request := &evoting.Cast{
			ID:        voteID,
			User:      voteUser,
			Ballot:    &ballot,
			Signature: lib.GenerateSignature(priv, reply.Election.Master, voteUser),
		}
		replyCast := &evoting.CastReply{}
		log.ErrFatal(client.SendProtobuf(roster.List[0], request, replyCast))
		log.Info("Successfully cast this vote")
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

	if err = client.SendProtobuf(roster.List[0], request, reply); err != nil {
		log.Fatal("link request: ", err)
	}

	log.Infof("Auth-server private key: %v", priv)
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

// A restricted version of Election, for printing/parsing
type jsonElection struct {
	Name    map[string]string // Name of the election. lang-code, value pair
	Creator uint32            // Creator is the election responsible.
	Users   []uint32          // Users is the list of registered voters.

	Candidates   []uint32          // Candidates is the list of candidate scipers.
	MaxChoices   int               // MaxChoices is the max votes in allowed in a ballot.
	Subtitle     map[string]string // Description in string format. lang-code, value pair
	MoreInfo     string            // MoreInfo is the url to AE Website for the given election.
	MoreInfoLang map[string]string
	Start        time.Time // Start denotes the election start unix timestamp
	End          time.Time // End (termination) datetime as unix timestamp.

	Theme       string // Theme denotes the CSS class for selecting background color of card title.
	FooterText  string
	FooterTitle string
	FooterPhone string
	FooterEmail string
}
