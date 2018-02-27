package lib

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

var client = skipchain.NewClient()

// New creates a new skipchain for a given roster and stores data in the genesis block.
func New(roster *onet.Roster, data interface{}) (*skipchain.SkipBlock, error) {
	return client.CreateGenesis(roster, 1, 1, skipchain.VerificationStandard, data, nil)
}

// chain returns a skipchain for a given id.
func chain(roster *onet.Roster, id skipchain.SkipBlockID) ([]*skipchain.SkipBlock, error) {
	chain, err := client.GetUpdateChain(roster, id)
	if err != nil {
		return nil, err
	}
	return chain.Update, nil
}
