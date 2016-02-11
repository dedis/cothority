package cosi

import (
	"github.com/dedis/cothority/lib/sda"
)

// ExceptionProtocol handles the exception mechanisms.
type ProtocolException struct {
	*sda.Node
	cosi *ProtocolCosi
}

func NewProtocolException(node *sda.Node) (*ProtocolException, error) {
	pe := &ProtocolException{
		Node: node,
	}
	cosi, err := NewProtocolCosi(node)
	pe.cosi = cosi
	return pe, err
}

func (pe *ProtocolException) listen() {
}

func (pe *ProtocolException) announcementMiddleware(announce AnnouncementHook) {

}
