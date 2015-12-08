package conode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"strings"
)

/*
All messages for stamper-related actions
*/

// struct to ease keeping track of who requires a reply after
// tsm is processed/ aggregated by the TSServer
type MustReplyMessage struct {
	Tsm TimeStampMessage
	To  string // name of reply destination
}

// Default port for the conode-setup - the stamping-request port
// is at ```DefaultPort + 1```
var DefaultPort int = 2000

type MessageType int

type SeqNo byte

const (
	Error MessageType = iota
	StampRequestType
	StampSignatureType
	StampClose
	StampExit
)

type StampRequest struct {
	Val []byte // Hash-size value to timestamp
}

// NOTE: In order to decode correctly the Proof, we need to the get the suite
// somehow. We could just simply add it as a field and not (un)marhsal it
// We'd just make sure that the suite is setup before unmarshaling.
type StampSignature struct {
	SuiteStr      string
	Timestamp     int64            // The timestamp requested for the file
	MerkleRoot    []byte           // root of the merkle tree
	Prf           proof.Proof      // Merkle proof for the value sent to be stamped
	Response      abstract.Secret  // Aggregate response
	Challenge     abstract.Secret  // Aggregate challenge
	AggCommit     abstract.Point   // Aggregate commitment key
	AggPublic     abstract.Point   // Aggregate public key (use for easy troubleshooting)
	ExceptionList []abstract.Point // challenge from root
}

func (Sreq StampRequest) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do MarshalBinary on StampRequest")
	return nil, nil
}
func (Sreq *StampRequest) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do UnamrshalBinary on StampRequest")
	return nil
}

func (sr *StampSignature) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	suite := app.GetSuite(sr.SuiteStr)
	if err := suite.Write(&b, sr.Response, sr.Challenge, sr.AggCommit, sr.AggPublic,
		sr.ExceptionList); err != nil {
		dbg.Lvl1("encoding stampreply response/challenge/AggCommit:", err)
		return nil, err
	}

	return json.Marshal(&struct {
		BinaryBlob      []byte
		ExceptionLength int
		Prf             proof.Proof
		Timestamp       int64
		MerkleRoot      []byte
	}{
		BinaryBlob:      b.Bytes(),
		ExceptionLength: len(sr.ExceptionList),
		Prf:             sr.Prf,
		Timestamp:       sr.Timestamp,
		MerkleRoot:      sr.MerkleRoot,
	})
}

func (sr *StampSignature) UnmarshalJSON(dataJSON []byte) error {
	suite := app.GetSuite(sr.SuiteStr)
	aux := &struct {
		BinaryBlob      []byte
		ExceptionLength int
		Prf             proof.Proof
		Timestamp       int64
		MerkleRoot      []byte
	}{}

	if err := json.Unmarshal(dataJSON, &aux); err != nil {
		return err
	}
	sr.ExceptionList = make([]abstract.Point, aux.ExceptionLength)
	sr.Response = suite.Secret()
	sr.Challenge = suite.Secret()
	sr.AggCommit = suite.Point()
	sr.AggPublic = suite.Point()
	if err := suite.Read(bytes.NewReader(aux.BinaryBlob), &sr.Response,
		&sr.Challenge, &sr.AggCommit, &sr.AggPublic, &sr.ExceptionList); err != nil {
		dbg.Fatal("decoding signature Response / Challenge / AggCommit:", err)
		return err
	}
	return nil
}

// sigFile represnets a signature to be written to a file or to be read in a
// human readble format (TOML + base64 encoding)
type sigFile struct {
	SuiteStr      string
	Name          string
	Timestamp     int64
	Proof         []string
	MerkleRoot    string
	Challenge     string
	Response      string
	AggCommitment string
	AggPublic     string
}

