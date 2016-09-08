package swupdate

import (
	"errors"

	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with the software-update service.
type Client struct {
	*sda.Client
	Policy *Policy
	Roster *sda.Roster
	ProjectID
}

type Policy struct {
	Threshold  int
	PublicKeys []string
}

// NewClient instantiates a new communication with the swupdate-client.
func NewClient(r *sda.Roster, policy *Policy) *Client {
	return &Client{
		Client: sda.NewClient(ServiceName),
		Policy: policy,
		Roster: r,
	}
}

/*
TODO:
Add different API-calls here so that the clients can interact with the
swupdate-service.
*/

// SignBuild
func (c *Client) SignBuild(r *sda.Roster) (bool, error) {
	reply, err := c.Send(r.RandomServerIdentity(), &SignBuild{"hello"})
	if e := sda.ErrMsg(reply, err); e != nil {
		return false, e
	}
	sr, ok := reply.Msg.(SignBuildRet)
	if !ok {
		return false, errors.New("This is odd: couldn't cast reply.")
	}
	return sr.OK, nil
}

// CreateProject sets up the skipchain and stores the first policy file.
func (c *Client) CreateProject(r *sda.Roster, p *Policy) error {
	msg, err := c.Send(c.Roster.RandomServerIdentity(), &CreateProject{r, p})
	if err != nil {
		return err
	}
	air := msg.Msg.(CreateProjectRet)
	c.ProjectID = air.ProjectID
	return nil
}
