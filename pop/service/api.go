package service

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/base64"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/satori/go.uuid"
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
	network.RegisterPacketType(&FinalStatement{})
	network.RegisterPacketType(&PopDesc{})
}

// FinalStatement is the final configuration holding all data necessary
// for a verifier.
type FinalStatement struct {
	Desc      *PopDesc
	Attendees []abstract.Point
	Signature []byte
}

type finalStatementToml struct {
	Desc      *popDescToml
	Attendees []string
	Signature string
}

// NewFinalStatementFromString creates a final statement from a string
func NewFinalStatementFromString(s string) *FinalStatement {
	fsToml := &finalStatementToml{}
	_, err := toml.Decode(s, fsToml)
	if err != nil {
		log.Error(err)
		return nil
	}
	sis := []*network.ServerIdentity{}
	for _, s := range fsToml.Desc.Roster {
		uid, err := uuid.FromString(s[2])
		if err != nil {
			log.Error(err)
			return nil
		}
		sis = append(sis, &network.ServerIdentity{
			Address:     network.Address(s[0]),
			Description: s[1],
			ID:          network.ServerIdentityID(uid),
			Public:      B64ToPoint(s[3]),
		})
	}
	rostr := onet.NewRoster(sis)
	desc := &PopDesc{
		Name:   fsToml.Desc.Name,
		Date:   fsToml.Desc.Date,
		Roster: rostr,
	}
	atts := []abstract.Point{}
	for _, p := range fsToml.Attendees {
		atts = append(atts, B64ToPoint(p))
	}
	sig := make([]byte, 64)
	sig, err = base64.StdEncoding.DecodeString(fsToml.Signature)
	if err != nil {
		log.Error(err)
		return nil
	}
	return &FinalStatement{
		Desc:      desc,
		Attendees: atts,
		Signature: sig,
	}
}

// PointToB64 converts an abstract.Point to a base64-point.
func PointToB64(p abstract.Point) string {
	pub, err := p.MarshalBinary()
	if err != nil {
		log.Error(err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(pub)
}

// B64ToPoint converts a base64-string to an abstract.Point.
func B64ToPoint(str string) abstract.Point {
	public := network.Suite.Point()
	buf, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.Error(err)
		return nil
	}
	err = public.UnmarshalBinary(buf)
	if err != nil {
		log.Error(err)
		return nil
	}
	return public
}

// ScalarToB64 converts an abstract.Scalar to a base64-string.
func ScalarToB64(s abstract.Scalar) string {
	sec, err := s.MarshalBinary()
	if err != nil {
		log.Error(err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(sec)
}

// B64ToScalar converts a base64-string to an abstract.Scalar.
func B64ToScalar(str string) abstract.Scalar {
	scalar := network.Suite.Scalar()
	buf, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.Error(err)
		return nil
	}
	err = scalar.UnmarshalBinary(buf)
	if err != nil {
		log.Error(err)
		return nil
	}
	return scalar
}

// ToToml returns a toml-string.
func (fs *FinalStatement) ToToml() string {
	rostr := [][]string{}
	for _, si := range fs.Desc.Roster.List {
		sistr := []string{si.Address.String(), si.Description,
			uuid.UUID(si.ID).String(), PointToB64(si.Public)}
		rostr = append(rostr, sistr)
	}
	descToml := &popDescToml{
		Name:   fs.Desc.Name,
		Date:   fs.Desc.Date,
		Roster: rostr,
	}
	atts := []string{}
	for _, p := range fs.Attendees {
		atts = append(atts, PointToB64(p))
	}
	fsToml := &finalStatementToml{
		Desc:      descToml,
		Attendees: atts,
		Signature: base64.StdEncoding.EncodeToString(fs.Signature),
	}
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(fsToml)
	if err != nil {
		return ""
	}
	return string(buf.Bytes())
}

// PopDesc holds the name, date and a roster of all involved conodes.
type PopDesc struct {
	Name   string
	Date   string
	Roster *onet.Roster
}

// represents a PopDesc in string-version for toml.
type popDescToml struct {
	Name   string
	Date   string
	Roster [][]string
}

// Hash calculates the hash of this structure
func (p *PopDesc) Hash() []byte {
	if p == nil {
		return nil
	}
	hash := network.Suite.Hash()
	hash.Write([]byte(p.Name))
	hash.Write([]byte(p.Date))
	buf, err := p.Roster.Aggregate.MarshalBinary()
	if err != nil {
		log.Error(err)
		return nil
	}
	hash.Write([]byte(buf))
	return hash.Sum(nil)
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

// Pin takes a destination-address, a PIN and a public key as an argument.
// If no PIN is given, the cothority will print out a "PIN: ...."-line on the stdout.
// If the PIN is given and is correct, the public key will be stored in the
// service.
func (c *Client) Pin(dst network.Address, pin string, pub abstract.Point) onet.ClientError {
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
