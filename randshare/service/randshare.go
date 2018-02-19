package service

import ()

//contains all the code to run protocol ?

// ServiceName is the name to refer to the RandShare service
const ServiceName = "RandShare"

func init() {
	onet.GlobalProtocolRegister(ServiceName, NewRandShareService)
	//TODO register request + response
}

// RandShare is the service
type RandShare struct {
	*onet.ServiceProcessor
}

func newRandShareService(c *onet.Context) (onet.Service, error) {
	//TODO
}

/* in randshare_with_pvss.go :
func NewRandShare(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &RandShare{
		TreeNodeInstance: n,
	}
	err := t.RegisterHandlers(t.HandleA1, t.HandleV1, t.HandleR1)
	return t, err
}*/
