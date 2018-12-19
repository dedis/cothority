package service

import (
	"bytes"
	"encoding/hex"
	"errors"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/blscosi/protocol"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/satori/go.uuid.v1"
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
	return &Client{Client: onet.NewClient(cothority.Suite, Name)}
}

// PinRequest takes a destination-address, a PIN and a public key as an argument.
// If no PIN is given, the cothority will print out a "PIN: ...."-line on the stdout.
// If the PIN is given and is correct, the public key will be stored in the
// service.
func (c *Client) PinRequest(dst network.Address, pin string, pub kyber.Point) error {
	si := &network.ServerIdentity{Address: dst}
	return c.SendProtobuf(si, &PinRequest{pin, pub}, nil)
}

// VerifyLink checks if a given public key is in the list of administrators
// in the service. It returns a nil error if the key is present.
func (c *Client) VerifyLink(dst network.Address, pub kyber.Point) error {
	si := &network.ServerIdentity{Address: dst}
	rep := &VerifyLinkReply{}
	err := c.SendProtobuf(si, &VerifyLink{pub}, rep)
	if err != nil {
		return err
	}
	if rep.Exists {
		return nil
	}
	return errors.New("this public key is not stored")
}

// StoreConfig sends the configuration to the conode for later usage.
func (c *Client) StoreConfig(dst network.Address, p *PopDesc, priv kyber.Scalar) error {
	si := &network.ServerIdentity{Address: dst}
	sg, err := schnorr.Sign(cothority.Suite, priv, p.Hash())
	if err != nil {
		return err
	}
	err = c.SendProtobuf(si, &StoreConfig{p, sg}, nil)
	if err != nil {
		return err
	}
	return nil
}

// GetProposals asks the conode if there is any proposed description waiting
// to be confirmed.
func (c *Client) GetProposals(dst network.Address) ([]PopDesc, error) {
	si := &network.ServerIdentity{Address: dst}
	rep := &GetProposalsReply{}
	err := c.SendProtobuf(si, &GetProposals{}, rep)
	if err != nil {
		return nil, err
	}
	return rep.Proposals, nil
}

// FetchFinal sends Request to update local final statement
func (c *Client) FetchFinal(dst network.Address, hash []byte) (
	*FinalStatement, error) {
	si := &network.ServerIdentity{Address: dst}
	res := &FinalizeResponse{}
	err := c.SendProtobuf(si, &FetchRequest{ID: hash}, res)
	if err != nil {
		return nil, err
	}
	return res.Final, nil
}

// Finalize takes the address of the conode-server, a pop-description and a
// list of attendees public keys. It contacts the other conodes and checks
// if they are available and already have a description. If so, all attendees
// not in all the conodes will be stripped, and that new pop-description
// collectively signed. The new pop-description and the final statement
// will be returned.
func (c *Client) Finalize(dst network.Address, p *PopDesc, attendees []kyber.Point,
	priv kyber.Scalar) (*FinalStatement, error) {
	si := &network.ServerIdentity{Address: dst}
	req := &FinalizeRequest{}
	req.DescID = p.Hash()
	req.Attendees = attendees
	hash, err := req.Hash()
	if err != nil {
		return nil, err
	}
	res := &FinalizeResponse{}
	sg, err := schnorr.Sign(cothority.Suite, priv, hash)
	if err != nil {
		return nil, err
	}
	req.Signature = sg
	e := c.SendProtobuf(si, req, res)
	if e != nil {
		return nil, e
	}
	return res.Final, nil
}

// Merge takes the address of the conode-server, pop-description and the
// private key of organizer. It triggers merge process on nodes mentioned in
// config
func (c *Client) Merge(dst network.Address, p *PopDesc, priv kyber.Scalar) (
	*FinalStatement, error) {
	si := &network.ServerIdentity{Address: dst}
	res := &FinalizeResponse{}
	hash := p.Hash()
	sg, err := schnorr.Sign(cothority.Suite, priv, hash)
	if err != nil {
		return nil, err
	}

	e := c.SendProtobuf(si, &MergeRequest{hash, sg}, res)
	if e != nil {
		return nil, e
	}
	return res.Final, nil
}

