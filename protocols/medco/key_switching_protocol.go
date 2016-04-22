package medco

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
)

type KeySwitchable interface {
	SwitchForKey(public abstract.Point)
}


func init() {

}

type KeySwitchingProtocol struct {
	*sda.Node

	FeedbackChannel chan KeySwitchable


}