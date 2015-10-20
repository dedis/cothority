package defs

import (
	"bytes"
	"encoding/gob"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/proto/sign"
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
	MerkleRoot []byte                         // root of the merkle tree
	PrfLen     int                            // Length of proof
	Prf        proof.Proof                    // Merkle proof of value
	SigBroad   sign.SignatureBroadcastMessage // All other elements necessary
}

func (Sreq StampRequest) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(Sreq.Val)
	return b.Bytes(), err
}

func (Sreq *StampRequest) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&Sreq.Val)
	return err
}

func (Srep StampReply) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	var err error
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
	dbg.Print("Stampreply decoding suite after: ", suite)
	Srep.SigBroad = sign.SignatureBroadcastMessage{}
	if err = suite.Read(b, &Srep.SigBroad); err != nil {
		dbg.Fatal("decoding signature broadcast : ", err)
	}
	return nil
}

type TimeStampMessage struct {
	ReqNo SeqNo // Request sequence number
	// ErrorReply *ErrorReply // Generic error reply to any request
	Type MessageType
	Sreq *StampRequest
	Srep *StampReply
}

func (tsm TimeStampMessage) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	var sub []byte
	var err error
	b.WriteByte(byte(tsm.Type))
	b.WriteByte(byte(tsm.ReqNo))
	// marshal sub message based on its Type
	switch tsm.Type {
	case StampRequestType:
		sub, err = tsm.Sreq.MarshalBinary()
	case StampReplyType:
		sub, err = tsm.Srep.MarshalBinary()
	}
	if err == nil {
		b.Write(sub)
	}
	return b.Bytes(), err
}

func (sm *TimeStampMessage) UnmarshalBinary(data []byte) error {
	sm.Type = MessageType(data[0])
	sm.ReqNo = SeqNo(data[1])
	msgBytes := data[2:]
	var err error
	switch sm.Type {
	case StampRequestType:
		sm.Sreq = &StampRequest{}
		err = sm.Sreq.UnmarshalBinary(msgBytes)
	case StampReplyType:
		sm.Srep = &StampReply{}
		err = sm.Srep.UnmarshalBinary(msgBytes)

	}
	return err
}
