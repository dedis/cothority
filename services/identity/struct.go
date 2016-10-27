package identity

import (
	"encoding/binary"
	"sort"

	"fmt"
	"strings"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

// Config holds the information about all devices and the data stored in this
// identity-blockchain. All Devices have voting-rights to the Config-structure.
type Config struct {
	Threshold int
	Device    map[string]*Device
	Data      map[string]string
}

// Device is represented by a public key.
type Device struct {
	Point abstract.Point
}

// NewConfig returns a new List with the first owner initialised.
func NewConfig(threshold int, pub abstract.Point, owner string) *Config {
	return &Config{
		Threshold: threshold,
		Device:    map[string]*Device{owner: {pub}},
		Data:      make(map[string]string),
	}
}

// Copy returns a deep copy of the AccountList.
func (c *Config) Copy() *Config {
	b, err := network.MarshalRegisteredType(c)
	if err != nil {
		log.Error("Couldn't marshal AccountList:", err)
		return nil
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		log.Error("Couldn't unmarshal AccountList:", err)
	}
	ilNew := msg.(Config)
	if len(ilNew.Data) == 0 {
		ilNew.Data = make(map[string]string)
	}
	return &ilNew
}

// Hash makes a cryptographic hash of the configuration-file - this
// can be used as an ID.
func (c *Config) Hash() (crypto.HashID, error) {
	hash := network.Suite.Hash()
	err := binary.Write(hash, binary.LittleEndian, int32(c.Threshold))
	if err != nil {
		return nil, err
	}
	var owners []string
	for s := range c.Device {
		owners = append(owners, s)
	}
	sort.Strings(owners)
	for _, s := range owners {
		_, err = hash.Write([]byte(s))
		if err != nil {
			return nil, err
		}
		_, err = hash.Write([]byte(c.Data[s]))
		if err != nil {
			return nil, err
		}
		b, err := network.MarshalRegisteredType(c.Device[s])
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return hash.Sum(nil), nil
}

// String returns a nicely formatted output of the AccountList
func (c *Config) String() string {
	var owners []string
	for n := range c.Device {
		owners = append(owners, fmt.Sprintf("Owner: %s", n))
	}
	var data []string
	for k, v := range c.Data {
		data = append(data, fmt.Sprintf("Data: %s/%s", k, v))
	}
	return fmt.Sprintf("Threshold: %d\n%s\n%s", c.Threshold,
		strings.Join(owners, "\n"), strings.Join(data, "\n"))
}

// GetSuffixColumn returns the unique values up to the next ":" of the keys.
// If given a slice of keys, it will join them using ":" and return the
// unique keys with that prefix.
func (c *Config) GetSuffixColumn(keys ...string) []string {
	var ret []string
	start := strings.Join(keys, ":")
	if len(start) > 0 {
		start += ":"
	}
	for k := range c.Data {
		if strings.HasPrefix(k, start) {
			// Create subkey
			subkey := strings.TrimPrefix(k, start)
			subkey = strings.SplitN(subkey, ":", 2)[0]
			ret = append(ret, subkey)
		}
	}
	return sortUniq(ret)
}

// GetValue returns the value of the key. If more than one key is given,
// the slice is joined using ":" and the value is returned. If the key
// is not found, an empty string is returned.
func (c *Config) GetValue(keys ...string) string {
	key := strings.Join(keys, ":")
	for k, v := range c.Data {
		if k == key {
			return v
		}
	}
	return ""
}

// GetIntermediateColumn returns the values of the column in the middle of
// prefix and suffix. Searching for the column-values, the method will add ":"
// after the prefix and before the suffix.
func (c *Config) GetIntermediateColumn(prefix, suffix string) []string {
	var ret []string
	if len(prefix) > 0 {
		prefix += ":"
	}
	if len(suffix) > 0 {
		suffix = ":" + suffix
	}
	for k := range c.Data {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			interm := strings.TrimPrefix(k, prefix)
			interm = strings.TrimSuffix(interm, suffix)
			if !strings.Contains(interm, ":") {
				ret = append(ret, interm)
			}
		}
	}
	return sortUniq(ret)
}

// sortUniq sorts the slice of strings and deletes duplicates
func sortUniq(slice []string) []string {
	sorted := make([]string, len(slice))
	copy(sorted, slice)
	sort.Strings(sorted)
	var ret []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			ret = append(ret, s)
		}
	}
	return ret
}

// Messages between the Client-API and the Service

// CreateIdentity starts a new identity-skipchain with the initial
// Config and asking all nodes in Roster to participate.
type CreateIdentity struct {
	Config *Config
	Roster *sda.Roster
}

// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the Root and Data-skipchain.
type CreateIdentityReply struct {
	Root *skipchain.SkipBlock
	Data *skipchain.SkipBlock
}

// ConfigUpdate verifies if a new update is available.
type ConfigUpdate struct {
	ID ID
}

// ConfigUpdateReply returns the updated configuration.
type ConfigUpdateReply struct {
	Config *Config
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID ID
	*Config
}

// ProposeUpdate verifies if a new config is available.
type ProposeUpdate struct {
	ID ID
}

// ProposeUpdateReply returns the updated propose-configuration.
type ProposeUpdateReply struct {
	Propose *Config
}

// ProposeVote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type ProposeVote struct {
	ID        ID
	Signer    string
	Signature *crypto.SchnorrSig
}

// ProposeVoteReply returns the signed new skipblock if the threshold of
// votes have arrived.
type ProposeVoteReply struct {
	Data *skipchain.SkipBlock
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*Storage
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID     ID
	Latest *skipchain.SkipBlock
}
