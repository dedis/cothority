package conode

import (
	"bytes"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/proto/sign"
	"encoding/json"
)

type MessageType int

type SeqNo byte

const (
	Error MessageType = iota
	StampRequestType
	StampReplyType
	StampClose
)

type StampRequest struct {
	Val []byte // Hash-size value to timestamp
}

// NOTE: In order to decoe correctly the Proof, we need to the get the suite
// somehow. We could just simply add it as a field and not (un)marhsal it
// We'd just make sure that the suite is setup before unmarshaling.
type StampReply struct {
	SuiteStr   string
	Timestamp  int64                          // The timestamp requested for the file
	MerkleRoot []byte                         // root of the merkle tree
	PrfLen     int                            // Length of proof
	Prf        proof.Proof                    // Merkle proof of value
	SigBroad   sign.SignatureBroadcastMessage // All other elements necessary
}

func (Sreq StampRequest) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}
func (Sreq *StampRequest) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}

func (sr *StampReply) MarshalJSON() ([]byte, error) {
	type Alias StampReply
	var b bytes.Buffer
	suite := app.GetSuite(sr.SuiteStr)
	if err := suite.Write(&b, sr.SigBroad); err != nil {
		dbg.Lvl1("encoding stampreply signature broadcast :", err)
		return nil, err
	}

	return json.Marshal(&struct {
		SigBroad []byte
		*Alias
	}{
		SigBroad: b.Bytes(),
		Alias:    (*Alias)(sr),
	})
}

func (sr *StampReply) UnmarshalJSON(dataJSON []byte) error {
	type Alias StampReply
	aux := &struct {
		SigBroad []byte
		*Alias
	}{
		Alias: (*Alias)(sr),
	}
	if err := json.Unmarshal(dataJSON, &aux); err != nil {
		return err
	}
	suite := app.GetSuite(sr.SuiteStr)
	sr.SigBroad = sign.SignatureBroadcastMessage{}
	if err := suite.Read(bytes.NewReader(aux.SigBroad), &sr.SigBroad); err != nil {
		dbg.Fatal("decoding signature broadcast : ", err)
		return err
	}
	return nil
}

func (Sreq StampReply) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}
func (Sreq *StampReply) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}

type TimeStampMessage struct {
	ReqNo SeqNo // Request sequence number
				// ErrorReply *ErrorReply // Generic error reply to any request
	Type  MessageType
	Sreq  *StampRequest
	Srep  *StampReply
}

func (tsm TimeStampMessage) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}

func (sm *TimeStampMessage) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}
