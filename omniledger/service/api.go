package lleap

/*
* The Sicpa service uses a CISC (https://github.com/dedis/cothority/cisc) to store
* key/value pairs on a skipchain.
 */

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

// ServiceName is used for registration on the onet.
const ServiceName = "Lleap"

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new cosi.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// CreateSkipchain sets up a new skipchain to hold the key/value pairs. If
// a key is given, it is used to authenticate towards the cothority.
func (c *Client) CreateSkipchain(r *onet.Roster, key []byte) (*CreateSkipchainResponse, error) {
	reply := &CreateSkipchainResponse{}
	err := c.SendProtobuf(r.List[0], &CreateSkipchain{
		Version: CurrentVersion,
		Roster:  *r,
		Writers: &[][]byte{key},
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// SetKeyValue sets a key/value pair and returns the created skipblock.
func (c *Client) SetKeyValue(r *onet.Roster, id skipchain.SkipBlockID, priv *rsa.PrivateKey,
	key, value []byte) (*SetKeyValueResponse, error) {
	reply := &SetKeyValueResponse{}
	hash := sha256.New()
	hash.Write(key)
	hash.Write(value)
	hashed := hash.Sum(nil)[:]
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed)
	if err != nil {
		return nil, errors.New("couldn't sign: " + err.Error())
	}
	err = c.SendProtobuf(r.List[0], &SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: id,
		Key:         key,
		Value:       value,
		Signature:   sig,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// GetValue returns the value of a key or nil if it doesn't exist.
func (c *Client) GetValue(r *onet.Roster, id skipchain.SkipBlockID, key []byte) (*GetValueResponse, error) {
	reply := &GetValueResponse{}
	err := c.SendProtobuf(r.List[0], &GetValue{
		Version:     CurrentVersion,
		SkipchainID: id,
		Key:         key,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
