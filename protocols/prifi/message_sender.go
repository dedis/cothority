package prifi

import (
	"errors"
	"strconv"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

/**
 * This is the struct we need to give PriFi-Lib so it can send messages.
 * It need to implement the "MessageSender interface" defined in prifi_lib/prifi.go
 */
type MessageSender struct {
	tree     *sda.TreeNodeInstance
	relay    *sda.TreeNode
	clients  map[int]*sda.TreeNode
	trustees map[int]*sda.TreeNode
}

func (ms MessageSender) SendToClient(i int, msg interface{}) error {

	if client, ok := ms.clients[i]; ok {
		dbg.Lvl5("Sending a message to client ", i, " (", client.Name(), ") - ", msg)
		return ms.tree.SendTo(client, msg)
	} else {
		e := "Client " + strconv.Itoa(i) + " is unknown !"
		dbg.Error(e)
		return errors.New(e)
	}

	return nil
}

func (ms MessageSender) SendToTrustee(i int, msg interface{}) error {

	if trustee, ok := ms.trustees[i]; ok {
		dbg.Lvl5("Sending a message to trustee ", i, " (", trustee.Name(), ") - ", msg)
		return ms.tree.SendTo(trustee, msg)
	} else {
		e := "Trustee " + strconv.Itoa(i) + " is unknown !"
		dbg.Error(e)
		return errors.New(e)
	}

	return nil
}

func (ms MessageSender) SendToRelay(msg interface{}) error {
	dbg.Lvl5("Sending a message to relay ", " - ", msg)
	return ms.tree.SendTo(ms.relay, msg)
}
