package timestamp

import (
	"errors"
	"github.com/dedis/cosi/protocol"
	"github.com/dedis/cothority/sda"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Timestamp"
const regularCosi = "CoSi"

var timestampSID sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceName, newTimestampService)
	timestampSID = sda.ServiceFactory.ServiceID(ServiceName)
	// TODO network.RegisterPacketType(&SkipBlockMap{})
	sda.ProtocolRegisterName(regularCosi, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return protocol.NewCoSi(n)
	})
}

type Service struct {
	*sda.ServiceProcessor
	path string
	// testVerify is set to true if a verification happened - only for testing
	testVerify bool
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	var pi sda.ProtocolInstance
	var err error
	if tn.ProtocolName() != regularCosi {
		return nil, errors.New("Expected " + regularCosi + " as protocol but got " + tn.ProtocolName())
	}
	n, err := protocol.NewCoSi(tn)
	if err != nil {
		return nil, err
	}
	// TODO here we
	/*switch tn.ProtocolName() {
	case "Propagate":
		pi, err = manage.NewPropagateProtocol(tn)
		if err != nil {
			return nil, err
		}
		pi.(*manage.Propagate).RegisterOnData(s.PropagateSkipBlock)
	case skipchainBFT:
		pi, err = bftcosi.NewBFTCoSiProtocol(tn, s.bftVerify)
	}*/
	sda.ProtocolRegisterName(regularCosi, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return protocol.NewCoSi(n)
	})
	return pi, err
}
