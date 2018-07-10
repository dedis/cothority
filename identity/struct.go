package identity

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000 * time.Millisecond

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

// Equal returns true if it's the same ID.
func (i ID) Equal(j ID) bool {
	return i.Equal(j)
}

// FuzzyEqual returns true if the first part of the ID
// and the given substring match.
func (i ID) FuzzyEqual(j []byte) bool {
	return bytes.Compare(i[0:len(j)], j) == 0
}

// NewData returns a new List with the first owner initialised.
func NewData(roster *onet.Roster, threshold int, pub kyber.Point, owner string) *Data {
	return &Data{
		Roster:    roster,
		Threshold: threshold,
		Device:    map[string]*Device{owner: {pub}},
		Storage:   make(map[string]string),
		Votes:     map[string][]byte{},
	}
}

// Copy returns a deep copy of the Data.
func (d *Data) Copy() *Data {
	b, err := network.Marshal(d)
	if err != nil {
		log.Error("Couldn't marshal Data:", err)
		return nil
	}
	_, msg, err := network.Unmarshal(b, cothority.Suite)
	if err != nil {
		log.Error("Couldn't unmarshal Data:", err)
	}
	dNew := msg.(*Data)
	if len(dNew.Storage) == 0 {
		dNew.Storage = make(map[string]string)
	}
	dNew.Votes = map[string][]byte{}

	return dNew
}

// Hash makes a cryptographic hash of the data-file - this
// can be used as an ID. The vote of the devices is not included in the hash!
func (d *Data) Hash(suite kyber.HashFactory) ([]byte, error) {
	hash := suite.Hash()
	err := binary.Write(hash, binary.LittleEndian, int32(d.Threshold))
	if err != nil {
		return nil, err
	}

	// Write all devices in alphabetical order, because golang
	// randomizes the maps.
	var owners []string
	for s := range d.Device {
		owners = append(owners, s)
	}
	sort.Strings(owners)
	for _, s := range owners {
		_, err = hash.Write([]byte(s))
		if err != nil {
			return nil, err
		}
		_, err = d.Device[s].Point.MarshalTo(hash)
		if err != nil {
			return nil, err
		}
	}

	// And write all keys in alphabetical order, because golang
	// randomizes the maps.
	var keys []string
	for k := range d.Storage {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, err = hash.Write([]byte(d.Storage[k]))
		if err != nil {
			return nil, err
		}
	}

	if d.Roster != nil {
		d.Roster.Aggregate.MarshalTo(hash)
	}

	return hash.Sum(nil), nil
}

// String returns a nicely formatted output of the AccountList
func (d *Data) String() string {
	var owners []string
	for n := range d.Device {
		owners = append(owners, fmt.Sprintf("Owner: %s", n))
	}
	var data []string
	for k, v := range d.Storage {
		data = append(data, fmt.Sprintf("Data: %s/%s", k, v))
	}
	for v := range d.Votes {
		data = append(data, fmt.Sprintf("Vote from: %s", v))
	}
	data = append(data, fmt.Sprintf("Number of votes: %d", len(d.Votes)))
	return fmt.Sprintf("Threshold: %d\n%s\n%s", d.Threshold,
		strings.Join(owners, "\n"), strings.Join(data, "\n"))
}

// GetSuffixColumn returns the unique values up to the next ":" of the keys.
// If given a slice of keys, it will join them using ":" and return the
// unique keys with that prefix.
func (d *Data) GetSuffixColumn(keys ...string) []string {
	var ret []string
	start := strings.Join(keys, ":")
	if len(start) > 0 {
		start += ":"
	}
	for k := range d.Storage {
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
func (d *Data) GetValue(keys ...string) string {
	key := strings.Join(keys, ":")
	for k, v := range d.Storage {
		if k == key {
			return v
		}
	}
	return ""
}

// GetIntermediateColumn returns the values of the column in the middle of
// prefix and suffix. Searching for the column-values, the method will add ":"
// after the prefix and before the suffix.
func (d *Data) GetIntermediateColumn(prefix, suffix string) []string {
	var ret []string
	if len(prefix) > 0 {
		prefix += ":"
	}
	if len(suffix) > 0 {
		suffix = ":" + suffix
	}
	for k := range d.Storage {
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
