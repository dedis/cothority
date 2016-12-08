package common_structs

import (
	//"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	//"errors"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"sort"
	"strings"
	"time"
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

// How many ms at most should be the time difference between a device/cothority node/CA and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

type My_Scalar struct {
	Private abstract.Scalar
}

// Config holds the information about all devices and the data stored in this
// identity-blockchain. All Devices have voting-rights to the Config-structure.
type Config struct {
	FQDN string
	// The time in ms when the request was started
	Timestamp int64
	// If a cert is going to be acquired for this config, MaxDuration indicates the maximum
	// time period for which the cert is going to be valid (countdown starting from the 'Timestamp')
	MaxDuration int64

	Threshold int
	Device    map[string]*Device
	Data      map[string]*WSconfig

	// The public keys of the trusted CAs
	CAs []CAInfo

	// Aggregate public key of the proxy cothority (to be trusted for Proofs-of-Freshness)
	//ProxyAggr abstract.Point
	ProxyRoster *sda.Roster
}

// Device is represented by a public key and possibly the signature of the
// associated device upon the current proposed config
type Device struct {
	Point abstract.Point
	Vote  *crypto.SchnorrSig
}

type WSconfig struct {
	ServerID *network.ServerIdentity
	// TLS public key of the web server
	TLSPublic abstract.Point
	K1        abstract.Point
	C1        abstract.Point
	K2        abstract.Point
	C2        abstract.Point
}

type APoint struct {
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
	//ID skipchain.SkipBlockID
	FQDN string
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
	// Trusted window for the current 'Pins'
	Window int64
	// Time when the latest pins were accepted
	TimePinAccept int64
}

type Cert struct {
	// Site's ID
	ID skipchain.SkipBlockID
	// The pointed config's hash
	Hash crypto.HashID
	// The signature of the certification authority upon the 'Hash'
	Signature *crypto.SchnorrSig
	// The public key of the certification authority
	Public abstract.Point
}

type CertInfo struct {
	// Hash of the skiblock the config of which has been certified by the latest cert
	// which is the only one that is currently valid
	SbHash skipchain.SkipBlockID
	Cert   *Cert
}

type Key []byte

func NewPinState(ctype string, threshold int, pins []abstract.Point, window int64) *PinState {
	return &PinState{
		Ctype:     ctype,
		Threshold: threshold,
		Pins:      pins,
		Window:    window,
	}
}

// NewConfig returns a new List with the first owner initialised.
func NewConfig(fqdn string, threshold int, pub abstract.Point, proxyroster *sda.Roster, owner string, cas []CAInfo, data map[string]*WSconfig, duration int64) *Config {
	return &Config{
		FQDN:        fqdn,
		Threshold:   threshold,
		Device:      map[string]*Device{owner: {Point: pub}},
		Data:        data,
		CAs:         cas,
		MaxDuration: duration,
		//ProxyAggr:   proxyaggr,
		ProxyRoster: proxyroster,
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
		//ilNew.Data = make(map[string]string)
		ilNew.Data = make(map[string]*WSconfig)
	}
	return &ilNew
}

// Hash makes a cryptographic hash of the configuration-file - this
// can be used as an ID.
func (c *Config) Hash() (crypto.HashID, error) {
	//log.Print("Computing config's hash")
	hash := network.Suite.Hash()

	_, err := hash.Write([]byte(c.FQDN))
	if err != nil {
		return nil, err
	}

	var data = []int64{
		int64(c.Timestamp),
		int64(c.Threshold),
		int64(c.MaxDuration),
	}
	err = binary.Write(hash, binary.LittleEndian, data)
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

		point := &APoint{Point: c.Device[s].Point}
		b, err := network.MarshalRegisteredType(point)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}

	var keys []string
	for k := range c.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// point := &APoint{Point: c.Data[k]}
		wsconf := &WSconfig{
			//ServerID: c.Data[k].ServerID,
			TLSPublic: c.Data[k].TLSPublic,
			K1:        c.Data[k].K1,
			C1:        c.Data[k].C1,
			K2:        c.Data[k].K2,
			C2:        c.Data[k].C2,
		}
		b, err := network.MarshalRegisteredType(wsconf)
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
	// Include the aggregate public key into the hash (cothority is trusted for issuing proofs of freshness)
	point := &APoint{Point: c.ProxyRoster.Aggregate}
	b, err2 := network.MarshalRegisteredType(point)
	if err2 != nil {
		return nil, err2
	}
	_, err = hash.Write(b)
	if err != nil {
		return nil, err
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
	// the number of ms elapsed since January 1, 1970 UTC
	c.Timestamp = time.Now().Unix() * 1000
	log.Printf("Setting proposed config's timestamp to: %v", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	return nil
}

func (c *Config) CheckTimeDiff(maxvalue int64) error {
	timestamp := c.Timestamp
	diff := time.Since(time.Unix(0, timestamp*1000000))
	diff_int := diff.Nanoseconds() / 1000000
	if diff_int > maxvalue {
		return fmt.Errorf("refused to sign off due to bad timestamp: time difference: %v exceeds the %v interval", diff, maxvalue)
	}
	log.Printf("Checking Timestamp: time difference: %v OK", diff)
	return nil
}

func (c *Config) ExpiredCertConfig() bool {
	diff := time.Since(time.Unix(0, c.Timestamp*1000000))
	diff_int := diff.Nanoseconds() / 1000000

	if c.MaxDuration < diff_int {
		log.LLvlf2("Expired cert!!")
		return true
	}
	return false
}

// returns true if c is older than c2
func (c *Config) IsOlderConfig(c2 *Config) bool {
	timestamp := c.Timestamp
	diff1 := time.Since(time.Unix(0, timestamp*1000000))
	//diff_int1 := diff.Nanoseconds() / 1000000
	log.Printf("Conf1 hash time difference: %v OK", diff1)
	//log.LLvlf2("%v", diff_int1)
	timestamp = c2.Timestamp
	diff2 := time.Since(time.Unix(0, timestamp*1000000))
	//diff_int2 := diff.Nanoseconds() / 1000000
	log.Printf("Conf2 hash time difference: %v OK", diff2)
	//log.LLvlf2("%v", diff_int2)
	log.LLvlf2("%v", diff1.Nanoseconds())
	log.LLvlf2("%v", diff2.Nanoseconds())
	if diff1.Nanoseconds() < diff2.Nanoseconds() {
		return true
	}
	return false
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
func (c *Config) GetValue(keys ...string) *WSconfig {
	key := strings.Join(keys, ":")
	for k, v := range c.Data {
		if k == key {
			return v
		}
	}
	return nil
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

func Encrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func Decrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetConfFromSb(sb *skipchain.SkipBlock) (*Config, error) {
	_, data, err := network.UnmarshalRegistered(sb.Data)
	if err != nil {
		return nil, errors.New("Couldn't unmarshal")
	}
	config, ok := data.(*Config)
	if !ok {
		return nil, errors.New("Couldn't get type '*Config'")
	}
	return config, nil
}

func ElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, message []byte) (
	K, C abstract.Point, remainder []byte) {
	// Embed the message (or as much of it as will fit) into a curve point.
	M, remainder := suite.Point().Pick(message, random.Stream)

	if len(remainder) != 0 {
		log.LLvlf2("message's len: %v", len(message))
		log.LLvlf2("remainder's len: %v", len(remainder))
	}

	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := suite.Scalar().Pick(random.Stream) // ephemeral private key
	K = suite.Point().Mul(nil, k)           // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k)       // ephemeral DH shared secret
	C = S.Add(S, M)                         // message blinded with secret
	return
}

func ElGamalDecrypt(suite abstract.Suite, prikey abstract.Scalar, K, C abstract.Point) (
	message []byte, err error) {

	// ElGamal-decrypt the ciphertext (K,C) to reproduce the message.
	S := suite.Point().Mul(K, prikey) // regenerate shared secret
	M := suite.Point().Sub(C, S)      // use to un-blind the message
	message, err = M.Data()           // extract the embedded data
	return
}
