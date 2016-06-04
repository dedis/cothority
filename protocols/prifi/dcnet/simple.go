package dcnet

import (
	"github.com/dedis/crypto/abstract"
)

type simpleCoder struct {
	suite abstract.Suite

	// Pseudorandom DC-nets ciphers shared with each peer.
	// On clients, there is one DC-nets cipher per trustee.
	// On trustees, there is one DC-nets cipher per client.
	dcciphers []abstract.Cipher

	xorbuf []byte
}

// Simple DC-net encoder providing no disruption or equivocation protection,
// for experimentation and baseline performance evaluations.
func SimpleCoderFactory() CellCoder {
	return new(simpleCoder)
}

///// Client methods /////

func (c *simpleCoder) ClientCellSize(payloadlen int) int {
	return payloadlen // no expansion
}

func (c *simpleCoder) ClientSetup(suite abstract.Suite,
	sharedsecrets []abstract.Cipher) {
	c.suite = suite
	keysize := suite.Cipher(nil).KeySize()

	// Use the provided shared secrets to seed
	// a pseudorandom DC-nets ciphers shared with each peer.
	npeers := len(sharedsecrets)
	c.dcciphers = make([]abstract.Cipher, npeers)
	for i := range sharedsecrets {
		key := make([]byte, keysize)
		sharedsecrets[i].Partial(key, key, nil)
		c.dcciphers[i] = suite.Cipher(key)
	}
}

func (c *simpleCoder) ClientEncode(payload []byte, payloadlen int,
	history abstract.Cipher) []byte {

	if payload == nil {
		payload = make([]byte, payloadlen)
	}
	for i := range c.dcciphers {
		c.dcciphers[i].XORKeyStream(payload, payload)
	}
	return payload
}

///// Trustee methods /////

func (c *simpleCoder) TrusteeCellSize(payloadlen int) int {
	return payloadlen // no expansion
}

func (c *simpleCoder) TrusteeSetup(suite abstract.Suite,
	sharedsecrets []abstract.Cipher) []byte {
	c.ClientSetup(suite, sharedsecrets)
	return nil
}

func (c *simpleCoder) TrusteeEncode(payloadlen int) []byte {
	emptyCode := abstract.Cipher{}
	return c.ClientEncode(nil, payloadlen, emptyCode)
}

///// Relay methods /////

func (c *simpleCoder) RelaySetup(suite abstract.Suite, trusteeinfo [][]byte) {
	// nothing to do
}

func (c *simpleCoder) DecodeStart(payloadlen int, history abstract.Cipher) {
	c.xorbuf = make([]byte, payloadlen)
}

func (c *simpleCoder) DecodeClient(slice []byte) {
	for i := range slice {
		c.xorbuf[i] ^= slice[i]
	}
}

func (c *simpleCoder) DecodeTrustee(slice []byte) {
	c.DecodeClient(slice)
}

func (c *simpleCoder) DecodeCell() []byte {
	return c.xorbuf
}
