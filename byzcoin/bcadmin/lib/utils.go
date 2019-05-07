package lib

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
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
	pr, err := cl.GetProof(id)
	if err != nil {
		return nil, err
	}

	p := &pr.Proof
	err = p.Verify(cl.ID)
	if err != nil {
		return nil, err
	}

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