// Write will write the struct in a human readable format into this writer
// The format is TOML and most fields are written in base64
func (sr *StampSignature) Save(file string) error {
	var p []string
	for _, pr := range sr.Prf {
		p = append(p, base64.StdEncoding.EncodeToString(pr))
	}
	suite := app.GetSuite(sr.SuiteStr)
	// Write challenge and response + commitment part
	var bufChall bytes.Buffer
	var bufResp bytes.Buffer
	var bufCommit bytes.Buffer
	var bufPublic bytes.Buffer
	if err := cliutils.WriteSecret64(suite, &bufChall, sr.Challenge); err != nil {
		return fmt.Errorf("Could not write secret challenge:", err)
	}
	if err := cliutils.WriteSecret64(suite, &bufResp, sr.Response); err != nil {
		return fmt.Errorf("Could not write secret response:", err)
	}
	if err := cliutils.WritePub64(suite, &bufCommit, sr.AggCommit); err != nil {
		return fmt.Errorf("Could not write aggregated commitment:", err)
	}
	if err := cliutils.WritePub64(suite, &bufPublic, sr.AggPublic); err != nil {
		return fmt.Errorf("Could not write aggregated public key:", err)
	}
	// Signature file struct containing everything needed
	sigStr := &sigFile{
		Name:          file,
		SuiteStr:      suite.String(),
		Timestamp:     sr.Timestamp,
		Proof:         p,
		MerkleRoot:    base64.StdEncoding.EncodeToString(sr.MerkleRoot),
		Challenge:     bufChall.String(),
		Response:      bufResp.String(),
		AggCommitment: bufCommit.String(),
		AggPublic:     bufPublic.String(),
	}

	// Print to the screen, and write to file
	dbg.Lvl2("Signature-file will be:\n%+v", sigStr)

	app.WriteTomlConfig(sigStr, file)
	return nil
}

func (sr *StampSignature) Open(file string) error {
	// Read in the toml-file
	sigStr := &sigFile{}
	err := app.ReadTomlConfig(sigStr, file)
	if err != nil {
		return err
	}
	suite := app.GetSuite(sigStr.SuiteStr)

	sr.Timestamp = sigStr.Timestamp
	for _, pr := range sigStr.Proof {
		pro, err := base64.StdEncoding.DecodeString(pr)
		if err != nil {
			dbg.Lvl1("Couldn't decode proof:", pr)
			return err
		}
		sr.Prf = append(sr.Prf, pro)
	}
	// Read the root, the challenge and response
	sr.MerkleRoot, err = base64.StdEncoding.DecodeString(sigStr.MerkleRoot)
	if err != nil {
		fmt.Errorf("Could not decode Merkle Root from sig file:", err)
	}
	sr.Response, err = cliutils.ReadSecret64(suite, strings.NewReader(sigStr.Response))
	if err != nil {
		fmt.Errorf("Could not read secret challenge:", err)
	}
	if sr.Challenge, err = cliutils.ReadSecret64(suite, strings.NewReader(sigStr.Challenge)); err != nil {
		fmt.Errorf("Could not read the aggregate commitment:", err)
	}
	if sr.AggCommit, err = cliutils.ReadPub64(suite, strings.NewReader(sigStr.AggCommitment)); err != nil {
		return err
	}
	if sr.AggPublic, err = cliutils.ReadPub64(suite, strings.NewReader(sigStr.AggPublic)); err != nil {
		return err
	}

	return nil
}

func (Sreq StampSignature) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do MarshalBinary on StampReply")
	return nil, nil
}
func (Sreq *StampSignature) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do UnarmsahlBinary on StampReply")
	return nil
}

type TimeStampMessage struct {
	ReqNo SeqNo // Request sequence number
	// ErrorReply *ErrorReply // Generic error reply to any request
	Type MessageType
	Sreq *StampRequest
	Srep *StampSignature
}

func (tsm TimeStampMessage) MarshalBinary() ([]byte, error) {
	dbg.Fatal("Don't want to do that")
	return nil, nil
}

func (sm *TimeStampMessage) UnmarshalBinary(data []byte) error {
	dbg.Fatal("Don't want to do that")
	return nil
}
