package common_structs

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/dedis/cothority/dns_id/skipchain"
	//"github.com/dedis/cothority/dns_id/swupdate"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"crypto/sha512"
)

func init() {
	for _, s := range []interface{}{
		// Structures
		&CAInfo{},
		&Config{},
	} {
		network.RegisterMessage(s)
	}
}

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many ms at most should be the time difference between a device/cothority node/CA and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

// ID represents one skipblock and corresponds to its Hash.
type ID []byte

type My_Scalar struct {
	Private abstract.Scalar
}

// Config holds the information about all devices and the data stored in this
// identity-blockchain. All Devices have voting-rights to the Config-structure.
type Config struct {
	// hash of the previous config-block
	BLink []byte
	FQDN string
	// The time in ms when the request was started
	Timestamp int64

	Threshold int
	Device    map[string]*Device
	Data      map[string]*WSconfig

	//ProxyRoster *onet.Roster
	Aggregate abstract.Point

	// Number of authoritative WKHs (strictly more that two-thirds of them have to sign off a valid POF)
	NbrWkhs int
}

type ConfigPlusNextHash struct {
	Config *Config
	NextHash []byte
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
	ID []byte
	// The pointed config's hash
	Hash []byte
	// The signature of the certification authority upon the 'Hash'
	Signature *crypto.SchnorrSig
	// The public key of the certification authority
	Public abstract.Point
}

type CertInfo struct {
	// Hash of the skiblock the config of which has been certified by the latest cert
	// which is the only one that is currently valid
	SbHash []byte
	Cert   *Cert
}

