package lib

import (
	"errors"
	"fmt"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

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
	Master skipchain.SkipBlockID // Master is the hash of the master skipchain.
	Roster *onet.Roster          // Roster is the set of responsible nodes
	Key    kyber.Point           // Key is the DKG public key.
	Stage  uint32                // Stage indicates the phase of the election.

	Candidates []uint32 // Candidates is the list of candidate scipers.
	MaxChoices int      // MaxChoices is the max votes in allowed in a ballot.
	Subtitle   string   // Description in string format.
	MoreInfo   string   // MoreInfo is the url to AE Website for the given election.
	Start      int64    // Start denotes the election start unix timestamp
	End        int64    // End (termination) datetime as unix timestamp.

	Theme  string // Theme denotes the CSS class for selecting background color of card title.
	Footer footer // Footer denotes the Election footer
}

// footer denotes the fields for the election footer
type footer struct {
	Text         string // Text is for storing footer content.
	ContactTitle string // ContactTitle stores the title of the Contact person.
	ContactPhone string // ContactPhone stores the phone number of the Contact person.
	ContactEmail string // ContactEmail stores the email address of the Contact person.
}

func init() {
	network.RegisterMessages(Election{}, Ballot{}, Box{}, Mix{}, Partial{})
}

func GetElection(roster *onet.Roster, id skipchain.SkipBlockID) (*Election, error) {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(roster, id)
	if err != nil {
		return nil, err
	}

	transaction := UnmarshalTransaction(reply.Update[1].Data)
	if transaction == nil || transaction.Election == nil {
		return nil, errors.New(fmt.Sprintf("no election structure in %s", id.Short()))
	}
	election := transaction.Election

	n, mixes, partials := len(election.Roster.List), 0, 0
	for _, block := range reply.Update {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Mix != nil {
			mixes++
		} else if transaction != nil && transaction.Partial != nil {
			partials++
		}
	}

	if mixes < n && partials == 0 {
		election.Stage = Running
	} else if mixes == n && partials < n {
		election.Stage = Shuffled
	} else if mixes == n && partials == n {
		election.Stage = Decrypted
	} else {
		election.Stage = Corrupt
	}
	return election, nil
}

// Box accumulates all the ballots while only keeping the last ballot for each user.
func (e *Election) Box() (*Box, error) {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	// Use map to only included a user's last ballot.
	ballots := make([]*Ballot, 0)
	for _, block := range reply.Update {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Ballot != nil {
			ballots = append(ballots, transaction.Ballot)
		}
	}

	// Reverse ballot list
	for i, j := 0, len(ballots)-1; i < j; i, j = i+1, j-1 {
		ballots[i], ballots[j] = ballots[j], ballots[i]
	}

	// Only keep last casted ballot per user
	mapping := make(map[uint32]bool)
	unique := make([]*Ballot, 0)
	for _, ballot := range ballots {
		if _, found := mapping[ballot.User]; !found {
			unique = append(unique, ballot)
			mapping[ballot.User] = true
		}
	}

	// Reverse back list of unique ballots
	for i, j := 0, len(unique)-1; i < j; i, j = i+1, j-1 {
		unique[i], unique[j] = unique[j], unique[i]
	}
	return &Box{Ballots: unique}, nil
}

// Mixes returns all mixes created by the roster conodes.
func (e *Election) Mixes() ([]*Mix, error) {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	mixes := make([]*Mix, 0)
	for _, block := range reply.Update {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Mix != nil {
			mixes = append(mixes, transaction.Mix)
		}
	}

	return mixes, nil
}

// Partials returns the partial decryption for each roster conode.
func (e *Election) Partials() ([]*Partial, error) {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(e.Roster, e.ID)
	if err != nil {
		return nil, err
	}

	partials := make([]*Partial, 0)
	for _, block := range reply.Update {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Partial != nil {
			partials = append(partials, transaction.Partial)
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
