package service

/*
* The Sicpa service uses a CISC (https://gopkg.in/dedis/cothority.v2/cisc) to store
* key/value pairs on a skipchain.
 */

import (
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
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
func (c *Client) CreateSkipchain(r *onet.Roster, tx Transaction) (*CreateSkipchainResponse, error) {
	reply := &CreateSkipchainResponse{}
	err := c.SendProtobuf(r.List[0], &CreateSkipchain{
		Version:     CurrentVersion,
		Roster:      *r,
		Transaction: tx,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// SetKeyValue sets a key/value pair and returns the created skipblock.
func (c *Client) SetKeyValue(r *onet.Roster, id skipchain.SkipBlockID,
	tx Transaction) (*SetKeyValueResponse, error) {
	reply := &SetKeyValueResponse{}
	err := c.SendProtobuf(r.List[0], &SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: id,
		Transaction: tx,
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
