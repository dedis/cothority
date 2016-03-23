package app

import (
	"bytes"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// ReadGroupToml reads a group.toml file and returns the list of Entity
// described in the file.
func ReadGroupToml(f io.Reader) (*sda.EntityList, error) {
	group := &GroupToml{}
	_, err := toml.DecodeReader(f, group)
	if err != nil {
		return nil, err
	}
	// convert from ServerTomls to entities
	var entities = make([]*network.Entity, 0, len(group.Servers))
	for _, s := range group.Servers {
		en, err := s.toEntity(network.Suite)
		if err != nil {
			return nil, err
		}
		entities = append(entities, en)
	}
	el := sda.NewEntityList(entities)
	return el, nil
}

func NewGroupToml(servers ...*ServerToml) *GroupToml {
	return &GroupToml{
		Servers: servers,
	}
}

// Returns the TOML representation of this GroupToml
func (gt *GroupToml) String() string {
	var buff bytes.Buffer
	if gt.Description == "" {
		gt.Description = "## Put your description of the cothority system for more convenience ##"
	}
	for _, s := range gt.Servers {
		if s.Description == "" {
			s.Description = "## Put your description here for convenience ##"
		}
	}
	enc := toml.NewEncoder(&buff)
	if err := enc.Encode(gt); err != nil {
		return "Error encoding grouptoml" + err.Error()
	}
	return buff.String()
}

// toEntity will convert this ServerToml struct to a network entity.
func (s *ServerToml) toEntity(suite abstract.Suite) (*network.Entity, error) {
	pubR := strings.NewReader(s.Public)
	public, err := cliutils.ReadPub64(suite, pubR)
	if err != nil {
		return nil, err
	}
	return network.NewEntity(public, s.Addresses...), nil
}

// Returns a ServerToml out of a public key and some addresses => to be printed
// or written to a file
func NewServerToml(suite abstract.Suite, public abstract.Point, addresses ...string) *ServerToml {
	var buff bytes.Buffer
	if err := cliutils.WritePub64(suite, &buff, public); err != nil {
		dbg.Error("Error writing public key")
		return nil
	}
	return &ServerToml{
		Addresses: addresses,
		Public:    buff.String(),
	}
}

// Returns its TOML representation
func (s *ServerToml) String() string {
	var buff bytes.Buffer
	if s.Description == "" {
		s.Description = "## Put your description here for convenience ##"
	}
	enc := toml.NewEncoder(&buff)
	if err := enc.Encode(s); err != nil {
		return "## Error encoding server informations ##" + err.Error()
	}
	return buff.String()
}

// GroupToml represents the structure of the group.toml file given to the cli.
type GroupToml struct {
	Description string
	Servers     []*ServerToml `toml:"servers"`
}

// ServerToml is one entry in the group.toml file describing one server to use for
// the cothority system.
type ServerToml struct {
	Addresses   []string
	Public      string
	Description string
}
