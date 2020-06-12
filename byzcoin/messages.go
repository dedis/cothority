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
const CurrentVersion Version = VersionSpawnerCoins

const (
	// VersionInstructionHash is the first version and indicates that a new,
	// correct, hash is used for the instructions.
	VersionInstructionHash Version = iota + 1
	// VersionPersonhood is when the personhood contract has been repaired
	VersionPersonhood = 2
	// VersionPopParty indicates when the pop-parties started using a correct darc.
	VersionPopParty = 3
	// VersionViewchange removed the BLS-signature on the view-change requests
	VersionViewchange = 4
	// VersionPreID adds preID to most of the contracts
	VersionPreID = 5
	// VersionSpawnerCoins indicates a fixed spawner contract that will treat
	// the coins correctly
	VersionSpawnerCoins = 6
)
