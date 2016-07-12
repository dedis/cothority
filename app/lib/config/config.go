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

	"os/user"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

var in *bufio.Reader
var out io.Writer

func init() {
	in = bufio.NewReader(os.Stdin)
	out = os.Stdout
}

// CothoritydConfig is the cothority daemon config
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
	secret, err := crypto.ReadScalarHex(network.Suite, hc.Private)
	if err != nil {
		return nil, nil, err
	}
	point, err := crypto.ReadPubHex(network.Suite, hc.Public)
	if err != nil {
		return nil, nil, err
	}
	host := sda.NewHost(network.NewServerIdentity(point, hc.Addresses...), secret)
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
		return nil, "", errors.New("Error reading IP:PORT (" + str + ") " + errStr.Error() + " => Abort")
	}

	if net.ParseIP(h) == nil {
		return nil, "", errors.New("Invalid IP address " + h)
	}

	fmt.Println("[+] Creation of the private and public keys...")
	kp := config.NewKeyPair(network.Suite)
	privStr, err := crypto.ScalarHex(network.Suite, kp.Secret)
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

// Group holds the Roster and the server-descriptions
type Group struct {
	Roster      *sda.Roster
	description map[*network.ServerIdentity]string
}

// GetDescription returns the description of an entity
func (g *Group) GetDescription(e *network.ServerIdentity) string {
	return g.description[e]
}

// ReadGroupDescToml reads a group.toml file and returns the list of ServerIdentities
// and descriptions in the file.
func ReadGroupDescToml(f io.Reader) (*Group, error) {
	group := &GroupToml{}
	_, err := toml.DecodeReader(f, group)
	if err != nil {
		return nil, err
	}
	// convert from ServerTomls to entities
	var entities = make([]*network.ServerIdentity, len(group.Servers))
	var descs = make(map[*network.ServerIdentity]string)
	for i, s := range group.Servers {
		en, err := s.toServerIdentity(network.Suite)
		if err != nil {
			return nil, err
		}
		entities[i] = en
		descs[en] = s.Description
	}
	el := sda.NewRoster(entities)
	return &Group{el, descs}, nil
}

// ReadGroupToml reads a group.toml file and returns the list of ServerIdentity
// described in the file.
func ReadGroupToml(f io.Reader) (*sda.Roster, error) {
	group, err := ReadGroupDescToml(f)
	if err != nil {
		return nil, err
	}
	return group.Roster, nil
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
		gt.Description = "Description of your cothority roster"
	}
	for _, s := range gt.Servers {
		if s.Description == "" {
			s.Description = "Description of your server"
		}
	}
	enc := toml.NewEncoder(&buff)
	if err := enc.Encode(gt); err != nil {
		return "Error encoding grouptoml" + err.Error()
	}
	return buff.String()
}

// toServerIdentity will convert this ServerToml struct to a network entity.
func (s *ServerToml) toServerIdentity(suite abstract.Suite) (*network.ServerIdentity, error) {
	pubR := strings.NewReader(s.Public)
	public, err := crypto.ReadPub64(suite, pubR)
	if err != nil {
		return nil, err
	}
	return network.NewServerIdentity(public, s.Addresses...), nil
}

// NewServerToml returns  a ServerToml out of a public key and some addresses => to be printed
// or written to a file
func NewServerToml(suite abstract.Suite, public abstract.Point, addresses ...string) *ServerToml {
	var buff bytes.Buffer
	if err := crypto.WritePub64(suite, &buff, public); err != nil {
		log.Error("Error writing public key")
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

// TildeToHome takes a path and replaces an eventual "~" with the home-directory
func TildeToHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		log.ErrFatal(err, "Got error while fetching home-directory")
		return usr.HomeDir + path[1:]
	}
	return path
}

// Input prints the arguments given with a 'input'-format and
// proposes the 'def' string as default. If the user presses
// 'enter', the 'dev' will be returned.
func Input(def string, args ...interface{}) string {
	fmt.Fprint(out, args...)
	fmt.Fprintf(out, " [%s]: ", def)
	str, err := in.ReadString('\n')
	if err != nil {
		log.Fatal("Could not read input.")
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return def
	}
	return str
}

// Inputf takes a format and calls Input
func Inputf(def string, f string, args ...interface{}) string {
	return Input(def, fmt.Sprintf(f, args...))
}

// InputYN asks a Yes/No question
func InputYN(def bool, args ...interface{}) bool {
	defStr := "Yn"
	if !def {
		defStr = "Ny"
	}
	return strings.ToLower(string(Input(defStr, args...)[0])) == "y"
}
