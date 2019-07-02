package byzcoin

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"go.dedis.ch/onet/v3/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		&GetAllByzCoinIDsRequest{}, &GetAllByzCoinIDsResponse{},
		&CreateGenesisBlock{}, &CreateGenesisBlockResponse{},
		&AddTxRequest{}, &AddTxResponse{},
		&GetSignerCounters{}, &GetSignerCountersResponse{},
	)
}

// Version indicates what version this client runs. In the first development
// phase, each next version will break the preceeding versions. Later on,
// new versions might correctly interpret earlier versions.
type Version int

// CurrentVersion is what we're running now
const CurrentVersion Version = 1
