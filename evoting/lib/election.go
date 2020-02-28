package lib

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"

	"go.dedis.ch/cothority/v3/skipchain"
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
	Name    map[string]string // Name of the election. lang-code, value pair
	Creator uint32            // Creator is the election responsible.
	Users   []uint32          // Users is the list of registered voters.

	ID        skipchain.SkipBlockID // ID is the hash of the genesis block.
	Master    skipchain.SkipBlockID // Master is the hash of the master skipchain.
	Roster    *onet.Roster          // Roster is the set of responsible nodes.
	Key       kyber.Point           // Key is the DKG public key.
	MasterKey kyber.Point           // MasterKey is the front-end public key.
	Stage     ElectionState         // Stage indicates the phase of election and is used for filtering in frontend

	Candidates []uint32          // Candidates is the list of candidate scipers.
	MaxChoices int               // MaxChoices is the max votes in allowed in a ballot.
	Subtitle   map[string]string // Description in string format. lang-code, value pair
	MoreInfo   string            // MoreInfo is the url to AE Website for the given election.
	Start      int64             // Start denotes the election start unix timestamp
	End        int64             // End (termination) datetime as unix timestamp.

	Theme  string // Theme denotes the CSS class for selecting background color of card title.
	Footer Footer // Footer denotes the Election footer

	Voted        skipchain.SkipBlockID // Voted denotes if a user has already cast a ballot for this election.
	MoreInfoLang map[string]string     // MoreInfoLang, is MoreInfo, but as a lang-code/value map. MoreInfoLang should be used in preference to MoreInfo.
}

// Footer denotes the fields for the election footer
type Footer struct {
	Text         string // Text is for storing footer content.
	ContactTitle string // ContactTitle stores the title of the Contact person.
	ContactPhone string // ContactPhone stores the phone number of the Contact person.
	ContactEmail string // ContactEmail stores the email address of the Contact person.
}

// GetElection fetches the election structure from its skipchain and sets the stage.
func GetElection(s *skipchain.Service, id skipchain.SkipBlockID,
	checkVoted bool, user uint32) (*Election, error) {

	var election *Election
	index := 1
	for {
		search, err := s.GetSingleBlockByIndex(
			&skipchain.GetSingleBlockByIndex{Genesis: id, Index: index},
		)
		if err != nil {
			return nil, err
		}

		transaction := UnmarshalTransaction(search.SkipBlock.Data)
		if transaction == nil {
			return nil, fmt.Errorf("no election structure in %s", id.Short())
		}
		// Found last Election tx, exit.
		if transaction.Election == nil {
			break
		}
		election = transaction.Election
		err = election.setStage(s)
		if err != nil {
			return nil, err
		}

		// Stop looping at the end of the chain.
		if len(search.SkipBlock.ForwardLink) == 0 {
			break
		}
		// otherwise try the next index.
		index++
	}
	if election == nil {
		return nil, errors.New("no election found")
	}
	// check for voted only if required. We cache things in localStorage
	// on the frontend
	if checkVoted {
		err := election.setVoted(s, user)
		if err != nil {
			return election, err
		}
	}
	return election, nil
}

// setVoted sets the Voted field of the election to the skipblock id
// of the last ballot cast by the user
func (e *Election) setVoted(s *skipchain.Service, user uint32) error {
	db := s.GetDB()
	block := db.GetByID(e.ID)
	if block == nil {
		return errors.New("Election skipchain empty")
	}

	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction == nil {
			if len(block.ForwardLink) == 0 {
				break
			}
			block = db.GetByID(block.ForwardLink[0].To)
			continue
		}
		if transaction.Ballot != nil && transaction.Ballot.User == user {
			e.Voted = block.Hash
		}
		if transaction.Mix != nil || transaction.Partial != nil {
			break
		}
		if len(block.ForwardLink) == 0 {
			break
		}
		block = db.GetByID(block.ForwardLink[0].To)
	}
	return nil
}

func (e *Election) setStage(s *skipchain.Service) error {
	// threshold is the minimum number of blocks we need
	// to complete a shuffle or a decryption. Following
	// byzantine consensus it's set to floor(2*n/3) + 1
	threshold := 2*len(e.Roster.List)/3 + 1
	partials, err := e.Partials(s)
	if err != nil {
		return err
	}
	if len(partials) >= threshold {
		e.Stage = Decrypted
		return nil
	}

	mixes, err := e.Mixes(s)
	if err != nil {
		return err
	}
	if len(mixes) >= threshold {
		e.Stage = Shuffled
		return nil
	}
	e.Stage = Running
	return nil
}

