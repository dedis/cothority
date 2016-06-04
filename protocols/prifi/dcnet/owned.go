package dcnet

import (
	"bytes"
	"crypto/rand"

	"github.com/dedis/crypto/abstract"
)

type ownedCoder struct {
	suite abstract.Suite

	// Length of Key and MAC part of verifiable DC-net point
	keylen, maclen int

	// Verifiable DC-nets secrets shared with each peer.
	vkeys []abstract.Secret

	// The sum of all our verifiable DC-nets secrets.
	vkey abstract.Secret

	// Pseudorandom DC-nets ciphers shared with each peer.
	// On clients, there is one DC-nets cipher per trustee.
	// On trustees, there ois one DC-nets cipher per client.
	dcciphers []abstract.Cipher

	// Pseudorandom stream
	random abstract.Cipher

	// Decoding state, used only by the relay
	point  abstract.Point
	pnull  abstract.Point // neutral/identity element
	xorbuf []byte
}

// OwnedCoderFactory creates a DC-net cell coder for "owned" cells:
// cells having a single owner identified by a public pseudonym key.
//
// This CellCoder upports variable-length payloads.
// For small payloads that can be embedded into half a Point,
// the encoding consists of a single verifiable DC-net point.
// For larger payloads, we use one verifiable DC-net point
// to transmit a key and a MAC for the associated variable-length,
// symmetric-key crypto based part of the cell.
func OwnedCoderFactory() CellCoder {
	return new(ownedCoder)
}

// For now just hard-code a single choice of trap-encoding word size
// for maximum simplicity and efficiency.
// We'll evaluate later whether we need to make it dynamic.
const wordbits = 32

type word uint32

///// Common methods /////

// Compute the size of the symmetric AES-encoded part of an encoded ciphertext.
func (c *ownedCoder) symmCellSize(payloadlen int) int {

	// If data fits in the space reserved for the key
	// in the verifiable DC-net point,
	// we can just inline the data in the point instead of the key.
	// (We'll still use the MAC part of the point for validation.)
	if payloadlen <= c.keylen {
		return 0
	}

	// Otherwise the point is used to hold an encryption key and a MAC,
	// and the payload is symmetric-key encrypted.
	// XXX trap encoding
	return payloadlen

	// Compute number of payload words we will need for trap-encoding.
	// words := (payloadlen*8 + wordbits-1) / wordbits

	// Number of bytes worth of trap-encoded payload words,
	// after padding the payload up to the next word boundary.
	// wordbytes := (words*wordbits+7)/8

	// We'll need to follow the payload with an inversion bitmask,
	// one bit per trap-encoded word.
	// invbytes := (words+7)/8

	// Total cell is the verifiable DC-nets point, plus payload,
	// plus inversion bitmask.  (XXX plus ZKP/signature.)
	// return c.suite.PointLen() + wordbytes + invbytes
}

func (c *ownedCoder) commonSetup(suite abstract.Suite) {
	c.suite = suite

	// Divide the embeddable data in the verifiable point
	// between an encryption key and a MAC check
	c.keylen = suite.Cipher(nil).KeySize()
	c.maclen = suite.Point().PickLen() - c.keylen
	if c.maclen < c.keylen*3/4 {
		panic("misconfigured ciphersuite: MAC too small!")
	}

	randkey := make([]byte, suite.Cipher(nil).KeySize())
	rand.Read(randkey)
	c.random = suite.Cipher(randkey)
}

///// Client methods /////

func (c *ownedCoder) ClientCellSize(payloadlen int) int {

	// Clients must produce a point plus the symmetric ciphertext
	return c.suite.PointLen() + c.symmCellSize(payloadlen)
}

