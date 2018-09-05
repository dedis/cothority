package contracts

import "github.com/dedis/onet/network"

func init() {
	network.RegisterMessages()
}

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :darc.ID:bytes
// type :Arguments:[]Argument
// type :Instructions:[]Instruction
// type :ClientTransactions:[]ClientTransaction
// type :InstanceID:bytes
// package omnicontracts;
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "OmniLedgerContracts";

type CoinInstance struct {
	// Type denotes what coin this is. Every coin can have a type, and only
	// compatible coins can be directly transferred. For incompatible coins,
	// you need an exchange (not yet implemented).
	Type []byte
	// Balance holds how many coins are stored in this account. It can only
	// be positive.
	Balance uint64
}
