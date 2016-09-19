package swupdate

import (
	"crypto/rand"
	"crypto/sha256"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/cothority/services/timestamp"
	"github.com/dedis/crypto/abstract"
)

/*
 * Defines the simulation for the service-template to be run with
 * `cothority/simul`.
 */

func init() {
	sda.SimulationRegister("SwUpRandClient", NewRandClientSimulation)
}

// Simulation only holds the BFTree simulation
type randClientSimulation struct {
	sda.SimulationBFTree
	// How many days between two updates
	Frequency int
	Base      int
	Height    int
	Snapshot  string
	PGPKeys   int
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewRandClientSimulation(config string) (sda.Simulation, error) {
	es := &randClientSimulation{Base: 2, Height: 10}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *randClientSimulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	err = CopyFiles(dir, e.Snapshot)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run is used on the destination machines and runs a number of
// rounds
func (e *randClientSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	service, ok := config.GetService(ServiceName).(*Service)
	if service == nil || !ok {
		log.Fatal("Didn't find service", ServiceName)
	}
	packets := make(map[string]*SwupChain)
	log.Lvl1("Loading releases")
	drs, err := GetReleases(e.Snapshot)
	log.Lvl1("Loading releases - done")
	if err != nil {
		return err
	}
	now := drs[0].Time
	updateFrequency := time.Duration(e.Frequency) * time.Hour * 24
	log.Lvl1("Frequency is", updateFrequency)

	// latests block known by the client for all packages
	latest := make(map[string]skipchain.SkipBlockID)
	for _, dr := range drs {
		pol := dr.Policy
		log.Lvl1("Building", pol.Name, pol.Version)
		// Verify if it's the first version of that packet
		sc, knownPacket := packets[pol.Name]
		release := &Release{pol, dr.Signatures, false}
		round := monitor.NewTimeMeasure("full_" + pol.Name)
		if knownPacket {
			// Append to skipchain, will build
			upr, _ := service.UpdatePackage(nil,
				&UpdatePackage{sc, release})
			packets[pol.Name] = upr.(*UpdatePackageRet).SwupChain
		} else {
			// Create the skipchain, will build
			cp, err := service.CreatePackage(nil,
				&CreatePackage{
					Roster:  config.Roster,
					Base:    e.Base,
					Height:  e.Height,
					Release: release})
			if err != nil {
				return err
			}
			packets[pol.Name] = cp.(*CreatePackageRet).SwupChain
			// suppose the client has the first packet
			latest[pol.Name] = packets[pol.Name].Data.Hash
		}
		round.Record()
		if dr.Time.Sub(now) >= updateFrequency {
			// Measure bandwidth-usage for updating client
			log.Lvlf1("Updating client at %s after %s", now, dr.Time.Sub(now))
			now = dr.Time
			/*         client := NewClient(config.Roster)*/
			//ids := orderedIdsFromName(latest)
			//lbr, err := client.LatestUpdates(ids)
			//log.ErrFatal(err)
			//// do verification
			//verification(client, latest, lbr, config.Roster.Publics())
			//// update latest
			//for i, n := range orderName(latest) {
			//upds := lbr.Updates[i]
			//latest[n] = upds[len(upds)-1].Hash
			/*}*/
		}

	}
	return nil
}

func verification(c *Client, latest map[string]skipchain.SkipBlockID, lbr *LatestBlocksRet, publics []abstract.Point) {
	// TODO
	timeClient := timestamp.NewClient()
	// create nonce:
	r := make([]byte, 20)
	_, err := rand.Read(r)
	log.ErrFatal(err, "Couldn't read random bytes:")
	nonce := sha256.Sum256(r)

	// send request:
	resp, err := timeClient.SignMsg(c.Root, nonce[:])
	log.ErrFatal(err, "Couldn't sign nonce.")

	// Verify the time is in the good range:
	ts := time.Unix(resp.Timestamp, 0)
	latesBlockTime := time.Unix(lbr.Timestamp.Timestamp, 0)
	if ts.Sub(latesBlockTime) > time.Hour {
		log.Warn("Timestamp of latest block is older than one hour!")
	}

	names := orderName(latest)
	tr, err := c.TimestampRequests(names)
	for i, n := range names {
		proof := tr.Proofs[n]
		updates := lbr.Updates[i]
		leaf := updates[len(updates)-1].Hash
		log.ErrFatal(err)
		if !proof.Check(HashFunc(), lbr.Timestamp.Root, leaf) {
			log.Warn("Proof of inclusion is not correct")
		} else {
			log.Lvl2("Proof verification!")
		}
	}

	// verify signature
	msg := MarshalPair(lbr.Timestamp.Root, lbr.Timestamp.SignatureResponse.Timestamp)
	err = swupdate.VerifySignature(network.Suite, publics, msg, lbr.Timestamp.SignatureResponse.Signature)
	if err != nil {
		log.Warn("Signature timestamp invalid")
	} else {
		log.Lvl2("Signature timestamp Valid :)")
	}
}

func orderName(m map[string]skipchain.SkipBlockID) []string {
	n := make([]string, len(m))
	var i int
	for k := range m {
		n[i] = k
		i++
	}
	sort.Strings(n)
	return n
}

func orderedIdsFromName(m map[string]skipchain.SkipBlockID) []skipchain.SkipBlockID {
	names := orderName(m)
	ids := make([]skipchain.SkipBlockID, len(names))
	for i, n := range names {
		ids[i] = m[n]
	}
	return ids
}
