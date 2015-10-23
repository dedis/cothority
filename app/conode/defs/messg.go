package defs

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


type JSONdata struct {
	Len  int64
	Data []byte
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

/*
func (Srep StampReply) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	var err error
	if err = enc.Encode(Srep.Timestamp); err != nil {
		dbg.Lvl1("encoding stampreply timestamp failed : ", err)
	}
	if err = enc.Encode(Srep.MerkleRoot); err != nil {
		dbg.Lvl1("encoding stampreply merkleroot : ", err)
		return nil, err
	}
	if err = enc.Encode(len(Srep.Prf)); err != nil {
		dbg.Lvl1("encoding stampreply proof length:", err)
		return nil, err
	}
	if err = enc.Encode(Srep.Prf); err != nil {
		dbg.Lvl1("encoding stampreply proof :", err)
		return nil, err
	}
	if err = enc.Encode(Srep.SuiteStr); err != nil {
		dbg.Lvl1("encoding stampreply suite string : ", err)
		return nil, err
	}
	suite := app.GetSuite(Srep.SuiteStr)
	if err = suite.Write(&b, Srep.SigBroad); err != nil {
		dbg.Lvl1("encoding stampreply signature broadcast :", err)
		return nil, err
	}
	return b.Bytes(), err
}

func (Srep *StampReply) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	var err error
	if err = dec.Decode(&Srep.Timestamp); err != nil {
		dbg.Fatal("Decoding stampreply timestamp failed :", err)
	}
	if err = dec.Decode(&Srep.MerkleRoot); err != nil {
		dbg.Fatal("decoding stampreply merkle root : ", err)
	}

	if err = dec.Decode(&Srep.PrfLen); err != nil {
		dbg.Fatal("decoding stampreply proof len :", err)
	}
	Srep.Prf = make([]hashid.HashId, Srep.PrfLen)
	if err = dec.Decode(&Srep.Prf); err != nil {
		dbg.Fatal("decoding stampreply proof :", err)
	}
	if err = dec.Decode(&Srep.SuiteStr); err != nil {
		dbg.Fatal("decoding suite : ", err)
	}
	suite := app.GetSuite(Srep.SuiteStr)
	Srep.SigBroad = sign.SignatureBroadcastMessage{}
	if err = suite.Read(b, &Srep.SigBroad); err != nil {
		dbg.Fatal("decoding signature broadcast : ", err)
	}
	return nil
}
*/

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

/*
func (tsm TimeStampMessage) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	var sub []byte
	var err error
	b.WriteByte(byte(tsm.Type))
	b.WriteByte(byte(tsm.ReqNo))
	// marshal sub message based on its Type
	switch tsm.Type {
	case StampRequestType:
		//sub, err = tsm.Sreq.MarshalBinary()
	case StampReplyType:
		sub, err = tsm.Srep.MarshalBinary()
	}
	if err == nil {
		b.Write(sub)
	}
	return b.Bytes(), err
}

func (sm *TimeStampMessage) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	sm.Type = MessageType(data[0])
	sm.ReqNo = SeqNo(data[1])
	msgBytes := data[2:]
	var err error
	switch sm.Type {
	case StampRequestType:
		//sm.Sreq = &StampRequest{}
		//err = sm.Sreq.UnmarshalBinary(msgBytes)
	case StampReplyType:
		sm.Srep = &StampReply{}
		err = sm.Srep.UnmarshalBinary(msgBytes)

	}
	return err
}

func (tsm *TimeStampMessage) MarshalJSON() ([]byte, error) {
	data, _ := tsm.MarshalBinary()
	j, err := json.Marshal(JSONdata{
		Len: int64(len(data)),
		Data: data,
	})
	dbg.Printf("%s", hex.EncodeToString(j))
	return j, err
}

func (tsm *TimeStampMessage) UnmarshalJSON(dataJSON []byte) error {
	jdata := JSONdata{}
	dbg.Printf("%s", hex.EncodeToString(dataJSON))
	json.Unmarshal(dataJSON, &jdata)
	return tsm.UnmarshalBinary(jdata.Data)
}
*/
