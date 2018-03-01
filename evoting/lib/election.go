package lib

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share/dkg/rabin"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
)

const (
	// Running depicts that an election is open for ballot casting.
	Running = iota
	// Shuffled depicts that the mixes have been created.
	Shuffled
	// Decrypted depicts that the partials have been created.
	Decrypted
	// Corrupt depicts that the election skipchain has been corrupted.
	Corrupt
)

// Election is the base object for a voting procedure. It is stored
// in the second skipblock right after the (empty) genesis block. A reference
// to the election skipchain is appended to the master skipchain upon opening.
type Election struct {
	Name    string   // Name of the election.
	Creator uint32   // Creator is the election responsible.
	Users   []uint32 // Users is the list of registered voters.

	ID     skipchain.SkipBlockID // ID is the hash of the genesis block.
	Roster *onet.Roster          // Roster is the set of responsible nodes
	Key    kyber.Point           // Key is the DKG public key.
	Stage  uint32                // Stage indicates the phase of the election.

	Candidates  []uint32 // Candidates is the list of candidate scipers.
	MaxChoices  uint32   // MaxChoices is the max votes in allowed in a ballot.
	Description string   // Description in string format.
	End         int64    // End (termination) datetime as unix timestamp.
}

func init() {
	network.RegisterMessages(Election{}, Ballot{}, Box{}, Mix{}, Partial{})
}

// FetchElection retrieves the election object from its skipchain and sets its stage.
func FetchElection(roster *onet.Roster, id skipchain.SkipBlockID) (*Election, error) {
	chain, err := chain(roster, id)
	if err != nil {
		return nil, err
	}

	_, blob, _ := network.Unmarshal(chain[1].Data, cothority.Suite)
	election := blob.(*Election)

	n, numMixes, numPartials := len(election.Roster.List), 0, 0
	for _, block := range chain {
		_, blob, _ := network.Unmarshal(block.Data, cothority.Suite)
		if _, ok := blob.(*Mix); ok {
			numMixes++
		} else if _, ok := blob.(*Partial); ok {
			numPartials++
		}
	}

	if numMixes == 0 && numPartials == 0 {
		election.Stage = Running
	} else if numMixes == n && numPartials == 0 {
		election.Stage = Shuffled
	} else if numMixes == n && numPartials == n {
		election.Stage = Decrypted
	} else {
		election.Stage = Corrupt
	}
	return election, nil
}

// GenChain creates an election skipchain for a specific stage and a given number of ballots.
func (e *Election) GenChain(numBallots int) []*dkg.DistKeyGenerator {
	chain, _ := New(e.Roster, nil)

	n := len(e.Roster.List)
	dkgs, _ := DKGSimulate(n, n-1)
	secret, _ := NewSharedSecret(dkgs[0])

	e.ID = chain.Hash
	e.Key = secret.X

	box := genBox(secret.X, numBallots)
	mixes := box.genMix(secret.X, n)
	partials := mixes[n-1].genPartials(dkgs)

	e.Store(e)
	e.storeBallots(box.Ballots)

	if e.Stage == Shuffled {
		e.storeMixes(mixes)
	} else if e.Stage == Decrypted {
		e.storeMixes(mixes)
		e.storePartials(partials)
	}
	return dkgs
}

// Store appends a given structure to the election skipchain.
func (e *Election) Store(data interface{}) error {
	chain, err := chain(e.Roster, e.ID)
	if err != nil {
		return err
	}

	if _, err := client.StoreSkipBlock(chain[len(chain)-1], e.Roster, data); err != nil {
		return err
	}
	return nil
}

// Box accumulates all the ballots while only keeping the last ballot for each user.
func (e *Election) Box() (*Box, error) {
	chain, err := chain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	// Use map to only included a user's last ballot.
	mapping := make(map[uint32]*Ballot)
	for _, block := range chain {
		_, blob, _ := network.Unmarshal(block.Data, cothority.Suite)
		if ballot, ok := blob.(*Ballot); ok {
			mapping[ballot.User] = ballot
		}
	}

	ballots := make([]*Ballot, 0)
	for _, ballot := range mapping {
		ballots = append(ballots, ballot)
	}
	return &Box{Ballots: ballots}, nil
}

// Mixes returns all mixes created by the roster conodes.
func (e *Election) Mixes() ([]*Mix, error) {
	chain, err := chain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	mixes := make([]*Mix, 0)
	for _, block := range chain {
		_, blob, _ := network.Unmarshal(block.Data, cothority.Suite)
		if mix, ok := blob.(*Mix); ok {
			mixes = append(mixes, mix)
		}
	}

	return mixes, nil
}

// Partials returns the partial decryption for each roster conode.
func (e *Election) Partials() ([]*Partial, error) {
	chain, err := chain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	partials := make([]*Partial, 0)
	for _, block := range chain {
		_, blob, _ := network.Unmarshal(block.Data, cothority.Suite)
		if partial, ok := blob.(*Partial); ok {
			partials = append(partials, partial)
		}
	}

	return partials, nil
}

// IsUser checks if a given user is a registered voter for the election.
func (e *Election) IsUser(user uint32) bool {
	for _, u := range e.Users {
		if u == user {
			return true
		}
	}
	return false
}

// IsCreator checks if a given user is the creator of the election.
func (e *Election) IsCreator(user uint32) bool {
	return user == e.Creator
}

// storeBallots appends a list of ballots to the election skipchain.
func (e *Election) storeBallots(ballots []*Ballot) error {
	for _, ballot := range ballots {
		if err := e.Store(ballot); err != nil {
			return err
		}
	}
	return nil
}

// storeBallots appends a list of mixes to the election skipchain.
func (e *Election) storeMixes(mixes []*Mix) error {
	for _, mix := range mixes {
		if err := e.Store(mix); err != nil {
			return err
		}
	}
	return nil
}

// storeBallots appends a list of partials to the election skipchain.
func (e *Election) storePartials(partials []*Partial) error {
	for _, partial := range partials {
		if err := e.Store(partial); err != nil {
			return err
		}
	}
	return nil
}
