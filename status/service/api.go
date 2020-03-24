package status

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"time"
)

// Client is a structure to communicate with status service
type Client struct {
	*onet.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// Request sends requests to all other members of network and creates client.
func (c *Client) Request(dst *network.ServerIdentity) (*Response, error) {
	resp := &Response{}
	err := c.SendProtobuf(dst, &Request{}, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CheckConnectivity sends a message from all nodes to all nodes in the list
// and checks if all messages are received correctly. If findFaulty == true,
// then the service will try very hard to get a list of nodes that can
// communicate with each other.
//
// The return value of this call is the set of nodes that can communicate
// with each other.
func (c *Client) CheckConnectivity(priv kyber.Scalar, list []*network.ServerIdentity,
	timeout time.Duration, findFaulty bool) ([]*network.ServerIdentity, error) {
	conn := &CheckConnectivity{
		List:       list,
		Timeout:    int64(timeout),
		FindFaulty: findFaulty,
		Time:       time.Now().Unix(),
	}
	hash, err := conn.hash()
	if err != nil {
		return nil, errors.New("couldn't hash message: " + err.Error())
	}
	conn.Signature, err = schnorr.Sign(cothority.Suite, priv, hash)
	if err != nil {
		return nil, errors.New("couldn't sign message: " + err.Error())
	}
	resp := &CheckConnectivityReply{}
	err = c.SendProtobuf(list[0], conn, resp)
	if err != nil {
		return nil, errors.New("failed to send CheckConnectivity: " + err.Error())
	}
	return resp.Nodes, nil
}

func (c *CheckConnectivity) hash() ([]byte, error) {
	hash := sha256.New()
	timeBuf := make([]byte, 8)

	binary.LittleEndian.PutUint64(timeBuf, uint64(c.Time))
	hash.Write(timeBuf)

	binary.LittleEndian.PutUint64(timeBuf, uint64(c.Timeout))
	hash.Write(timeBuf)

	if c.FindFaulty {
		hash.Write([]byte{1})
	} else {
		hash.Write([]byte{0})
	}

	for _, si := range c.List {
		buf, err := protobuf.Encode(si)
		if err != nil {
			return nil, errors.New("couldn't encode ServerIdentity: " + err.Error())
		}
		hash.Write(buf)
	}
	return hash.Sum(nil), nil
}
