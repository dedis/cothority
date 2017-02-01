package service

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/base64"
	"gopkg.in/dedis/crypto.v0/eddsa"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

const (
	// ErrorWrongPIN indicates a wrong PIN
	ErrorWrongPIN = iota + 4100
	// ErrorInternal indicates something internally went wrong - see the
	// error message
	ErrorInternal
	// ErrorOtherFinals indicates that one or more of the other conodes
	// are still missing the finalization-step
	ErrorOtherFinals
)

func init() {
	network.RegisterMessage(&FinalStatement{})
	network.RegisterMessage(&PopDesc{})
}

// Client is a structure to communicate with any app that wants to use our
// service.
type Client struct {
	*onet.Client
}

// NewClient instantiates a new Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(Name)}
}

// PinRequest takes a destination-address, a PIN and a public key as an argument.
// If no PIN is given, the cothority will print out a "PIN: ...."-line on the stdout.
// If the PIN is given and is correct, the public key will be stored in the
// service.
func (c *Client) PinRequest(dst network.Address, pin string, pub abstract.Point) onet.ClientError {
	si := &network.ServerIdentity{Address: dst}
	return c.SendProtobuf(si, &PinRequest{pin, pub}, nil)
}

// StoreConfig sends the configuration to the conode for later usage.
func (c *Client) StoreConfig(dst network.Address, p *PopDesc) onet.ClientError {
	si := &network.ServerIdentity{Address: dst}
	err := c.SendProtobuf(si, &StoreConfig{p}, nil)
	if err != nil {
		return err
	}
	return nil
}

// Finalize takes the address of the conode-server, a pop-description and a
// list of attendees public keys. It contacts the other conodes and checks
// if they are available and already have a description. If so, all attendees
// not in all the conodes will be stripped, and that new pop-description
// collectively signed. The new pop-description and the final statement
// will be returned.
func (c *Client) Finalize(dst network.Address, p *PopDesc, attendees []abstract.Point) (
	*FinalStatement, onet.ClientError) {
	si := &network.ServerIdentity{Address: dst}
	res := &FinalizeResponse{}
	err := c.SendProtobuf(si, &FinalizeRequest{p.Hash(), attendees}, res)
	if err != nil {
		return nil, err
	}
	return res.Final, nil
}

// FinalStatement is the final configuration holding all data necessary
// for a verifier.
type FinalStatement struct {
	// Desc is the description of the pop-party.
	Desc *PopDesc
	// Attendees holds a slice of all public keys of the attendees.
	Attendees []abstract.Point
	// Signature is created by all conodes responsible for that pop-party
	Signature []byte
}

// The toml-structure for (un)marshaling with toml
type finalStatementToml struct {
	Desc      *popDescToml
	Attendees []string
	Signature string
}

// NewFinalStatementFromToml creates a final statement from a toml slice-of-bytes.
func NewFinalStatementFromToml(b []byte) (*FinalStatement, error) {
	fsToml := &finalStatementToml{}
	_, err := toml.Decode(string(b), fsToml)
	if err != nil {
		return nil, err
	}
	sis := []*network.ServerIdentity{}
	for _, s := range fsToml.Desc.Roster {
		uid, err := uuid.FromString(s[2])
		if err != nil {
			return nil, err
		}
		pub, err := crypto.String64ToPub(network.Suite, s[3])
		if err != nil {
			return nil, err
		}
		sis = append(sis, &network.ServerIdentity{
			Address:     network.Address(s[0]),
			Description: s[1],
			ID:          network.ServerIdentityID(uid),
			Public:      pub,
		})
	}
	rostr := onet.NewRoster(sis)
	desc := &PopDesc{
		Name:     fsToml.Desc.Name,
		DateTime: fsToml.Desc.DateTime,
		Location: fsToml.Desc.Location,
		Roster:   rostr,
	}
	atts := []abstract.Point{}
	for _, p := range fsToml.Attendees {
		pub, err := crypto.String64ToPub(network.Suite, p)
		if err != nil {
			return nil, err
		}
		atts = append(atts, pub)
	}
	sig := make([]byte, 64)
	sig, err = base64.StdEncoding.DecodeString(fsToml.Signature)
	// TODO: sign and verify signature
	if err != nil {
		return nil, err
	}
	return &FinalStatement{
		Desc:      desc,
		Attendees: atts,
		Signature: sig,
	}, nil
}

// ToToml returns a toml-slice of byte and an eventual error.
func (fs *FinalStatement) ToToml() ([]byte, error) {
	rostr := [][]string{}
	for _, si := range fs.Desc.Roster.List {
		str, err := crypto.PubToString64(nil, si.Public)
		if err != nil {
			return nil, err
		}
		sistr := []string{si.Address.String(), si.Description,
			uuid.UUID(si.ID).String(), str}
		rostr = append(rostr, sistr)
	}
	descToml := &popDescToml{
		Name:     fs.Desc.Name,
		DateTime: fs.Desc.DateTime,
		Location: fs.Desc.Location,
		Roster:   rostr,
	}
	atts := []string{}
	for _, p := range fs.Attendees {
		str, err := crypto.PubToString64(nil, p)
		if err != nil {
			return nil, err
		}
		atts = append(atts, str)
	}
	fsToml := &finalStatementToml{
		Desc:      descToml,
		Attendees: atts,
		Signature: base64.StdEncoding.EncodeToString(fs.Signature),
	}
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(fsToml)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Hash returns the hash of the popdesc and the attendees. In case of an error
// in the hashing it will return a nil-slice and the error.
func (fs *FinalStatement) Hash() ([]byte, error) {
	h := network.Suite.Hash()
	_, err := h.Write(fs.Desc.Hash())
	if err != nil {
		return nil, err
	}
	for _, a := range fs.Attendees {
		b, err := a.MarshalBinary()
		if err != nil {
			return nil, err
		}
		_, err = h.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// Verify checks if the collective signature is correct and has been created
// by the roster. On success, this returns nil.
func (fs *FinalStatement) Verify() error {
	h, err := fs.Hash()
	if err != nil {
		return err
	}
	return eddsa.Verify(fs.Desc.Roster.Aggregate, h, fs.Signature)
}

// PopDesc holds the name, date and a roster of all involved conodes.
type PopDesc struct {
	// Name and purpose of the party.
	Name string
	// DateTime of the party. It is in the following format, following UTC:
	//   YYYY-MM-DD HH:mm
	DateTime string
	// Location of the party
	Location string
	// Roster of all responsible conodes for that party.
	Roster *onet.Roster
}

// represents a PopDesc in string-version for toml.
type popDescToml struct {
	Name     string
	DateTime string
	Location string
	Roster   [][]string
}

// Hash of this structure - calculated by hand instead of using network.Marshal.
func (p *PopDesc) Hash() []byte {
	hash := network.Suite.Hash()
	hash.Write([]byte(p.Name))
	hash.Write([]byte(p.DateTime))
	hash.Write([]byte(p.Location))
	buf, err := p.Roster.Aggregate.MarshalBinary()
	if err != nil {
		log.Error(err)
		return []byte{}
	}
	hash.Write([]byte(buf))
	return hash.Sum(nil)
}
