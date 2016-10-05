package proto

import "github.com/dedis/cothority/sda"

const ProtoName = "RandhounCo"

func init() {
	sda.ProtocolRegisterName(ProtoName, NewRandhounCo)
}

type RandhounCo struct {
}

func NewRandhoundCo(n *sda.TreeNode) (sda.ProtocolInstance, error) {

}
