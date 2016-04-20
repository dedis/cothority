package skipchain

import (
	"errors"
	"time"

	"encoding/json"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

/*
A simple client structure to be used when wanting to connect to services. It
holds the private and public key and allows to connect to a service through
the network.
The error-handling is done using the ErrorRet structure which can be returned
in place of the standard reply. The Client.Send method will catch that and return
 the appropriate error.
*/

// Client for a service
type Client struct {
	Private abstract.Secret
	*network.Entity
	ServiceID sda.ServiceID
}

// NewClient returns a random client using the service s
func NewClient(s string) *Client {
	kp := config.NewKeyPair(network.Suite)
	return &Client{
		Entity:    network.NewEntity(kp.Public, ""),
		Private:   kp.Secret,
		ServiceID: sda.ServiceFactory.ServiceID(s),
	}
}

// NetworkSend opens the connection to 'dst' and sends the message 'req'. The
// reply is returned, or an error if the timeout of 10 seconds is reached.
func (c *Client) Send(dst *network.Entity, req []byte) (*network.Message, error) {
	client := network.NewSecureTCPHost(c.Private, c.Entity)

	// Connect to the root
	dbg.Lvl4("Opening connection to", dst)
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	serviceReq := &sda.ClientRequest{
		Service: c.ServiceID,
		Type:    CosiRequestType,
		Data:    json.RawMessage(req),
	}
	pchan := make(chan network.Message)
	go func() {
		// send the request
		dbg.Lvlf3("Sending request %+v", serviceReq)
		if err := con.Send(context.TODO(), serviceReq); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl4("Waiting for the response")
		// wait for the response
		packet, err := con.Receive(context.TODO())
		if err != nil {
			close(pchan)
			return
		}
		pchan <- packet
	}()
	select {
	case response := <-pchan:
		dbg.Lvlf5("Response: %+v", response)
		// Catch an eventual error
		err := ErrMsg(&response, nil)
		if err != nil {
			return nil, err
		}
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.Message{}, errors.New("Timeout on signing")
	}
}

// BinaryMarshaler can be used to store the client in a configuration-file
func (c *Client) BinaryMarshaler() ([]byte, error) {
	dbg.Fatal("Not yet implemented")
	return nil, nil
}

// BinaryUnmarshaler sets the different values from a byte-slice
func (c *Client) BinaryUnmarshaler(b []byte) error {
	dbg.Fatal("Not yet implemented")
	return nil
}

// ErrorRet is used when an error is returned - Error may be nil
type ErrorRet struct {
	Error error
}

// ErrMsg converts a combined err and status-message to an error. It
// returns either the error, or the errormsg, if there is one.
func ErrMsg(em *network.Message, err error) error {
	if err != nil {
		return err
	}
	errMsg, ok := em.Msg.(ErrorRet)
	if !ok {
		return nil
	}
	errMsgStr := errMsg.Error.Error()
	if errMsgStr != "" {
		return errors.New("Remote-error: " + errMsgStr)
	}
	return nil
}
