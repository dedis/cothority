package bypros

import (
	"context"
	"fmt"

	"github.com/gorilla/websocket"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

var overlayClient OverlayClient = onetOverlay{
	onet.NewClient(cothority.Suite, ServiceName),
}

// OverlayClient defines the primitives needed by our client. That's all we need
// from the outer world.
type OverlayClient interface {
	SendProtobuf(dst *network.ServerIdentity, msg interface{}, ret interface{}) error
	OpenWS(url string) (WsHandler, error)
}

// WsHandler defines the primitives to handle a websocket connection.
type WsHandler interface {
	Close() error
	Write(messageType int, data []byte) error
	Read() (messageType int, p []byte, err error)
}

// NewClient creates a new proxy client
func NewClient() *Client {
	return &Client{OverlayClient: overlayClient}
}

// Client defines a proxy client
//
// - implements OverlayClient
type Client struct {
	OverlayClient
}

// Follow sends a request to start following a node, ie. listening to new blocks
// and updating the database accordingly.
func (c *Client) Follow(host, target *network.ServerIdentity,
	scID skipchain.SkipBlockID) error {

	req := Follow{
		Target: target,
		ScID:   scID,
	}

	resp := EmptyReply{}

	err := c.SendProtobuf(host, &req, &resp)
	if err != nil {
		return xerrors.Errorf("failed to send follow request: %v", err)
	}

	return nil
}

// Unfollow sends a request to stop following a node.
func (c *Client) Unfollow(host *network.ServerIdentity) error {
	req := Unfollow{}
	resp := EmptyReply{}

	err := c.SendProtobuf(host, &req, &resp)
	if err != nil {
		return xerrors.Errorf("failed to send follow request: %v", err)
	}

	return nil
}

// Query sends a query request. The query must be a read-only SQL query, for
// example:
//   Select * from cothority.block
func (c *Client) Query(host *network.ServerIdentity, query string) ([]byte, error) {
	req := Query{
		Query: query,
	}
	resp := QueryReply{}

	err := c.SendProtobuf(host, &req, &resp)
	if err != nil {
		return nil, xerrors.Errorf("failed to send query request: %v", err)
	}

	return resp.Result, nil
}

// CatchUP sends a request to catch up from a particular block. A catch up will
// update the database from the specified block until the end of the chain.
// Every specifies the interval at which updates are sent.
func (c *Client) CatchUP(ctx context.Context, host, target *network.ServerIdentity,
	scID skipchain.SkipBlockID, fromBlock skipchain.SkipBlockID,
	every int) (<-chan CatchUpResponse, error) {

	req := CatchUpMsg{
		ScID:        scID,
		Target:      target,
		FromBlock:   fromBlock,
		UpdateEvery: every,
	}

	apiEndpoint, err := getWsAddr(req.Target)
	if err != nil {
		return nil, xerrors.Errorf("failed to get ws addr: %v", err)
	}

	apiURL := fmt.Sprintf("%s/%s/%s", apiEndpoint, ServiceName, "CatchUpMsg")

	ws, err := c.OverlayClient.OpenWS(apiURL)
	if err != nil {
		return nil, xerrors.Errorf("failed to open ws: %v", err)
	}

	buf, err := protobuf.Encode(&req)
	if err != nil {
		return nil, xerrors.Errorf("failed to encode streaming request: %v", err)
	}

	err = ws.Write(websocket.BinaryMessage, buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to send streaming request: %v", err)
	}

	outChan := make(chan CatchUpResponse)

	go listenCatchup(ws, outChan)

	return outChan, nil
}

// listenCatchup listens for messages on the ws and writes the responses to the
// outChan. It closes the outChan once a done message is received.
func listenCatchup(ws WsHandler, outChan chan CatchUpResponse) {
	defer func() {
		err := ws.Write(websocket.CloseMessage, nil)
		if err != nil {
			log.Warnf("failed to send close: %v", err)
		}
		ws.Close()
	}()

	for {
		_, buf, err := ws.Read()
		if err != nil {
			outChan <- CatchUpResponse{
				Err: fmt.Sprintf("failed to read response: %v", err),
			}
			return
		}

		resp := CatchUpResponse{}

		err = protobuf.Decode(buf, &resp)
		if err != nil {
			outChan <- CatchUpResponse{
				Err: fmt.Sprintf("failed to decode response: %v", err),
			}
			return
		}

		outChan <- resp
		if resp.Done {
			close(outChan)
			return
		}
	}
}

// onetOverlay provides an overlay implementation based on onet and gorilla.
//
// - implements OverlayClient
type onetOverlay struct {
	*onet.Client
}

// OpenWS implements OverlayClient
func (o onetOverlay) OpenWS(url string) (WsHandler, error) {
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial %s: %v", url, err)
	}

	return gorillaWs{
		client: ws,
	}, nil
}

// gorillaWs is a websocket handler that uses the gorilla package
//
// - implements WsHandler
type gorillaWs struct {
	client *websocket.Conn
}

// CloseWS implements WsHandler
func (g gorillaWs) Close() error {
	return g.client.Close()
}

// SendWSMessage implements WsHandler
func (g gorillaWs) Write(messageType int, data []byte) error {
	return g.client.WriteMessage(messageType, data)
}

// ReadWSMessage implements WsHandler
func (g gorillaWs) Read() (messageType int, p []byte, err error) {
	return g.client.ReadMessage()
}
