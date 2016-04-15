package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"io"

	"github.com/dedis/crypto/abstract"
)

// ReadPub64 a public point to a base64 representation
func ReadPub64(suite abstract.Suite, r io.Reader) (abstract.Point, error) {
	public := suite.Point()
	dec := base64.NewDecoder(base64.StdEncoding, r)
	err := suite.Read(dec, &public)
	return public, err
}

// WritePub64 writes a public point to a base64 representation
func WritePub64(suite abstract.Suite, w io.Writer, point abstract.Point) error {
	enc := base64.NewEncoder(base64.StdEncoding, w)
	return write64(suite, enc, point)
}

func write64(suite abstract.Suite, wc io.WriteCloser, data ...interface{}) error {
	if err := suite.Write(wc, data); err != nil {
		return err
	}
	return wc.Close()
}

// WriteSecret64 converts a secret key to a Base64-string
func WriteSecret64(suite abstract.Suite, w io.Writer, secret abstract.Secret) error {
	enc := base64.NewEncoder(base64.StdEncoding, w)
	return write64(suite, enc, secret)
}

// ReadSecret64 takes a Base64-encoded secret and returns that secret,
// optionally an error
func ReadSecret64(suite abstract.Suite, r io.Reader) (abstract.Secret, error) {
	sec := suite.Secret()
	dec := base64.NewDecoder(base64.StdEncoding, r)
	err := suite.Read(dec, &sec)
	return sec, err
}

// PubHex converts a Public point to a hexadecimal representation
func PubHex(suite abstract.Suite, point abstract.Point) (string, error) {
	pbuf, err := point.MarshalBinary()
	return hex.EncodeToString(pbuf), err
}

// ReadPubHex reads a hexadecimal representation of a public point and convert it to the
// right struct
func ReadPubHex(suite abstract.Suite, s string) (abstract.Point, error) {
	encoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	point := suite.Point()
	err = point.UnmarshalBinary(encoded)
	return point, err
}

// SecretHex encodes a secret to hexadecimal
func SecretHex(suite abstract.Suite, secret abstract.Secret) (string, error) {
	sbuf, err := secret.MarshalBinary()
	return hex.EncodeToString(sbuf), err
}

// ReadSecretHex reads a secret in hexadecimal from string
func ReadSecretHex(suite abstract.Suite, str string) (abstract.Secret, error) {
	enc, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	sec := suite.Secret()
	err = sec.UnmarshalBinary(enc)
	return sec, err
}