// GetLink returns the link of the organizer, if available.
func (c *Client) GetLink(dst network.Address) (kyber.Point, error) {
	si := &network.ServerIdentity{Address: dst}
	res := &GetLinkReply{}

	e := c.SendProtobuf(si, &GetLink{}, res)
	if e != nil {
		return nil, e
	}
	return res.Public, nil
}

// GetFinalStatements returns a map of all final statements.
func (c *Client) GetFinalStatements(dst network.Address) (map[string]*FinalStatement, error) {
	si := &network.ServerIdentity{Address: dst}
	res := &GetFinalStatementsReply{}

	e := c.SendProtobuf(si, &GetFinalStatements{}, res)
	if e != nil {
		return nil, e
	}
	return res.FinalStatements, nil
}

// StoreKeys asks the service to store public keys for a party.
func (c *Client) StoreKeys(dst network.Address, partyID []byte, keys []kyber.Point) error {
	si := &network.ServerIdentity{Address: dst}
	ret := &StoreKeysReply{}

	return c.SendProtobuf(si, &StoreKeys{partyID, keys, nil}, ret)
}

// GetKeys asks the service for the public keys for a party.
func (c *Client) GetKeys(dst network.Address, partyID []byte) ([]kyber.Point, error) {
	si := &network.ServerIdentity{Address: dst}
	ret := &GetKeysReply{}

	err := c.SendProtobuf(si, &GetKeys{partyID}, ret)
	return ret.Keys, err
}

// StoreInstanceID asks the service to store an instanceID for a given party.
func (c *Client) StoreInstanceID(dst network.Address, partyID []byte, instanceID byzcoin.InstanceID,
	darcID darc.ID) error {
	si := &network.ServerIdentity{Address: dst}
	ret := &StoreInstanceIDReply{}

	return c.SendProtobuf(si, &StoreInstanceID{partyID, instanceID, darcID}, ret)
}

// GetInstanceID asks the service for an instanceID for a given party.
func (c *Client) GetInstanceID(dst network.Address, partyID []byte) (byzcoin.InstanceID, darc.ID, error) {
	si := &network.ServerIdentity{Address: dst}
	ret := &GetInstanceIDReply{}

	err := c.SendProtobuf(si, &GetInstanceID{partyID}, ret)
	return ret.InstanceID, ret.DarcID, err
}

// StoreSigner asks the service to store an Signer for a given party.
func (c *Client) StoreSigner(dst network.Address, partyID []byte, Signer darc.Signer) error {
	si := &network.ServerIdentity{Address: dst}
	ret := &StoreSignerReply{}

	return c.SendProtobuf(si, &StoreSigner{partyID, Signer}, ret)
}

// GetSigner asks the service for an Signer for a given party.
func (c *Client) GetSigner(dst network.Address, partyID []byte) (darc.Signer, error) {
	si := &network.ServerIdentity{Address: dst}
	ret := &GetSignerReply{}

	err := c.SendProtobuf(si, &GetSigner{partyID}, ret)
	return ret.Signer, err
}

// The toml-structure for (un)marshaling with toml
type finalStatementToml struct {
	Desc      *popDescToml
	Attendees []string
	Signature string
	Merged    bool
}

func newFinalStatementFromTomlStruct(fsToml *finalStatementToml) (*FinalStatement, error) {
	desc, err := newPopDescFromTomlStruct(fsToml.Desc)
	if err != nil {
		return nil, err
	}
	atts := []kyber.Point{}
	for _, p := range fsToml.Attendees {
		pub, err := encoding.StringHexToPoint(cothority.Suite, p)
		if err != nil {
			return nil, err
		}
		atts = append(atts, pub)
	}
	sig, err := hex.DecodeString(fsToml.Signature)
	// TODO: sign and verify signature
	if err != nil {
		return nil, err
	}
	return &FinalStatement{
		Desc:      desc,
		Attendees: atts,
		Signature: sig,
		Merged:    fsToml.Merged,
	}, nil
}

// NewFinalStatementFromToml creates a final statement from a toml slice-of-bytes.
func NewFinalStatementFromToml(b []byte) (*FinalStatement, error) {
	fsToml := &finalStatementToml{}
	_, err := toml.Decode(string(b), fsToml)
	if err != nil {
		return nil, err
	}
	fs, err := newFinalStatementFromTomlStruct(fsToml)
	if err != nil {
		return nil, err
	}
	return fs, nil

}

