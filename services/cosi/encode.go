package cosi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/network"
)

// MarshalJSON implements golang's JSON marshal interface
func (s *SignatureResponse) MarshalJSON() ([]byte, error) {
	cw := new(bytes.Buffer)
	rw := new(bytes.Buffer)

	err := crypto.WriteSecret64(network.Suite, cw, s.Challenge)
	if err != nil {
		return nil, err
	}
	err = crypto.WriteSecret64(network.Suite, rw, s.Response)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Sum       string
		Challenge string
		Response  string
	}{
		Sum:       base64.StdEncoding.EncodeToString(s.Sum),
		Challenge: cw.String(),
		Response:  rw.String(),
	})
}

// UnmarshalJSON implements golang's JSON unmarshal interface
func (s *SignatureResponse) UnmarshalJSON(data []byte) error {
	type Aux struct {
		Sum       string
		Challenge string
		Response  string
	}
	aux := &Aux{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var err error
	if s.Sum, err = base64.StdEncoding.DecodeString(aux.Sum); err != nil {
		return err
	}
	suite := network.Suite
	cr := strings.NewReader(aux.Challenge)
	if s.Challenge, err = crypto.ReadSecret64(suite, cr); err != nil {
		return err
	}
	rr := strings.NewReader(aux.Response)
	if s.Response, err = crypto.ReadSecret64(suite, rr); err != nil {
		return err
	}
	return nil
}
