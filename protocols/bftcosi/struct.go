package bftcosi

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"bytes"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/cothority/sda"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	/*	for _, msg := range []interface{}{
			CountRequest{}, CountResponse{},
			ClockRequest{}, ClockResponse{},
		} {
			network.RegisterPacketType(msg)
		}*/
}

type MicroBlock struct {
	*blockchain.TrBlock
	BlockSig *BFTSignature
	Roster   *sda.Roster
	Priority int
}

// VerifySignatures returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sb *MicroBlock) VerifySignatures() error {
	if err := sb.BlockSig.Verify(network.Suite, sb.Roster.Publics()); err != nil {
		log.Error(err.Error() + log.Stack())
		return err
	}
	//for _, fl := range sb.ForwardLink {
	//	if err := fl.VerifySignature(sb.Aggregate); err != nil {
	//		return err
	//	}
	//}
	//if sb.ChildSL != nil && sb.ChildSL.Hash == nil {
	//	return sb.ChildSL.VerifySignature(sb.Aggregate)
	//}
	return nil
}

// Equal returns bool if both hashes are equal
func (sb *MicroBlock) Equal(other *MicroBlock) bool {
	return bytes.Equal([]byte(sb.HeaderHash), []byte(other.HeaderHash))
}
