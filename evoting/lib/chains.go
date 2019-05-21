package lib

import (
	"errors"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"

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

// Store appends a new block holding data to an existing skipchain using the
// skipchain service
func Store(s *skipchain.Service, ID skipchain.SkipBlockID, transaction *Transaction) (skipchain.SkipBlockID, error) {
	db := s.GetDB()
	latest, err := db.GetLatest(db.GetByID(ID))
	if err != nil {
		return nil, errors.New("couldn't find latest block: " + err.Error())
	}

	enc, err := protobuf.Encode(transaction)
	if err != nil {
		return nil, err
	}

	block := latest.Copy()
	block.Data = enc
	block.GenesisID = block.SkipChainID()
	if transaction.Master != nil {
		log.Lvl2("Setting new roster for master skipchain.")
		block.Roster = transaction.Master.Roster
	}
	block.Index++
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
