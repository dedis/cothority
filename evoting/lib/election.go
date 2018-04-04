package lib

import (
	"fmt"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/skipchain"
)

// ElectionState is the type for storing the stage of Election.
type ElectionState uint32

const (
	// Running depicts that an election is open for ballot casting
	Running ElectionState = iota + 1
	// Shuffled depicts that the mixes have been created
	Shuffled
	// Decrypted depicts that the partials have been decrypted
	Decrypted
)

func init() {
	network.RegisterMessages(Election{}, Ballot{}, Box{}, Mix{}, Partial{})
}

// Election is the base object for a voting procedure. It is stored
// in the second skipblock right after the (empty) genesis block. A reference
// to the election skipchain is appended to the master skipchain upon opening.
type Election struct {
	Name    string   // Name of the election.
	Creator uint32   // Creator is the election responsible.
	Users   []uint32 // Users is the list of registered voters.

	ID        skipchain.SkipBlockID // ID is the hash of the genesis block.
	Master    skipchain.SkipBlockID // Master is the hash of the master skipchain.
	Roster    *onet.Roster          // Roster is the set of responsible nodes.
	Key       kyber.Point           // Key is the DKG public key.
	MasterKey kyber.Point           // MasterKey is the front-end public key.
	Stage     ElectionState         // Stage indicates the phase of election and is used for filtering in frontend

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

// GetElection fetches the election structure from its skipchain and sets the stage.
func GetElection(roster *onet.Roster, id skipchain.SkipBlockID) (*Election, error) {
	client := skipchain.NewClient()

	block, err := client.GetSingleBlockByIndex(roster, id, 1)
	if err != nil {
		return nil, err
	}

	// transaction := UnmarshalTransaction(reply.Update[1].Data)
	transaction := UnmarshalTransaction(block.Data)
	if transaction == nil || transaction.Election == nil {
		return nil, fmt.Errorf("no election structure in %s", id.Short())
	}
	election := transaction.Election
	err = election.setStage()
	if err != nil {
		return nil, err
	}
	return election, nil
}

func (e *Election) setStage() error {
	mixes, err := e.Mixes()
	if err != nil {
		return err
	}

	partials, err := e.Partials()
	if err != nil {
		return err
	}

	if len(mixes) == 0 {
		e.Stage = Running
	} else if len(partials) == len(e.Roster.List) {
		e.Stage = Decrypted
	} else {
		e.Stage = Shuffled
	}
	return nil
}

// Box accumulates all the ballots while only keeping the last ballot for each user.
func (e *Election) Box() (*Box, error) {
	client := skipchain.NewClient()

	block, err := client.GetSingleBlockByIndex(e.Roster, e.ID, 0)
	if err != nil {
		return nil, err
	}

	// Use map to only included a user's last ballot.
	ballots := make([]*Ballot, 0)
	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Ballot != nil {
			ballots = append(ballots, transaction.Ballot)
		}

		if len(block.ForwardLink) <= 0 {
			break
		}
		block, _ = client.GetSingleBlock(e.Roster, block.ForwardLink[0].To)
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

	block, err := client.GetSingleBlockByIndex(e.Roster, e.ID, 0)
	if err != nil {
		return nil, err
	}

	mixes := make([]*Mix, 0)
	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Mix != nil {
			mixes = append(mixes, transaction.Mix)
		}

		if len(block.ForwardLink) <= 0 {
			break
		}
		block, _ = client.GetSingleBlock(e.Roster, block.ForwardLink[0].To)
	}
	return mixes, nil
}

// Partials returns the partial decryption for each roster conode.
func (e *Election) Partials() ([]*Partial, error) {
	client := skipchain.NewClient()

	block, err := client.GetSingleBlockByIndex(e.Roster, e.ID, 0)
	if err != nil {
		return nil, err
	}

	partials := make([]*Partial, 0)
	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Partial != nil {
			partials = append(partials, transaction.Partial)
		}

		if len(block.ForwardLink) <= 0 {
			break
		}
		block, _ = client.GetSingleBlock(e.Roster, block.ForwardLink[0].To)
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
