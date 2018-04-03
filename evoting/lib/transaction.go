package lib

import (
	"errors"
	"strconv"
	"time"

	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
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
func (t *Transaction) Digest(roster *onet.Roster, genesis skipchain.SkipBlockID) []byte {
	var election *Election
	if t.Election != nil {
		election = t.Election
	} else {
		election, _ = GetElection(roster, genesis)
	}
	// Master or Link transaction
	if election == nil {
		return nil
	}
	message := election.Master
	for _, c := range strconv.Itoa(int(t.User)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	return message
}

// Verify checks that the corresponding transaction is valid before storing it.
func (t *Transaction) Verify(genesis skipchain.SkipBlockID, roster *onet.Roster) error {
	digest := t.Digest(roster, genesis)
	if t.Master != nil {
		return nil
	} else if t.Link != nil {
		master, err := GetMaster(roster, genesis)
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

		master, err := GetMaster(roster, election.Master)
		if err != nil {
			return err
		}
		if !master.IsAdmin(t.User) {
			return errors.New("open error: user not admin")
		}
		return nil
	} else if t.Ballot != nil {
		election, err := GetElection(roster, genesis)
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}

		mixes, err := election.Mixes()
		if err != nil {
			return err
		} else if len(mixes) > 0 {
			return errors.New("cast error: election not in running stage")
		} else if !election.IsUser(t.User) {
			return errors.New("cast error: user not part")
		}
		return nil
	} else if t.Mix != nil {
		election, err := GetElection(roster, genesis)
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}

		mixes, err := election.Mixes()
		if err != nil {
			return err
		} else if len(mixes) == len(roster.List) {
			return errors.New("shuffle error: election already shuffled")
		} else if !election.IsCreator(t.User) {
			return errors.New("shuffle error: user is not election creator")
		}
		return nil
	} else if t.Partial != nil {
		election, err := GetElection(roster, genesis)
		if err != nil {
			return err
		}
		err = schnorr.Verify(cothority.Suite, election.MasterKey, digest, t.Signature)
		if err != nil {
			return err
		}

		mixes, err := election.Mixes()
		if err != nil {
			return err
		} else if len(mixes) != len(roster.List) {
			return errors.New("decrypt error, election not shuffled yet")
		}

		partials, err := election.Partials()
		if err != nil {
			return err
		} else if len(partials) == len(roster.List) {
			return errors.New("decrypt error: election already decrypted")
		} else if !election.IsCreator(t.User) {
			return errors.New("decrypt error: user is not election creator")
		}
		return nil
	}
	return errors.New("transaction error: empty transaction")
}
