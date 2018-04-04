package lib

import (
	"github.com/dedis/onet"
	"github.com/dedis/protobuf"

	"github.com/dedis/cothority/skipchain"
)

// NewSkipchain creates a new skipchain for a given roster and verification function.
func NewSkipchain(roster *onet.Roster, verifier []skipchain.VerifierID, data interface{}) (
	*skipchain.SkipBlock, error) {
	client := skipchain.NewClient()
	return client.CreateGenesis(roster, 8, 4, verifier, data, nil)
}

// Store appends a new block holding data to an existing skipchain.
func Store(id skipchain.SkipBlockID, roster *onet.Roster, transaction *Transaction) error {
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