func (c *ownedCoder) ClientSetup(suite abstract.Suite,
	sharedsecrets []abstract.Cipher) {
	c.commonSetup(suite)
	keysize := suite.Cipher(nil).KeySize()

	// Use the provided shared secrets to seed
	// a pseudorandom public-key encryption secret, and
	// a pseudorandom DC-nets cipher shared with each peer.
	npeers := len(sharedsecrets)
	c.vkeys = make([]abstract.Secret, npeers)
	c.vkey = suite.Secret()
	c.dcciphers = make([]abstract.Cipher, npeers)
	for i := range sharedsecrets {
		c.vkeys[i] = suite.Secret().Pick(sharedsecrets[i])
		c.vkey.Add(c.vkey, c.vkeys[i])
		key := make([]byte, keysize)
		sharedsecrets[i].Partial(key, key, nil)
		c.dcciphers[i] = suite.Cipher(key)
	}
}

func (c *ownedCoder) ClientEncode(payload []byte, payloadlen int,
	history abstract.Cipher) []byte {

	// Compute the verifiable blinding point for this cell.
	// To protect clients from equivocation by relays,
	// we choose the blinding generator for each cell pseudorandomly
	// based on the history of all past downstream messages
	// the client has received from the relay.
	// If any two honest clients disagree on this history,
	// they will produce encryptions based on unrelated generators,
	// rendering the cell unintelligible,
	// so that any data the client might be sending based on
	// having seen a divergent history gets suppressed.
	p := c.suite.Point()
	p.Pick(nil, history)
	p.Mul(p, c.vkey)

	// Encode the payload data, if any.
	payout := make([]byte, c.symmCellSize(payloadlen))
	if payload != nil {
		// We're the owner of this cell.
		if len(payload) <= c.keylen {
			c.inlineEncode(payload, p)
		} else {
			c.ownerEncode(payload, payout, p)
		}
	}

	// XOR the symmetric DC-net streams into the payload part
	for i := range c.dcciphers {
		c.dcciphers[i].XORKeyStream(payout, payout)
	}

	// Build the full cell ciphertext
	out, _ := p.MarshalBinary()
	out = append(out, payout...)
	return out
}

func (c *ownedCoder) inlineEncode(payload []byte, p abstract.Point) {

	// Hash the cleartext payload to produce the MAC
	h := c.suite.Hash()
	h.Write(payload)
	mac := h.Sum(nil)[:c.maclen]

	// Embed the payload and MAC into a Point representing the message
	hdr := append(payload, mac...)
	mp, _ := c.suite.Point().Pick(hdr, c.random)

	// Add this to the blinding point we already computed to transmit.
	p.Add(p, mp)
}

func (c *ownedCoder) ownerEncode(payload, payout []byte, p abstract.Point) {

	// XXX trap-encode

	// Pick a fresh random key with which to encrypt the payload
	key := make([]byte, c.keylen)
	c.random.XORKeyStream(key, key)

	// Encrypt the payload with it
	c.suite.Cipher(key).XORKeyStream(payout, payload)

	// Compute a MAC over the encrypted payload
	h := c.suite.Hash()
	h.Write(payout)
	mac := h.Sum(nil)[:c.maclen]

	// Combine the key and the MAC into the Point for this cell header
	hdr := append(key, mac...)
	if len(hdr) != p.PickLen() {
		panic("oops, length of key+mac turned out wrong")
	}
	mp, _ := c.suite.Point().Pick(hdr, c.random)

	// Add this to the blinding point we already computed to transmit.
	p.Add(p, mp)
}

///// Trustee methods /////

func (c *ownedCoder) TrusteeCellSize(payloadlen int) int {

	// Trustees produce only the symmetric ciphertext, if any
	return c.symmCellSize(payloadlen)
}

// Setup the trustee side.
// May produce coder configuration info to be passed to the relay,
// which will become available to the RelaySetup() method below.
func (c *ownedCoder) TrusteeSetup(suite abstract.Suite,
	clientstreams []abstract.Cipher) []byte {

	// Compute shared secrets
	c.ClientSetup(suite, clientstreams)

	// Release the negation of the composite shared verifiable secret
	// to the relay, so the relay can decode each cell's header.
	c.vkey.Neg(c.vkey)
	rv, _ := c.vkey.MarshalBinary()
	return rv
}

