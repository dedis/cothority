package lib

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// StringToDarcID converts a string representation of a DARC to a byte array
func StringToDarcID(id string) ([]byte, error) {
	if id == "" {
		return nil, errors.New("no string given")
	}
	if strings.HasPrefix(id, "darc:") {
		id = id[5:]
	}
	return hex.DecodeString(id)
}

// StringToEd25519Buf converts the string representation of an ed25519 key to a
// byte array
func StringToEd25519Buf(pub string) ([]byte, error) {
	if pub == "" {
		return nil, errors.New("no string given")
	}
	if strings.HasPrefix(pub, "ed25519:") {
		pub = pub[8:]
	}
	return hex.DecodeString(pub)
}

// GetDarcByString returns a DARC given its ID as a string
func GetDarcByString(cl *byzcoin.Client, id string) (*darc.Darc, error) {
	xrep, err := StringToDarcID(id)
	if err != nil {
		return nil, err
	}
	return GetDarcByID(cl, xrep)
}

// GetDarcByID returns a DARC given its ID as a byte array
func GetDarcByID(cl *byzcoin.Client, id []byte) (*darc.Darc, error) {
	pr, err := cl.GetProofFromLatest(id)
	if err != nil {
		return nil, err
	}

	p := &pr.Proof

	vs, cid, _, err := p.Get(id)
	if err != nil {
		return nil, fmt.Errorf("could not find darc for %x", id)
	}
	if cid != byzcoin.ContractDarcID {
		return nil, fmt.Errorf("unexpected contract %v, expected a darc", cid)
	}

	d, err := darc.NewFromProtobuf(vs)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// ExportTransactionAndExit will redirect the transaction to stdout. If no errors are
// raised, this method exits the program with code 0.
func ExportTransactionAndExit(tx byzcoin.ClientTransaction) error {
	// When exporting, we must not pass SignerCounter, SignerIdentities and
	// Signatures. Hence, we build a new list of instructions by ommiting
	// those parameters. We can't edit current ones because those are not
	// pointers.
	instrs := make([]byzcoin.Instruction, len(tx.Instructions))
	for i, instr := range tx.Instructions {
		instrs[i] = byzcoin.Instruction{
			InstanceID: instr.InstanceID,
			Spawn:      instr.Spawn,
			Invoke:     instr.Invoke,
			Delete:     instr.Delete,
		}
	}
	tx.Instructions = instrs
	buf, err := protobuf.Encode(&tx)
	if err != nil {
		return errors.New("failed to encode tx: " + err.Error())
	}
	reader := bytes.NewReader(buf)
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.New("failed to copy to stdout: " + err.Error())
	}
	os.Exit(0)
	// Never reached...
	return nil
}

// FindRecursivefBool recursively check the context to find argname
func FindRecursivefBool(argname string, c *cli.Context) bool {
	for c != nil {
		if c.IsSet(argname) {
			return c.Bool(argname)
		}
		c = c.Parent()
	}
	return false
}
