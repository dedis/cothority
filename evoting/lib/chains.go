package lib

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

func NewSkipchain(roster *onet.Roster, verifier []skipchain.VerifierID, data interface{}) (
	*skipchain.SkipBlock, error) {
	client := skipchain.NewClient()
	return client.CreateGenesis(roster, 1, 1, verifier, data, nil)
}

func Store(id skipchain.SkipBlockID, roster *onet.Roster, data ...interface{}) error {
	client := skipchain.NewClient()
	reply, err := client.GetUpdateChain(roster, id)
	if err != nil {
		return err
	}

	for _, d := range data {
		_, err = client.StoreSkipBlock(reply.Update[len(reply.Update)-1], nil, d)
		if err != nil {
			return err
		}
	}
	return nil
}
