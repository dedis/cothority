package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// CothoritydConfig is the Cothority daemon config
type CothoritydConfig struct {
	Public    string
	Private   string
	Addresses []string
}

// Save will save this CothoritydConfig to the given file name
func (hc *CothoritydConfig) Save(file string) error {
	fd, err := os.Create(file)
	if err != nil {
		return err
	}
	err = toml.NewEncoder(fd).Encode(hc)
	if err != nil {
		return err
	}
	return nil
}

// ParseCothorityd will try to parse the config file into a CothoritydConfig.
// It returns the CothoritydConfig, the Host so we can already use it and an error if
// occured.
func ParseCothorityd(file string) (*CothoritydConfig, *sda.Host, error) {
	hc := &CothoritydConfig{}
	_, err := toml.DecodeFile(file, hc)
	if err != nil {
		return nil, nil, err
	}
	// Try to decode the Hex values
	secret, err := crypto.ReadSecretHex(network.Suite, hc.Private)
	if err != nil {
		return nil, nil, err
	}
	point, err := crypto.ReadPubHex(network.Suite, hc.Public)
	if err != nil {
		return nil, nil, err
	}
	host := sda.NewHost(network.NewEntity(point, hc.Addresses...), secret)
	return hc, host, nil
}

// CreateCothoritydConfig will ask through the command line to create a Private / Public
// key, what is the listening address
func CreateCothoritydConfig(defaultFile string) (*CothoritydConfig, string, error) {
	reader := bufio.NewReader(os.Stdin)
	var err error
	var str string
	// IP:PORT
	fmt.Println("[+] Type the IP:PORT (ipv4) address of this host (accessible from Internet):")
	str, err = reader.ReadString('\n')
	str = strings.TrimSpace(str)
	h, _, errStr := net.SplitHostPort(str)
	if err != nil || errStr != nil {
		return nil, "", fmt.Errorf("Error reading IP:PORT (", str, ") ", errStr, " => Abort")
	}

	if net.ParseIP(h) == nil {
		return nil, "", errors.New("Invalid IP address " + h)
	}

	fmt.Println("[+] Creation of the private and public keys...")
	kp := config.NewKeyPair(network.Suite)
	privStr, err := crypto.SecretHex(network.Suite, kp.Secret)
	if err != nil {
		return nil, "", fmt.Errorf("Could not parse private key. Abort.")
	}
	pubStr, err := crypto.PubHex(network.Suite, kp.Public)
	if err != nil {
		return nil, "", fmt.Errorf("Could not parse public key. Abort.")
	}
	fmt.Println("\tPrivate:\t", privStr)
	fmt.Println("\tPublic: \t", pubStr)

	fmt.Println("[+] Name of the config file [", defaultFile, "]:")
	fname, err := reader.ReadString('\n')
	fname = strings.TrimSpace(fname)

	config := &CothoritydConfig{
		Public:    pubStr,
		Private:   privStr,
		Addresses: []string{str},
	}
	return config, fname, err
}

// GroupToml represents the structure of the group.toml file given to the cli.
type GroupToml struct {
	Description string
	Servers     []*ServerToml `toml:"servers"`
}

// NewGroupToml creates a new GroupToml struct from the given ServerTomls.
// Currently used together with calling String() on the GroupToml to output
// a snippet which is needed to define the CoSi group
func NewGroupToml(servers ...*ServerToml) *GroupToml {
	return &GroupToml{
		Servers: servers,
	}
}

// ServerToml is one entry in the group.toml file describing one server to use for
// the cothority system.
type ServerToml struct {
	Addresses   []string
	Public      string
	Description string
}

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

// Save writes the grouptoml definition into the file
func (gt *GroupToml) Save(fname string) error {
	file, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(gt.String())
	return err
}

// String returns the TOML representation of this GroupToml
func (gt *GroupToml) String() string {
	var buff bytes.Buffer
	if gt.Description == "" {
		gt.Description = "Best Cothority Roster"
	}
	for _, s := range gt.Servers {
		if s.Description == "" {
			s.Description = "Buckaroo Bonzai's Cothority Server"
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
	public, err := crypto.ReadPub64(suite, pubR)
	if err != nil {
		return nil, err
	}
	return network.NewEntity(public, s.Addresses...), nil
}

// NewServerToml returns  a ServerToml out of a public key and some addresses => to be printed
// or written to a file
func NewServerToml(suite abstract.Suite, public abstract.Point, addresses ...string) *ServerToml {
	var buff bytes.Buffer
	if err := crypto.WritePub64(suite, &buff, public); err != nil {
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
