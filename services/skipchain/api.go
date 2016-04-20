package skipchain

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/dbg"
	"golang.org/x/net/context"
	"errors"
	"time"
	"github.com/dedis/crypto/config"
)

func init(){
}

func SendActiveAdd(e *network.Entity, prev, new *SkipBlock)(*ActiveAddRet, error){
	msg, err := NetworkSend(nil, e, &ActiveAdd{prev, new})
	if err != nil{
		return nil, err
	}
	aar, ok := msg.Msg.(ActiveAddRet)
	if !ok{
		return nil, ErrMsg(msg, err)
	}
	return &aar, nil
}

