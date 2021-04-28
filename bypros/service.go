// Package bypros contains the Byzcoin Proxy Service
package bypros

import (
	"fmt"
	"log"
	"net/url"
	"strconv"

	"go.dedis.ch/cothority/v3/bypros/storage"
	"go.dedis.ch/cothority/v3/bypros/storage/sqlstore"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

// ServiceName can be used to refer to this service
const ServiceName = "ByzcoinProxy"

// storageFac defines the default storage factory. We define it here to be able
// to change it for tests.
var storageFac = sqlstore.NewSQL

func init() {
	_, err := onet.RegisterNewService(ServiceName, newProxyService)
	if err != nil {
		log.Fatal(err)
	}
}

// Service holds the Proxy service
type Service struct {
	*onet.ServiceProcessor

	follow     chan struct{}
	following  bool
	stopFollow chan struct{}

	// normalUnfollow is set when the Unfollow method is called so we can check,
	// when the websocket connection closes, if it's normal or not.
	normalUnfollow chan struct{}
	followReq      *Follow

	storage storage.Storage

	// the proxy should only use one skipchain. We keep track of it there.
	scID skipchain.SkipBlockID
}

// newProxyService returns a new proxy service for onet
func newProxyService(c *onet.Context) (onet.Service, error) {
	sqlStorage, err := storageFac()
	if err != nil {
		return nil, xerrors.Errorf("failed to get storage: %v", err)
	}

	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		follow:           make(chan struct{}, 1),
		storage:          sqlStorage,
	}

	// initial token to start following
	s.follow <- struct{}{}

	err = s.RegisterHandlers(s.Follow, s.Unfollow, s.Query)
	if err != nil {
		return nil, xerrors.Errorf("failed to register handlers: %v", err)
	}

	err = s.RegisterStreamingHandler(s.CatchUP)
	if err != nil {
		return nil, xerrors.Errorf("failed to register streaming handler: %v", err)
	}

	return s, nil
}

// getWsAddr return the needed ws address
func getWsAddr(si *network.ServerIdentity) (string, error) {
	url, err := getClientAddr(si)
	if err != nil {
		return "", xerrors.Errorf("failed to get client addr: %v", err)
	}

	protocol := "wss"
	if url.Port() != "443" {
		protocol = "ws"
	}

	return fmt.Sprintf("%s://%s%s", protocol, url.Host, url.Path), nil
}

func getClientAddr(si *network.ServerIdentity) (*url.URL, error) {
	if si.URL != "" {
		result, err := getClientAddrFromURL(si.URL)
		if err != nil {
			return nil, xerrors.Errorf("failed to get from URL: %v", err)
		}

		return result, nil
	}

	port, err := strconv.ParseUint(si.Address.Port(), 10, 16)
	if err != nil {
		return nil, xerrors.Errorf("failed to pase port: %v", err)
	}

	u, err := url.Parse(fmt.Sprintf("//%s:%d", si.Address.Host(), port+1))
	if err != nil {
		return nil, xerrors.Errorf("failed to parse url: %v", err)
	}

	return u, nil
}

func getClientAddrFromURL(urlStr string) (*url.URL, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, xerrors.Errorf("unable to parse URL: %v", err)
	}

	if !u.IsAbs() {
		return nil, xerrors.Errorf("URL is not absolute")
	}

	var port uint64

	portStr := u.Port()
	if portStr == "" {
		port, err = schemeToPort(u.Scheme)
		if err != nil {
			return nil, xerrors.Errorf("failed to get port: %v", err)
		}
	} else {
		port, err = strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return nil, xerrors.Errorf("URL doesn't contain a valid port: %v", err)
		}
	}

	u.Host = fmt.Sprintf("%s:%d", u.Hostname(), port)

	return u, nil
}

// schemeToPort returns the port corresponding to the given scheme, much like
// netdb.
func schemeToPort(name string) (uint64, error) {
	switch name {
	case "http":
		return 80, nil
	case "https":
		return 443, nil
	default:
		return 0, xerrors.Errorf("no such scheme: %v", name)
	}
}
