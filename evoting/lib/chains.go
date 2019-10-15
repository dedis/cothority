package lib

import (
	"encoding/binary"
	"errors"
	"sync"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/schnorr"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
)

// NewSkipchain creates a new skipchain for a given roster and verification function.
func NewSkipchain(s *skipchain.Service, roster *onet.Roster, testMode bool) (
	*skipchain.SkipBlock, error) {
	block := skipchain.NewSkipBlock()
	block.Roster = roster
	block.BaseHeight = 8
	block.MaximumHeight = 4
	verifier := []skipchain.VerifierID{skipchain.VerifyBase, TransactionVerifierID}
	if testMode {
		verifier = skipchain.VerificationStandard
	}
	block.VerifierIDs = verifier
	block.Data = []byte{}

	reply, err := s.StoreSkipBlockInternal(&skipchain.StoreSkipBlock{
		NewBlock: block,
	})
	if err != nil {
		return nil, err
	}
	return reply.Latest, nil
}

// StoreUsingWebsocket appends a new block holding data to an existing skipchain
// using websockets. Used for storing a block while executing a protocol.
func StoreUsingWebsocket(id skipchain.SkipBlockID, roster *onet.Roster, transaction *Transaction) error {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(roster, id)
	if err != nil {
		return err
	}
	enc, err := protobuf.Encode(transaction)
	if err != nil {
		return err
	}
	_, err = client.StoreSkipBlock(reply.Update[len(reply.Update)-1], nil, enc)
	if err != nil {
		return err
	}
	return nil
}

// This global is a hack; it makes sure that there can be no parallel invocation
// of Store, so that Store can accurately predict what will be the next block's
// index.  When StoreUsingWebsocket is being called, calls to Store are no
// longer gauranteed to find the correct Index, but in practice that does not
// happen, since StoreUsingWebsocket is only used to write Mix and Decrypt txns,
// and ballots cannot be cast once Shuffles start getting executed.
// In theory, interleaving election Opens with finalising another election
// could fail, but we do not use the election system like that either.
var storeMu sync.Mutex

// Store appends a new block holding data to an existing skipchain using the
// skipchain service. The transaction is signed if the private key is provided.
func Store(s *skipchain.Service, ID skipchain.SkipBlockID, transaction *Transaction, priv kyber.Scalar) (skipchain.SkipBlockID, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	db := s.GetDB()
	latest, err := db.GetLatest(db.GetByID(ID))
	if err != nil {
		return nil, errors.New("couldn't find latest block: " + err.Error())
	}

	block := latest.Copy()
	block.GenesisID = block.SkipChainID()
	if transaction.Master != nil {
		log.Lvl2("Setting new roster for master skipchain.")
		block.Roster = transaction.Master.Roster
	}
	block.Index++

	if priv != nil {
		txhash := transaction.Hash()
		msg := make([]byte, 8)
		binary.LittleEndian.PutUint64(msg, uint64(block.Index))
		msg = append(msg, txhash...)

		sig, err := schnorr.Sign(cothority.Suite, priv, msg)
		if err != nil {
			return nil, err
		}
		transaction.Signature = sig
	}

	enc, err := protobuf.Encode(transaction)
	if err != nil {
		return nil, err
	}
	block.Data = enc

	// Using an unset LatestID with block.GenesisID set is to ensure concurrent
	// append.
	storeSkipBlockReply, err := s.StoreSkipBlockInternal(&skipchain.StoreSkipBlock{
		NewBlock:          block,
		TargetSkipChainID: latest.SkipChainID(),
	})
	if err != nil {
		return nil, err
	}
	return storeSkipBlockReply.Latest.Hash, nil
}
