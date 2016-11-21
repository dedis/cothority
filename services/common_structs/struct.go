package common_structs

import (
	//"bytes"
	"encoding/binary"
	//"errors"
	"fmt"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"sort"
	"strings"
	"time"
	//"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

func init() {
	for _, s := range []interface{}{
		// Structures
		&CAInfo{},
		&Config{},
	} {
		network.RegisterPacketType(s)
	}
}

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

// How many msec at most should be the time difference between a device/cothority node/CA and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

// Config holds the information about all devices and the data stored in this
// identity-blockchain. All Devices have voting-rights to the Config-structure.
type Config struct {
	// The time in seconds when the request was started
	Timestamp int64
	Threshold int
	Device    map[string]*Device
	Data      map[string]string
	// The public keys of the trusted CAs
	CAs []CAInfo
}

// Device is represented by a public key and possibly the signature of the
// associated device upon the current proposed config
type Device struct {
	Point abstract.Point
	Vote  *crypto.SchnorrSig
}

type DevicePoint struct {
	Point abstract.Point
}

type CAInfo struct {
	Public   abstract.Point
	ServerID *network.ServerIdentity
}

type WSInfo struct {
	ServerID *network.ServerIdentity
}

type SiteInfo struct {
	// Site's ID (hash of the genesis block)
	ID skipchain.SkipBlockID
	// Addresses of the site's web servers
	WSs []WSInfo
}

type PinState struct {
	// The type of our identity ("device", "ws", "user")
	Ctype string
	// Minimum number of 'Pins' keys signing the new skipblock
	Threshold int
	// The trusted pins for the time interval 'Window'
	Pins []abstract.Point
	// Trusted window in ms for the current 'Pins'
	Window int64
	// When we received the latest accepted skipblock
	TimeSbRec int64
}

func NewPinState(ctype string, threshold int, pins []abstract.Point, window int64) *PinState {
	return &PinState{
		Ctype:     ctype,
		Threshold: threshold,
		Pins:      pins,
		Window:    window,
	}
}

// NewConfig returns a new List with the first owner initialised.
func NewConfig(threshold int, pub abstract.Point, owner string, cas []CAInfo) *Config {
	return &Config{
		Threshold: threshold,
		Device:    map[string]*Device{owner: {Point: pub}},
		Data:      make(map[string]string),
		CAs:       cas,
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
	//log.Print("Computing config's hash")
	hash := network.Suite.Hash()
	var data = []int64{
		int64(c.Timestamp),
		int64(c.Threshold),
	}
	err := binary.Write(hash, binary.LittleEndian, data)
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
		point := &DevicePoint{Point: c.Device[s].Point}
		b, err := network.MarshalRegisteredType(point)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}

	if c.CAs == nil {
		log.Print("No CAs found")
	}
	for _, info := range c.CAs {
		//log.Printf("public: %v", info.Public)
		b, err := network.MarshalRegisteredType(&info)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
	}
	the_hash := hash.Sum(nil)
	//log.Printf("End of config's hash computation, hash: %v", the_hash)
	return the_hash, nil
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

func (c *Config) SetNowTimestamp() error {
	c.Timestamp = time.Now().Unix()
	log.Printf("Setting proposed config's timestamp to: %v", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	return nil
}

func (c *Config) CheckTimeDiff(maxvalue int64) error {
	timestamp := c.Timestamp
	diff := time.Since(time.Unix(timestamp, 0))
	diff_int := diff.Nanoseconds() / 1000000
	if diff_int > maxvalue {
		return fmt.Errorf("refused to sign off due to bad timestamp: time difference: %v exceeds the %v interval", diff, maxvalue)
	}
	log.Printf("Checking Timestamp: time difference: %v OK", diff)
	return nil
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
