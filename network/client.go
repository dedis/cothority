package network

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dedis/crypto/config"
)

func init() {
	RegisterPacketType(&StatusRet{})
}

// Client is used for the external API of services.
// NOTE: This interface is likely to be removed to be replaced by a full
// pledged REST HTTP Api directly connected to the sda/services.
type Client struct {
	connector func(own, remote *ServerIdentity) (Conn, error)
}

func newClient(c func(own, remote *ServerIdentity) (Conn, error)) *Client {
	return &Client{c}
}

var baseID uint64
var baseIDLock sync.Mutex

var timeoutResponse = 10 * time.Second

// Send will send the message to the destination service and return the
// reply.
// In case of an error, it returns a nil-packet and the error.
// Send will timeout and return an error if it has not received any response
// under 10 sec.
func (cl *Client) Send(dst *ServerIdentity, msg Body) (*Packet, error) {
	kp := config.NewKeyPair(Suite)
	// Use a unique ID for each connection.
	baseIDLock.Lock()
	id := baseID
	baseID++
	baseIDLock.Unlock()
	sid := NewServerIdentity(kp.Public, NewAddress(dst.Address.ConnType(),
		"client:"+strconv.FormatUint(id, 10)))

	c, err := cl.connector(sid, dst)
	if err != nil {
		return nil, fmt.Errorf("Could not connect %x", err)
	}
	defer c.Close()

	if err := c.Send(sid); err != nil {
		return nil, err
	}

	msgCh := make(chan Packet)
	errCh := make(chan error)
	go func() {
		if err := c.Send(msg); err != nil {
			errCh <- err
			return
		}
		p, err := c.Receive()
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
	case <-time.After(timeoutResponse):
		return nil, errors.New("Timeout on sending message")
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
	if statusStr == "" {
		return nil
	}
	return errors.New("Remote-error: " + statusStr)
}
