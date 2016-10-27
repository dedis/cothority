package blockchain

import (
	"bytes"
	"encoding/json"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/suites"
)

type MessageType int

type SeqNo byte

type TransactionAnnouncment struct {
	Val blkparser.Tx // Trasaction to be included in a block
}

// NOTE: In order to decode correctly the proof, we need to the get the suite
// somehow. We could just simply add it as a field and not (un)marhsal it
// We'd just make sure that the suite is setup before unmarshaling.
type BlockReply struct {
	SuiteStr      string
	Timestamp     int64           // The timestamp requested for the block to prove its ordering
	BlockLen      int             // Length of Block
	Block         Block           // The Block including a number of transactions
	MerkleRoot    []byte          // root of the merkle tree
	PrfLen        int             // Length of proof
	Prf           crypto.Proof    // Merkle proof of value
	Response      abstract.Scalar // Aggregate response
	Challenge     abstract.Scalar // Aggregate challenge
	AggCommit     abstract.Point  // Aggregate commitment key
	AggPublic     abstract.Point  // Aggregate public key (use for easy troubleshooting)
	SignatureInfo []byte          // All other elements necessary
}

type BitCoSiMessage struct {
	ReqNo SeqNo // Request sequence number
	// ErrorReply *ErrorReply // Generic error reply to any request
	Type MessageType
	Treq *TransactionAnnouncment
	Brep *BlockReply
}

func (sr *BlockReply) MarshalJSON() ([]byte, error) {
	type Alias BlockReply
	var b bytes.Buffer
	suite, err := suites.StringToSuite(sr.SuiteStr)
	if err != nil {
		return nil, err
	}
	//log.Print("Preparing abstracts")
	if err := suite.Write(&b, sr.Response, sr.Challenge, sr.AggCommit, sr.AggPublic); err != nil {
		log.Lvl1("encoding stampreply response/challenge/AggCommit:", err)
		return nil, err
	}

	//log.Print("Returning helper-struct")
	return json.Marshal(&struct {
		SignatureInfo []byte
		*Alias
	}{
		SignatureInfo: b.Bytes(),
		Alias:         (*Alias)(sr),
	})
}

func (sr *BlockReply) UnmarshalJSON(dataJSON []byte) error {
	type Alias BlockReply
	//log.Print("Starting unmarshal")
	suite, err := suites.StringToSuite(sr.SuiteStr)
	if err != nil {
		return err
	}
	aux := &struct {
		SignatureInfo []byte
		Response      abstract.Scalar
		Challenge     abstract.Scalar
		AggCommit     abstract.Point
		AggPublic     abstract.Point
		*Alias
	}{
		Response:  suite.Scalar(),
		Challenge: suite.Scalar(),
		AggCommit: suite.Point(),
		AggPublic: suite.Point(),
		Alias:     (*Alias)(sr),
	}
	//log.Print("Doing JSON unmarshal")
	if err := json.Unmarshal(dataJSON, &aux); err != nil {
		return err
	}
	if err := suite.Read(bytes.NewReader(aux.SignatureInfo), &sr.Response, &sr.Challenge, &sr.AggCommit, &sr.AggPublic); err != nil {
		log.Fatal("decoding signature Response / Challenge / AggCommit: ", err)
		return err
	}
	return nil
}

func (Treq BlockReply) MarshalBinary() ([]byte, error) {
	log.Fatal("Don't want to do that")
	return nil, nil
}
func (Treq *BlockReply) UnmarshalBinary(data []byte) error {
	log.Fatal("Don't want to do that")
	return nil
}

func (tsm BitCoSiMessage) MarshalBinary() ([]byte, error) {
	log.Fatal("Don't want to do that")
	return nil, nil
}

func (sm *BitCoSiMessage) UnmarshalBinary(data []byte) error {
	log.Fatal("Don't want to do that")
	return nil
}