func (c *ownedCoder) TrusteeEncode(payloadlen int) []byte {

	// Trustees produce only symmetric DC-nets streams
	// for the payload portion of each cell.
	payout := make([]byte, payloadlen) // XXX trap expansion
	for i := range c.dcciphers {
		c.dcciphers[i].XORKeyStream(payout, payout)
	}
	return payout
}

///// Relay methods /////

func (c *ownedCoder) RelaySetup(suite abstract.Suite, trusteeinfo [][]byte) {

	c.commonSetup(suite)

	// Decode the trustees' composite verifiable DC-net secrets
	ntrustees := len(trusteeinfo)
	c.vkeys = make([]abstract.Secret, ntrustees)
	c.vkey = suite.Secret()
	for i := range c.vkeys {
		c.vkeys[i] = c.suite.Secret()
		c.vkeys[i].UnmarshalBinary(trusteeinfo[i])
		c.vkey.Add(c.vkey, c.vkeys[i])
	}

	c.pnull = c.suite.Point().Null()
}

func (c *ownedCoder) DecodeStart(payloadlen int, history abstract.Cipher) {

	// Compute the composite trustees-side verifiable DC-net unblinder
	// based on the appropriate message history.
	p := c.suite.Point()
	p.Pick(nil, history)
	p.Mul(p, c.vkey)
	c.point = p

	// Initialize the symmetric ciphertext XOR buffer
	if payloadlen > c.keylen {
		c.xorbuf = make([]byte, payloadlen)
	}
}

func (c *ownedCoder) DecodeClient(slice []byte) {
	// Decode and add in the point in the slice header
	plen := c.suite.PointLen()
	p := c.suite.Point()
	if err := p.UnmarshalBinary(slice[:plen]); err != nil {
		println("warning: error decoding point")
	}
	c.point.Add(c.point, p)

	// Combine in the symmetric ciphertext streams
	if c.xorbuf != nil {
		slice = slice[plen:]
		for i := range slice {
			c.xorbuf[i] ^= slice[i]
		}
	}
}

func (c *ownedCoder) DecodeTrustee(slice []byte) {

	// Combine in the trustees' symmetric ciphertext streams
	if c.xorbuf != nil {
		for i := range slice {
			c.xorbuf[i] ^= slice[i]
		}
	}
}

func (c *ownedCoder) DecodeCell() []byte {

	if c.point.Equal(c.pnull) {
		//println("no transmission in cell")
		return nil
	}

	// Decode the header from the decrypted point.
	hdr, err := c.point.Data()
	if err != nil || len(hdr) < c.maclen {
		println("warning: undecipherable cell header")
		return nil // XXX differentiate from no transmission?
	}

	if c.xorbuf == nil { // short inline cell
		return c.inlineDecode(hdr)
	} else { // long payload cell
		return c.ownerDecode(hdr)
	}
}

func (c *ownedCoder) inlineDecode(hdr []byte) []byte {

	// Split the inline payload from the MAC
	datlen := len(hdr) - c.maclen
	dat := hdr[:datlen]
	mac := hdr[datlen:]

	// Check the MAC
	h := c.suite.Hash()
	h.Write(dat)
	check := h.Sum(nil)[:c.maclen]
	if !bytes.Equal(mac, check) {
		println("warning: MAC check failed on inline cell")
		return nil
	}

	return dat
}

func (c *ownedCoder) ownerDecode(hdr []byte) []byte {

	// Split the payload encryption key from the MAC
	keylen := len(hdr) - c.maclen
	if keylen != c.keylen {
		println("warning: wrong size cell encryption key")
		return nil
	}
	key := hdr[:keylen]
	mac := hdr[keylen:]
	dat := c.xorbuf

	// Check the MAC on the still-encrypted data
	h := c.suite.Hash()
	h.Write(dat)
	check := h.Sum(nil)[:c.maclen]
	if !bytes.Equal(mac, check) {
		println("warning: MAC check failed on out-of-line cell")
		return nil
	}

	// Decrypt and return the payload data
	c.suite.Cipher(key).XORKeyStream(dat, dat)
	return dat
}
