package lib

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"

	uuid "github.com/satori/go.uuid"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
)

func init() {
	network.RegisterMessage(&Transaction{})
}

// TransactionVerifierID identifes the core transaction verification function.
var TransactionVerifierID = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, ""))

// TransactionVerifiers is a list of accepted skipchain verification functions.
var TransactionVerifiers = []skipchain.VerifierID{TransactionVerifierID}

// Transaction is the sole data structure withing the blocks of an election
// skipchain, it holds all the other containers.
type Transaction struct {
	Master *Master
	Link   *Link

	Election *Election
	Ballot   *Ballot
	Mix      *Mix
	Partial  *Partial

	User      uint32
	Signature []byte
}

// UnmarshalTransaction decodes a data blob to a transaction structure.
func UnmarshalTransaction(data []byte) *Transaction {
	transaction := &Transaction{}
	err := protobuf.DecodeWithConstructors(
		data,
		transaction,
		network.DefaultConstructors(cothority.Suite),
	)
	if err != nil {
		return nil
	}
	return transaction
}

// NewTransaction constructs a new transaction for the given arguments.
func NewTransaction(data interface{}, user uint32, signature []byte) *Transaction {
	transaction := &Transaction{User: user, Signature: signature}
	switch data.(type) {
	case *Master:
		transaction.Master = data.(*Master)
	case *Link:
		transaction.Link = data.(*Link)
	case *Election:
		transaction.Election = data.(*Election)
	case *Ballot:
		transaction.Ballot = data.(*Ballot)
	case *Mix:
		transaction.Mix = data.(*Mix)
	case *Partial:
		transaction.Partial = data.(*Partial)
	default:
		return nil
	}
	return transaction
}

// Digest appends the digits of sciper to master genesis skipblock ID
func (t *Transaction) Digest(s *skipchain.Service, genesis skipchain.SkipBlockID) []byte {
	var message []byte
	switch {
	case t.Master != nil:
		message = t.Master.ID
	case t.Election != nil:
		message = t.Election.Master
	default:
		election, _ := GetElection(s, genesis, false, t.User)
		if election == nil {
			return nil
		}
		message = election.Master
	}
	for _, c := range strconv.Itoa(int(t.User)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	return message
}

// Verify checks that the corresponding transaction is valid before storing it.
func (t *Transaction) Verify(genesis skipchain.SkipBlockID, s *skipchain.Service) error {
	digest := t.Digest(s, genesis)
	if t.Master != nil {
		// Find the current master in order to compare against it.
		m, err := GetMaster(s, genesis)
		if err != nil {
			// This chain does not exist, yet. Allow it to be created.
			return nil
		}

		err = schnorr.Verify(cothority.Suite, m.Key, digest, t.Signature)
		if err != nil {
			return err
		}
		if !m.IsAdmin(t.User) {
			return errors.New("current user was not in previous admin list")
		}

		// Changing this would not make any sense.
		if !t.Master.ID.Equal(m.ID) {
			return errors.New("mismatched ID in master update")
		}

		// All the other fields (admin list, roster, and front end key) may change, but
		// let's apply some sanity checks to them.

		if len(t.Master.Admins) == 0 {
			return errors.New("empty admin list in master update")
		}
		if len(t.Master.Roster.List) == 0 {
			return errors.New("empty roster in master update")
		}
		null := t.Master.Key.Clone().Null()
		if t.Master.Key.Equal(null) {
			return errors.New("null key in master update")
		}

		return nil
	} else if t.Link != nil {
		master, err := GetMaster(s, genesis)
		if err != nil {
			return err
		}

		if !master.IsAdmin(t.User) {
			return errors.New("link error: user not admin")
		}
		return nil
	} else if t.Election != nil {
		election := t.Election
		err := schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}
		if election.End < time.Now().Unix() {
			return errors.New("open error: invalid end date")
		}

		master, err := GetMaster(s, election.Master)
		if err != nil {
			return err
		}
		if !master.IsAdmin(t.User) {
			return errors.New("open error: user not admin")
		}
		return nil
	} else if t.Ballot != nil {
		election, err := GetElection(s, genesis, false, t.User)
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}

		// t.User is trusted at this point, so make sure that they did not try to sneak
		// through a different user-id in the ballot.
		if t.User != t.Ballot.User {
			return errors.New("ballot user-id differs from transaction user-id")
		}

		latest, err := s.GetDB().GetLatest(s.GetDB().GetByID(election.ID))
		transaction := UnmarshalTransaction(latest.Data)
		if err != nil {
			return err
		}
		if transaction.Mix != nil || transaction.Partial != nil {
			return errors.New("cast error: election not in running stage")
		} else if !election.IsUser(t.User) {
			return errors.New("cast error: user not part")
		}
		return nil
	} else if t.Mix != nil {
		election, err := GetElection(s, genesis, false, t.User)
		if err != nil {
			return err
		}

		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}
		if !election.IsCreator(t.User) {
			return errors.New("shuffle error: user is not election creator")
		}

		// verify proposer
		_, proposer := election.Roster.Search(t.Mix.NodeID)
		if proposer == nil {
			return errors.New("didn't find signer in mix")
		}
		data, err := proposer.Public.MarshalBinary()
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, proposer.Public, data, t.Mix.Signature)
		if err != nil {
			return err
		}

		mixes, err := election.Mixes(s)
		if err != nil {
			return err
		}

		if len(mixes) > 2*len(election.Roster.List)/3 {
			return errors.New("shuffle error: election already shuffled")
		}

		for _, mix := range mixes {
			_, mixProposer := election.Roster.Search(mix.NodeID)
			if mixProposer == nil {
				return errors.New("didn't find signer in mix")
			}

			if mixProposer.Public.Equal(proposer.Public) {
				return fmt.Errorf("%s has already proposed a shuffle", mixProposer)
			}
		}

		// check if Mix is valid
		var x, y []kyber.Point
		if len(mixes) == 0 {
			// verify against Boxes
			boxes, err := election.Box(s)
			if err != nil {
				return err
			}
			x, y = Split(boxes.Ballots)
		} else {
			// verify against the last mix
			x, y = Split(mixes[len(mixes)-1].Ballots)
		}
		v, w := Split(t.Mix.Ballots)
		err = Verify(t.Mix.Proof, election.Key, x, y, v, w)
		if err != nil {
			return err
		}

		return nil
	} else if t.Partial != nil {
		election, err := GetElection(s, genesis, false, t.User)
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}
		if !election.IsCreator(t.User) {
			return errors.New("decrypt error: user is not election creator")
		}

		mixes, err := election.Mixes(s)
		target := 2 * len(election.Roster.List) / 3
		if err != nil {
			return err
		} else if len(mixes) <= target {
			return errors.New("decrypt error: election not shuffled yet")
		}
		partials, err := election.Partials(s)

		if len(partials) >= len(election.Roster.List) {
			return errors.New("decrypt error: election already decrypted")
		}

		for _, partial := range partials {
			if partial.NodeID.Equal(t.Partial.NodeID) {
				return fmt.Errorf("%s has already proposed a partial", t.Partial.NodeID)
			}
		}

		// verify proposer
		_, proposer := election.Roster.Search(t.Partial.NodeID)
		if proposer == nil {
			return errors.New("didn't find node who created the partial")
		}
		data, err := proposer.Public.MarshalBinary()
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, proposer.Public, data, t.Partial.Signature)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("transaction error: empty transaction")
}