type SignatureResponse struct {
	// id of the site's genesis skipblock
	ID []byte
	// the number of ms elapsed since January 1, 1970 UTC
	Timestamp int64
	// The tree root that was signed:
	Root []byte
	// Proof is an Inclusion proof for the data the client requested:
	Proof Proof
	// Collective signature on Timestamp||hash(treeroot):
	Signature []byte

	// for debug purposes only
	Identifier int

	// public keys of the wkhs that refused to sign off
	PublicsRef []abstract.Point

	// TODO should we return the roster used to sign this message?
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
func NewConfig(s abstract.Suite, fqdn string, threshold int, pub abstract.Point, proxyroster *onet.Roster, owner string, data map[string]*WSconfig) *Config {

	publics := make([]abstract.Point, 0)
	for _, proxy := range proxyroster.List {
		publics = append(publics, proxy.Public)
	}

	// compute the aggregate key of all the signers
	aggPublic := s.Point().Null()
	for i := range publics {
	aggPublic.Add(aggPublic, publics[i])
	}

	return &Config{
		FQDN:        fqdn,
		Threshold:   threshold,
		Device:      map[string]*Device{owner: {Point: pub}},
		//ProxyRoster: proxyroster,
		Aggregate:   aggPublic,
		NbrWkhs: len(publics),
		Data:        data,
	}
}

// Copy returns a deep copy of the AccountList.
func (c *Config) Copy() *Config {
	b, err := network.Marshal(c)
	if err != nil {
		log.Error("Couldn't marshal AccountList:", err)
		return nil
	}
	_, msg, err := network.Unmarshal(b)
	if err != nil {
		log.Error("Couldn't unmarshal AccountList:", err)
	}
	ilNew := msg.(*Config)
	if len(ilNew.Data) == 0 {
		ilNew.Data = make(map[string]*WSconfig)
	}
	return ilNew
}

// Hash makes a cryptographic hash of the configuration-file - this
// can be used as an ID.
func (c *Config) Hash() ([]byte, error) {
	hash := network.Suite.Hash()

	_, err := hash.Write([]byte(c.FQDN))
	if err != nil {
		return nil, err
	}

	_, err = hash.Write(c.BLink)
	if err != nil {
		return nil, err
	}

	var data = []int64{
		int64(c.Timestamp),
		int64(c.Threshold),
		int64(c.NbrWkhs),
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
		b, err := network.Marshal(point)
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
		b, err := network.Marshal(wsconf)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}

	// Include the aggregate public key into the hash (cothority is trusted for issuing proofs of freshness)
	//point := &APoint{Point: c.ProxyRoster.Aggregate}
	point := &APoint{Point: c.Aggregate}
	b, err2 := network.Marshal(point)
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
	log.Lvl3("Setting proposed config's timestamp to: %v", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	return nil
}

func (c *Config) CheckTimeDiff(maxvalue int64) error {
	timestamp := c.Timestamp
	diff := time.Since(time.Unix(0, timestamp*1000000))
	diff_int := diff.Nanoseconds() / 1000000
	if diff_int > maxvalue {
		log.Lvlf2("Stale block (site: %v) - Time difference: %v exceeds the %v interval", c.FQDN, diff, maxvalue)
		return fmt.Errorf("time difference: %v exceeds the %v interval", diff, maxvalue)
	}
	log.Lvlf3("Checking Timestamp: time difference: %v OK", diff)
	return nil
}

// returns 'true' in case the PoF is indeed fresh
func (sr *SignatureResponse) CheckFreshness(maxvalue int64) bool {
	//return true // for debug purposes (flag -race)
	timestamp := sr.Timestamp
	diff := time.Since(time.Unix(0, timestamp*1000000))
	diff_int := diff.Nanoseconds() / 1000000
	if diff_int > maxvalue {
		log.LLvlf2("Stale POF (id: %v) - Time difference: %v exceeds the %v interval", sr.ID, diff, maxvalue)
		return false
	}
	log.Lvlf3("Time difference: %v is within the \"fresh\" %v interval", diff, maxvalue)
	return true
}


// returns true if c is older than c2
func (c *Config) IsOlderConfig(c2 *Config) bool {
	timestamp := c.Timestamp
	diff1 := time.Since(time.Unix(0, timestamp*1000000))
	//diff_int1 := diff.Nanoseconds() / 1000000
	log.Lvl3("Conf1 hash time difference: %v OK", diff1)
	//log.Lvlf2("%v", diff_int1)
	timestamp = c2.Timestamp
	diff2 := time.Since(time.Unix(0, timestamp*1000000))
	//diff_int2 := diff.Nanoseconds() / 1000000
	log.Lvl3("Conf2 hash time difference: %v OK", diff2)
	//log.Lvlf2("%v", diff_int2)
	log.Lvl3("%v", diff1.Nanoseconds())
	log.Lvl3("%v", diff2.Nanoseconds())
	if diff1.Nanoseconds() < diff2.Nanoseconds() {
		return true
	}
	return false
}

func (pof *SignatureResponse) Validate(s abstract.Suite, latestconf *Config, maxdiff int64) error {
	log.Lvlf2("CHECKING POF (identifier: %v)", pof.Identifier)
	// Check whether the 'latest' skipblock is stale or not (by checking the freshness of the PoF)
	isfresh := pof.CheckFreshness(maxdiff)
	if !isfresh {
		log.LLvl2("Stale pof can not be accepted")
		return fmt.Errorf("Stale pof can not be accepted")
	}

	signedmsg := RecreateSignedMsg(pof.Root, pof.Timestamp)

	// check that strictly more that two-thirds of the authoritative wkhs have signed off the POF
	if 3*len(pof.PublicsRef)>latestconf.NbrWkhs-1 {
		log.LLvl2("More that f byzantine failures out of the 3*f+1 WKHs in total")
		return errors.New("More that f byzantine failures out of the 3*f+1 WKHs in total")
	}

	// compute the reduced public aggregate key (all - exception)
	aggReducedPublic := s.Point().Null().Add(s.Point().Null(), latestconf.Aggregate)
	for _, public := range pof.PublicsRef {
		aggReducedPublic.Sub(aggReducedPublic, public)
	}

	err := VerifySig(network.Suite, aggReducedPublic, signedmsg, pof.Signature)
	if err != nil {
		log.LLvlf2("Warm Key Holders' signature doesn't verify")
		return errors.New("Warm Key Holders' signature doesn't verify")
	}
	/*
	publics := make([]abstract.Point, 0)
	for _, proxy := range latestconf.ProxyRoster.List {
		publics = append(publics, proxy.Public)
	}

	err := swupdate.VerifySignature(network.Suite, publics, signedmsg, pof.Signature)
	if err != nil {
		log.LLvlf2("Warm Key Holders' signature doesn't verify")
		return errors.New("Warm Key Holders' signature doesn't verify")
	}
	*/
	// verify inclusion proof
	origmsg, _ := latestconf.Hash()
	log.Lvlf3("for site: %v, %v", latestconf.FQDN, []byte(origmsg))
	log.Lvlf3("root hash: %v", []byte(pof.Root))
	log.Lvlf3("timestamp: %v", pof.Timestamp)
	log.Lvlf3("signature: %v", pof.Signature)
	//log.Lvlf2("proof: %v", pof.Proof)
	validproof := pof.Proof.Check(sha256.New, pof.Root, []byte(origmsg))
	if !validproof {
		log.LLvlf2("Invalid inclusion proof!")
		return errors.New("Invalid inclusion proof!")
	}
	log.LLvl2("Validation of POF DONE")
	return nil
}

func VerifySig(suite abstract.Suite, aggPublic abstract.Point, message, sig []byte) error {
	aggCommitBuff := sig[:32]
	aggCommit := suite.Point()
	if err := aggCommit.UnmarshalBinary(aggCommitBuff); err != nil {
		panic(err)
	}
	sigBuff := sig[32:64]
	sigInt := suite.Scalar().SetBytes(sigBuff)

	aggPublicMarshal, err := aggPublic.MarshalBinary()
	if err != nil {
		return err
	}

	hash := sha512.New()
	hash.Write(aggCommitBuff)
	hash.Write(aggPublicMarshal)
	hash.Write(message)
	buff := hash.Sum(nil)
	k := suite.Scalar().SetBytes(buff)

	// k * -aggPublic + s * B = k*-A + s*B
	// from s = k * a + r => s * B = k * a * B + r * B <=> s*B = k*A + r*B
	// <=> s*B + k*-A = r*B
	minusPublic := suite.Point().Neg(aggPublic)
	kA := suite.Point().Mul(minusPublic, k)
	sB := suite.Point().Mul(nil, sigInt)
	left := suite.Point().Add(kA, sB)

	if !left.Equal(aggCommit) {
		return errors.New("Signature invalid")
	}

	return nil
}

// RecreateSignedMsg is a helper that can be used by the client to recreate the
// message signed by the timestamp service (which is treeroot||timestamp)
func RecreateSignedMsg(treeroot []byte, timestamp int64) []byte {
	timeB := timestampToBytes(timestamp)
	m := make([]byte, len(treeroot)+len(timeB))
	m = append(m, treeroot...)
	m = append(m, timeB...)
	return m
}

func timestampToBytes(t int64) []byte {
	timeBuf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(timeBuf, t)
	return timeBuf
}

func bytesToTimestamp(b []byte) (int64, error) {
	t, err := binary.ReadVarint(bytes.NewReader(b))
	if err != nil {
		return t, err
	}
	return t, nil
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
	_, data, err := network.Unmarshal(sb.Data)
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
		log.Lvlf2("message's len: %v", len(message))
		log.Lvlf2("remainder's len: %v", len(remainder))
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

type IdentityReady struct {
	ID            []byte
	Cothority     *onet.Roster
	FirstIdentity *network.ServerIdentity
	CkhIdentity   *network.ServerIdentity
}

type PushedPublic struct {
}

type StartWebserver struct {
	Roster    *onet.Roster
	Roster_WK *onet.Roster
	Index_CK  int
}

type MinusOne struct {
	Sites *SiteInfo
}

type ConnectClient struct {
	Info []*SiteInfo
}

type MinusOneClient struct {
}

type MinusOneWkh struct {
}

type StartUptWebserver struct {
	Roster *onet.Roster
	Updates int
}

type MinusOneWebserver struct {

}

type MinusOneTimestamper struct {

}

type SetupWkh struct {
	Roster *onet.Roster
	Wkhs *onet.Roster
}

type StartTimestamper struct {
	Roster *onet.Roster
	Wkhs *onet.Roster
}

type CreateEvolveIdentity struct {
	Roster_WK *onet.Roster
	Index_CK int
	FirstIdentity *network.ServerIdentity
	WsIdentity *network.ServerIdentity
	Fqdn string
	Data map[string]*WSconfig
	Evol1 int
	Clients int
	Webservers int
}

type EvolvedIdentity struct {
	ID []byte
	Index_CK int
	Clients int
	Webservers int
}