// Box accumulates all the ballots while only keeping the last ballot for each user.
func (e *Election) Box(s *skipchain.Service) (*Box, error) {
	search, err := s.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{
			Genesis: e.ID,
			Index:   0,
		})
	if err != nil {
		return nil, err
	}
	block := search.SkipBlock

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
		block, _ = s.GetSingleBlock(
			&skipchain.GetSingleBlock{
				ID: block.ForwardLink[0].To,
			})
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
func (e *Election) Mixes(s *skipchain.Service) ([]*Mix, error) {

	// Traversing forward one by one might be expensive in a large
	// election. It's better to search for the Mix transactions
	// from the end of the skipchain
	block, err := s.GetDB().GetLatest(s.GetDB().GetByID(e.ID))
	if err != nil {
		return nil, err
	}

	mixes := make([]*Mix, 0)
	for block != nil {
		transaction := UnmarshalTransaction(block.Data)
		if transaction == nil {
			if len(block.BackLinkIDs) == 0 {
				break
			}
			block = s.GetDB().GetByID(block.BackLinkIDs[0])
			continue
		}
		if transaction.Mix == nil && transaction.Partial == nil {
			// we're done
			break
		}

		if transaction.Mix != nil {
			// append to the mixes array
			mixes = append(mixes, transaction.Mix)
		}

		if len(block.BackLinkIDs) == 0 {
			break
		}

		// keep iterating back
		block = s.GetDB().GetByID(block.BackLinkIDs[0])
	}
	// reverse the slice since we iterated in reverse before
	for i := len(mixes)/2 - 1; i >= 0; i-- {
		opp := len(mixes) - 1 - i
		mixes[i], mixes[opp] = mixes[opp], mixes[i]
	}
	return mixes, nil
}

// Partials returns the partial decryption for each roster conode.
func (e *Election) Partials(s *skipchain.Service) ([]*Partial, error) {

	// Traversing forward one by one might be expensive in a large
	// election. It's better to search for the Partial transactions
	// from the end of the skipchain
	block, err := s.GetDB().GetLatest(s.GetDB().GetByID(e.ID))
	if err != nil {
		return nil, err
	}

	partials := make([]*Partial, 0)
	for block != nil {
		transaction := UnmarshalTransaction(block.Data)
		if transaction == nil {
			if len(block.BackLinkIDs) == 0 {
				break
			}
			block = s.GetDB().GetByID(block.BackLinkIDs[0])
			continue
		}
		if transaction.Partial == nil {
			// we're done
			break
		}

		if transaction.Partial != nil {
			partials = append(partials, transaction.Partial)
		}

		if len(block.BackLinkIDs) == 0 {
			break
		}

		// keep iterating back
		block = s.GetDB().GetByID(block.BackLinkIDs[0])
	}
	// reverse the slice since we iterated in reverse before
	for i := len(partials)/2 - 1; i >= 0; i-- {
		opp := len(partials) - 1 - i
		partials[i], partials[opp] = partials[opp], partials[i]
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

func (e *Election) String() string {
	str := new(strings.Builder)

	fmt.Fprintf(str, "Election %x on master %x\n", e.ID, e.Master)
	fmt.Fprintf(str, "Creator: %v\n", e.Creator)
	fmt.Fprintf(str, "Name:\n")
	printLang(str, e.Name)
	fmt.Fprintf(str, "Subtitle:\n")
	printLang(str, e.Subtitle)
	fmt.Fprintf(str, "Candidates: %v\n", e.Candidates)
	fmt.Fprintf(str, "MaxChoices: %v\n", e.MaxChoices)
	fmt.Fprintf(str, "MoreInfo: %v\n", e.MoreInfo)
	fmt.Fprintf(str, "MoreInfoLang:\n")
	printLang(str, e.MoreInfoLang)
	fmt.Fprintf(str, "Theme: %v\n", e.Theme)
	fmt.Fprintf(str, "Footer Text: %v\n", e.Footer.Text)
	fmt.Fprintf(str, "Footer ContactTitle: %v\n", e.Footer.ContactTitle)
	fmt.Fprintf(str, "Footer ContactPhone: %v\n", e.Footer.ContactPhone)
	fmt.Fprintf(str, "Footer ContactEmail: %v\n", e.Footer.ContactEmail)
	fmt.Fprintf(str, "Start: %v\n", e.Start)
	fmt.Fprintf(str, "End: %v\n", e.End)
	fmt.Fprintf(str, "Election pubkey: %v\n", e.Key)
	fmt.Fprintf(str, "Authentication server pubkey: %v\n", e.MasterKey)
	fmt.Fprintf(str, "Stage: %v\n", e.Stage)
	fmt.Fprintf(str, "Voters: %v\n", e.Users)

	return str.String()
}

func printLang(w io.Writer, x map[string]string) {
	var l []string
	for lang := range x {
		l = append(l, lang)
	}
	sort.Strings(l)
	for _, lang := range l {
		fmt.Fprintf(w, "  %v: %v\n", lang, x[lang])
	}
}
