package evoting

import (
	"github.com/dedis/onet/network"

)

func init() {
	network.RegisterMessage(Ping{})
	network.RegisterMessages(Link{}, LinkReply{})
	network.RegisterMessages(LookupSciper{}, LookupSciperReply{})
	network.RegisterMessages(Open{}, OpenReply{})
	network.RegisterMessages(Cast{}, CastReply{})
	network.RegisterMessages(Shuffle{}, ShuffleReply{})
	network.RegisterMessages(Decrypt{}, DecryptReply{})
	network.RegisterMessages(GetElections{}, GetElectionsReply{})
	network.RegisterMessages(GetBox{}, GetBoxReply{})
	network.RegisterMessages(GetMixes{}, GetMixesReply{})
	network.RegisterMessages(GetPartials{}, GetPartialsReply{})
	network.RegisterMessages(Reconstruct{}, ReconstructReply{})
}