func decodeMapFinal(b []byte) (map[string]*FinalStatement, error) {
	mapToml := make(map[string]*finalStatementToml)
	_, err := toml.Decode(string(b), &mapToml)
	if err != nil {
		return nil, err
	}
	res := make(map[string]*FinalStatement)
	for _, fsToml := range mapToml {
		fs, err := newFinalStatementFromTomlStruct(fsToml)
		if err != nil {
			return nil, err
		}
		res[string(fs.Desc.Hash())] = fs
	}
	return res, nil
}

func encodeMapFinal(stmts map[string]*FinalStatement) ([]byte, error) {
	mapToml := make(map[string]*finalStatementToml)
	var err error
	for key, fs := range stmts {
		mapToml[hex.EncodeToString([]byte(key))], err = fs.toTomlStruct()
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	err = toml.NewEncoder(&buf).Encode(mapToml)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (desc *PopDesc) toTomlStruct() (*popDescToml, error) {
	rostr, err := toToml(desc.Roster)
	if err != nil {
		return nil, err
	}
	parties := make([]shortDescToml, len(desc.Parties))
	for i, p := range desc.Parties {
		parties[i] = shortDescToml{}
		parties[i].Location = p.Location
		parties[i].Roster, err = toToml(p.Roster)
		if err != nil {
			return nil, err
		}
	}
	descToml := &popDescToml{
		Name:     desc.Name,
		DateTime: desc.DateTime,
		Location: desc.Location,
		Roster:   rostr,
		Parties:  parties,
	}
	return descToml, nil
}

func newPopDescFromTomlStruct(descToml *popDescToml) (*PopDesc, error) {
	sis := []*network.ServerIdentity{}
	if descToml == nil {
		return nil, errors.New("no toml struct given")
	}
	for _, s := range descToml.Roster {
		uid, err := uuid.FromString(s[2])
		if err != nil {
			return nil, err
		}
		pub, err := encoding.StringHexToPoint(cothority.Suite, s[3])
		if err != nil {
			return nil, err
		}

		si := &network.ServerIdentity{
			Address:     network.Address(s[0]),
			Description: s[1],
			ID:          network.ServerIdentityID(uid),
			Public:      pub,
		}

		services := []network.ServiceIdentity{}
		for i := 4; i < len(s); i += 2 {
			suite := onet.ServiceFactory.Suite(s[i])
			if suite != nil {
				pub, err := encoding.StringHexToPoint(suite, s[i+1])
				if err != nil {
					return nil, err
				}
				services = append(services, network.ServiceIdentity{
					Name:   s[i],
					Public: pub,
				})
			}
		}
		si.ServiceIdentities = services

		sis = append(sis, si)
	}
	rostr := onet.NewRoster(sis)
	mparties := make([]*ShortDesc, len(descToml.Parties))
	for i, desc := range descToml.Parties {
		mparties[i] = &ShortDesc{}
		mparties[i].Location = desc.Location

		sis := []*network.ServerIdentity{}
		for _, s := range desc.Roster {
			uid, err := uuid.FromString(s[2])
			if err != nil {
				return nil, err
			}
			pub, err := encoding.StringHexToPoint(cothority.Suite, s[3])
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
		mparties[i].Roster = onet.NewRoster(sis)
	}

	return &PopDesc{
		Name:     descToml.Name,
		DateTime: descToml.DateTime,
		Location: descToml.Location,
		Roster:   rostr,
		Parties:  mparties,
	}, nil
}

func (fs *FinalStatement) toTomlStruct() (*finalStatementToml, error) {
	descToml, err := fs.Desc.toTomlStruct()
	if err != nil {
		return nil, err
	}
	if len(fs.Desc.Parties) > 1 {
		descToml.Parties = make([]shortDescToml, len(fs.Desc.Parties))
		for i, p := range fs.Desc.Parties {
			rostr, err := toToml(p.Roster)
			if err != nil {
				return nil, err
			}
			sh := shortDescToml{
				Location: p.Location,
				Roster:   rostr,
			}
			descToml.Parties[i] = sh
		}
	}
	atts := make([]string, len(fs.Attendees))
	for i, p := range fs.Attendees {
		str, err := encoding.PointToStringHex(nil, p)
		if err != nil {
			return nil, err
		}
		atts[i] = str
	}
	fsToml := &finalStatementToml{
		Desc:      descToml,
		Attendees: atts,
		Signature: hex.EncodeToString(fs.Signature),
		Merged:    fs.Merged,
	}
	return fsToml, nil
}

// ToToml returns a toml-slice of byte and an eventual error.
func (fs *FinalStatement) ToToml() ([]byte, error) {
	fsToml, err := fs.toTomlStruct()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = toml.NewEncoder(&buf).Encode(fsToml)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Hash returns the hash of the popdesc and the attendees. In case of an error
// in the hashing it will return a nil-slice and the error.
func (fs *FinalStatement) Hash() ([]byte, error) {
	h := cothority.Suite.Hash()
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
	return fs.VerifyWithService(Name)
}

// VerifyWithService will verify the signature using the public keys
// registered for the given service name
func (fs *FinalStatement) VerifyWithService(name string) error {
	h, err := fs.Hash()
	if err != nil {
		return err
	}

	return protocol.BlsSignature(fs.Signature).Verify(pairingSuite, h, fs.Desc.Roster.ServicePublics(name))
}

// represents a PopDesc in string-version for toml.
type popDescToml struct {
	Name     string
	DateTime string
	Location string
	Roster   [][]string
	Parties  []shortDescToml
}

type shortDescToml struct {
	Location string
	Roster   [][]string
}

// Hash of this structure - calculated by hand instead of using network.Marshal.
func (desc *PopDesc) Hash() []byte {
	hash := cothority.Suite.Hash()
	hash.Write([]byte(desc.Name))
	hash.Write([]byte(desc.DateTime))
	hash.Write([]byte(desc.Location))
	buf, err := desc.Roster.Aggregate.MarshalBinary()
	if err != nil {
		log.Error(err)
		return []byte{}
	}
	hash.Write(buf)
	if len(desc.Parties) > 0 {
		for _, party := range desc.Parties {
			hash.Write([]byte(party.Location))
			buf, err = party.Roster.Aggregate.MarshalBinary()
			if err != nil {
				log.Error(err)
				return []byte{}
			}
			hash.Write(buf)
		}
	}
	return hash.Sum(nil)
}

// rosterEqual checks if the first list contains the second
func rosterEqual(r1, r2 *onet.Roster) bool {
	if len(r1.List) != len(r2.List) {
		return false
	}
	return r1.Contains(r2.Publics())
}

func toToml(r *onet.Roster) ([][]string, error) {
	rostr := make([][]string, len(r.List))
	for i, si := range r.List {
		str, err := encoding.PointToStringHex(nil, si.Public)
		if err != nil {
			return nil, err
		}
		sistr := []string{si.Address.String(), si.Description,
			uuid.UUID(si.ID).String(), str}

		for _, sid := range si.ServiceIdentities {
			suite := onet.ServiceFactory.Suite(sid.Name)
			if suite != nil {
				pub, err := encoding.PointToStringHex(suite, sid.Public)
				if err != nil {
					return nil, err
				}
				sistr = append(sistr, sid.Name, pub)
			}
		}

		rostr[i] = sistr
	}
	return rostr, nil
}

// PopToken represents pop-token
type PopToken struct {
	Final   *FinalStatement
	Private kyber.Scalar
	Public  kyber.Point
}

type popTokenToml struct {
	Final   *finalStatementToml
	Private string
	Public  string
}

func newPopTokenFromTomlStruct(t *popTokenToml) (*PopToken, error) {
	token := &PopToken{}
	var err error
	token.Final, err = newFinalStatementFromTomlStruct(t.Final)
	if err != nil {
		return nil, err
	}
	token.Private, err = encoding.StringHexToScalar(cothority.Suite, t.Private)
	if err != nil {
		return nil, err
	}
	token.Public, err = encoding.StringHexToPoint(cothority.Suite, t.Public)
	if err != nil {
		return nil, err
	}
	if token.Final.Verify() != nil {
		return nil, errors.New("FinalStatement is invalid")
	}
	return token, nil
}

// NewPopTokenFromToml recovers PopToken struct from toml
func NewPopTokenFromToml(b []byte) (*PopToken, error) {
	tokenToml := &popTokenToml{}
	_, err := toml.Decode(string(b), tokenToml)
	if err != nil {
		return nil, err
	}
	return newPopTokenFromTomlStruct(tokenToml)
}
