package network

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/dedis/crypto/config"
)

// Client is an interface which is a simpler version of a Router. The main use
// for a Client is to directly Send something and get a result back. It is used
// intensively by Services to have a easy external API.
// Two implementations are done: TcpClient to use for applications and
// deployement,and localClient to use for local testing alongside with
// LocalRouter.
// NOTE: This interface is likely to be removed to be replaced by a full
// pledged REST HTTP Api directly connected to the sda/services.
type Client struct {
	connector func(own, remote *ServerIdentity) (Conn, error)
}

func newClient(c func(own, remote *ServerIdentity) (Conn, error)) *Client {
	return &Client{c}
}

// Send will send the message to the destination service and return the
// reply.
// The error-handling is done using the ErrorRet structure which can be returned
// in place of the standard reply. This method will catch that and return
// the appropriate error as a network.Packet.
func (cl *Client) Send(dst *ServerIdentity, msg Body) (*Packet, error) {
	kp := config.NewKeyPair(Suite)
	id := rand.Intn(256) + 1
	sid := NewServerIdentity(kp.Public, NewLocalAddress("localhost:"+strconv.Itoa(id)))

	var c Conn
	var err error
	for i := 0; i < MaxRetry; i++ {
		c, err = cl.connector(sid, dst)
		if err == nil {
			break
		} else if i == MaxRetry-1 {
			return nil, fmt.Errorf("Could not connect", err)
		}
		time.Sleep(WaitRetry)
	}
	defer c.Close()

	if err := negotiateOpen(sid, dst, c); err != nil {
		return nil, err
	}

	msgCh := make(chan Packet)
	errCh := make(chan error)
	go func() {
		if err := c.Send(context.TODO(), msg); err != nil {
			errCh <- err
			return
		}
		p, err := c.Receive(context.TODO())
		if ret := ErrMsg(&p, err); ret != nil {
			errCh <- ret
		} else {
			msgCh <- p
		}
	}()

	select {
	case resp := <-msgCh:
		return &resp, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(time.Second * 10):
		return &Packet{}, errors.New("Timeout on sending message")
	}
}

// StatusRet is used when a status is returned - mostly an error
type StatusRet struct {
	Status string
}

// StatusOK is used when there is no error but nothing to return
var StatusOK = &StatusRet{""}

// ErrMsg converts a combined err and status-message to an error. It
// returns either the error, or the errormsg, if there is one.
func ErrMsg(em *Packet, err error) error {
	if err != nil {
		return err
	}
	status, ok := em.Msg.(StatusRet)
	if !ok {
		return nil
	}
	statusStr := status.Status
	if statusStr != "" {
		return errors.New("Remote-error: " + statusStr)
		return nil
	}
	return nil
}

func init() {
	RegisterMessageType(&StatusRet{})
}
