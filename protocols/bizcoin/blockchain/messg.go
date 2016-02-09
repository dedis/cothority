package blockchain

import (
	"bytes"
	"encoding/json"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/bitcosi/blkparser"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
)

// Default port for the conode-setup - the stamping-request port
// is at ```DefaultPort + 1```
var DefaultPort int = 2000

type MessageType int

type SeqNo byte

const (
	Error MessageType = iota
	TransactionAnnouncmentType
	BlockReplyType
	BlockRequestType
	KeyBlockRequestType
	BitCoSiClose
	BitCoSiExit
)

type TransactionAnnouncment struct {
	Val blkparser.Tx // Trasaction to be included in a block
}

// NOT: In order to decoe correctly the Proof, we need to the get the suite
// somehow. We could just simply add it as a field and not (un)marhsal it
// We'd just make sure that the suite is setup before unmarshaling.
type BlockReply struct {
	SuiteStr      string
	Timestamp     int64           // The timestamp requested for the block to prove its ordering
	BlockLen      int             // Length of Block
	Block         Block           // The Block including a number of transactions
	MerkleRoot    []byte          // root of the merkle tree
	PrfLen        int             // Length of proof
	Prf           proof.Proof     // Merkle proof of value
	Response      abstract.Secret // Aggregate response
	Challenge     abstract.Secret // Aggregate challenge
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
	//dbg.Print("Starting marshalling")
	suite := app.GetSuite(sr.SuiteStr)
	//dbg.Print("Preparing abstracts")
	if err := suite.Write(&b, sr.Response, sr.Challenge, sr.AggCommit, sr.AggPublic); err != nil {
		dbg.Lvl1("encoding stampreply response/challenge/AggCommit:", err)
		return nil, err
	}

	//dbg.Print("Returning helper-struct")
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
	//dbg.Print("Starting unmarshal")
	suite := app.GetSuite(sr.SuiteStr)
	aux := &struct {
		SignatureInfo []byte
		Response      abstract.Secret
		Challenge     abstract.Secret
		AggCommit     abstract.Point
		AggPublic     abstract.Point
		*Alias
	}{
		Response:  suite.Secret(),
		Challenge: suite.Secret(),
		AggCommit: suite.Point(),
		AggPublic: suite.Point(),
		Alias:     (*Alias)(sr),
	}
	//dbg.Print("Doing JSON unmarshal")
	if err := json.Unmarshal(dataJSON, &aux); err != nil {
		dbg.Print("Error in unmarshal:", err)
		return err
	}
	//dbg.Print("Preparing suites")
	if err := suite.Read(bytes.NewReader(aux.SignatureInfo), &sr.Response, &sr.Challenge, &sr.AggCommit, &sr.AggPublic); err != nil {
		dbg.Fatal("decoding signature Response / Challenge / AggCommit: ", err)
		return err
	}
	//dbg.Print("Finished")
	return nil
}

func (Treq BlockReply) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}
func (Treq *BlockReply) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}

func (tsm BitCoSiMessage) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}

func (sm *BitCoSiMessage) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}
