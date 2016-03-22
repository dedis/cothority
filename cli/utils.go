package cli

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"io"
	"strings"
)

// groupToml represents the structure of the group.toml file given to the cli.
type groupToml struct {
	Description string
	Servers     []server `toml:"servers"`
}

// server is one entry in the group.toml file describing one server to use for
// the cothority system.
type server struct {
	Addresses   []string
	Public      string
	Description string
}

// ReadGroupToml reads a group.toml file and returns the list of Entity
// described in the file.
func ReadGroupToml(f io.Reader) (*sda.EntityList, error) {
	group := &groupToml{}
	_, err := toml.DecodeReader(f, group)
	if err != nil {
		return nil, err
	}
	// convert from servers to entities
	var entities = make([]*network.Entity, 0, len(group.Servers))
	for _, s := range group.Servers {
		en, err := s.toEntity()
		if err != nil {
			return nil, err
		}
		entities = append(entities, en)
	}
	el := sda.NewEntityList(entities)
	return el, nil
}

// toEntity will convert this server struct to a network entity.
func (s *server) toEntity() (*network.Entity, error) {
	public, err := cliutils.ReadPub64(network.Suite, strings.NewReader(s.Public))
	if err != nil {
		return nil, err
	}
	return network.NewEntity(public, s.Addresses...), nil
}
