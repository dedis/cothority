package skipchain

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/dbg"
	"golang.org/x/net/context"
	"github.com/dedis/crypto/config"
	"time"
	"errors"
)

// Client for a service
type Client struct{
	Private abstract.Secret
	*network.Entity
	Name string
}

// NewClient returns a random client using the name
func NewClient(n string)*Client{
	kp := config.NewKeyPair(network.Suite)
	return &Client{
		Entity: network.NewEntity(kp.Public, ""),
		Private: kp.Secret,
		Name: n,
	}
}

// NetworkSend opens the connection to 'dst' and sends the message 'req'. The
// reply is returned, or an error if the timeout of 10 seconds is reached.
func (c *Client)Send(dst *network.Entity, req network.ProtocolMessage) (*network.Message, error) {
	client := network.NewSecureTCPHost(c.Private, c.Entity)

	// Connect to the root
	dbg.Lvl4("Opening connection to", dst)
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return &network.Message{}, err
	}

	pchan := make(chan network.Message)
	go func() {
		// send the request
		dbg.Lvlf3("Sending request %+v", req)
		if err := con.Send(context.TODO(), req); err != nil {
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
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.Message{}, errors.New("Timeout on signing")
	}
}

// BinaryMarshaler can be used to store the client in a configuration-file
func (c *Client)BinaryMarshaler()([]byte, error){
	dbg.Fatal("Not yet implemented")
	return nil, nil
}

// BinaryUnmarshaler sets the different values from a byte-slice
func (c *Client)BinaryUnmarshaler(b []byte)error{
	dbg.Fatal("Not yet implemented")
	return nil
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

