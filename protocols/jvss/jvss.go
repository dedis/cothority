package protocols

import (
	"github.com/dedis/cothority/lib/sda"
)

// SDA-based JVSS , a port of app/shamir

// JVSS Protocol Instance structure holding the information for a longterm JVSS
// signing mechanism
type JVSSProtocol struct {
	// The host we are running on
	Host *sda.Host
	// The tree we are using
	Tree *sda.Tree
	// The EntityList we are using / this is needed to "bypass" the tree
	// structure for the internals communication, when we set up the shares and
	// everything. We directly send our share to everyone else directly by using
	// this entitylist instead of broadcasting into the tree.
	List *sda.EntityList
	// the token for this protocol instance
	Token *sda.Token
}

// NewJVSSProtocol returns a JVSSProtocol with the fields set. You can then
// change the fields or set specific ones etc. If you want to use JVSSProotocol
// directly with SDA, you just need to  register this function:
// ```func(h,t,tok) ProtocolInstance  { return NewJVSSProtocol(h,t,tok) }```
func NewJVSSProtocol(h *sda.Host, t *sda.Tree, tok *sda.Token) *JVSSProtocol {
	return &JVSSProtocol{
		Host:  h,
		Tree:  t,
		List:  t.EntityList,
		Token: tok,
	}
}
