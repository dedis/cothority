package skipchain

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"time"
)

func init() {
}

func SendActiveAdd(e *network.Entity, prev, new *SkipBlock) (*ActiveAddRet, error) {
	msg, err := NetworkSend(nil, e, &ActiveAdd{prev, new})
	if err != nil {
		return nil, err
	}
	aar, ok := msg.Msg.(ActiveAddRet)
	if !ok {
		return nil, ErrMsg(msg, err)
	}
	return &aar, nil
}
