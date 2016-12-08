package sda

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"time"

	"reflect"
	"strings"

	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/gorilla/websocket"
	"gopkg.in/tylerb/graceful.v1"
)

// WebSocket handles incoming client-requests using the WebSocket
// protocol for JavaScript-compatibility. When making a new WebSocket,
// it will listen one port above the ServerIdentity-port-#.
type WebSocket struct {
	services map[string]Service
	server   *graceful.Server
	mux      *http.ServeMux
}

const (
	// WebSocketErrorPathNotFound indicates the path has not been registered
	WebSocketErrorPathNotFound = 4000
	// WebSocketErrorProtobufDecode indicates an error in decoding the protobuf-packet
	WebSocketErrorProtobufDecode = 4001
	// WebSocketErrorProtobufEncode indicates an error in encoding the return packet
	WebSocketErrorProtobufEncode = 4002
	// WebSocketErrorInvalidErrorCode indicates the service returned
	// an invalid error-code
	WebSocketErrorInvalidErrorCode = 4003
)

// NewWebSocket opens a webservice-listener one port above the given
// ServerIdentity.
func NewWebSocket(si *network.ServerIdentity) *WebSocket {
	w := &WebSocket{
		services: make(map[string]Service),
	}
	webHost, err := getWebHost(si)
	log.ErrFatal(err)
	w.mux = http.NewServeMux()
	w.server = &graceful.Server{
		Timeout: 100 * time.Millisecond,
		Server: &http.Server{
			Addr:    webHost,
			Handler: w.mux,
		},
	}
	return w
}

// Start listening on the port.
func (w *WebSocket) Start() {
	log.Lvl1("Starting to listen on", w.server.Server.Addr)
	w.server.ListenAndServe()
}

// RegisterService saves the service as being able to handle messages.
func (w *WebSocket) RegisterService(service string, s Service) error {
	w.services[service] = s
	return nil
}

// RegisterMessageHandler for a service. Requests from a client to
// "ws://service/path" will be forwarded to the corresponding
// ProcessClientRequest of the 'service'.
func (w *WebSocket) RegisterMessageHandler(service, path string) error {
	log.Lvlf3("Registering websocket for ws://hostname/%s/%s", service, path)
	h := &wsHandler{
		ws:      w,
		service: service,
		path:    path,
	}
	w.mux.Handle(fmt.Sprintf("/%s/%s", service, path), h)
	return nil
}

// Stop the websocket and free the port.
func (w *WebSocket) Stop() {
	w.server.Stop(100 * time.Millisecond)
}

// Client is a struct used to communicate with a remote Service running on a
// sda.Conode
type Client struct {
	service string
	si      *network.ServerIdentity
	conn    *websocket.Conn
	// whether to keep the connection
	keep bool
	sync.Mutex
}

// NewClient returns a client using the service s. On the first Send, the
// connection will be started, until Close is called.
func NewClient(s string) *Client {
	return &Client{
		service: s,
	}
}

// NewClientKeep returns a Client that doesn't close the connection between
// two messages if it's the same conode.
func NewClientKeep(s string) *Client {
	return &Client{
		service: s,
		keep:    true,
	}
}

// Send will marshal the message into a ClientRequest message and send it.
func (c *Client) Send(dst *network.ServerIdentity, path string, buf []byte) ([]byte, ClientError) {
	c.Lock()
	defer c.Unlock()
	if c.keep &&
		(c.si != nil && !c.si.ID.Equal(dst.ID)) {
		// We already have a connection, but to another ServerIdentity
		c.Close()
	}
	if c.si == nil {
		// Open connection to service.
		url, err := getWebHost(dst)
		if err != nil {
			return nil, NewClientError(err)
		}
		log.Lvlf4("Sending %x to %s/%s/%s", buf, url, c.service, path)
		d := &websocket.Dialer{}
		c.conn, _, err = d.Dial(fmt.Sprintf("ws://%s/%s/%s", url, c.service, path),
			http.Header{})
		if err != nil {
			return nil, NewClientError(err)
		}
	}
	defer func() {
		if !c.keep {
			c.Close()
		}
	}()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, buf); err != nil {
		return nil, NewClientError(err)
	}
	_, rcv, err := c.conn.ReadMessage()
	if err != nil {
		return nil, NewClientError(err)
	}
	log.Lvlf4("Received %x", rcv)
	return rcv, nil
}

// SendProtobuf wraps protobuf.(En|De)code over the Client.Send-function. It
// takes the destination, a pointer to a msg-structure that will be
// protobuf-encoded and sent over the websocket. If ret is non-nil, it
// has to be a pointer to the struct that is sent back to the
// client. If there is no error, the ret-structure is filled with the
// data from the service. ClientError has a code and a msg in case
// something went wrong.
func (c *Client) SendProtobuf(dst *network.ServerIdentity, msg interface{}, ret interface{}) ClientError {
	buf, err := protobuf.Encode(msg)
	if err != nil {
		return NewClientError(err)
	}
	path := strings.Split(reflect.TypeOf(msg).String(), ".")[1]
	reply, cerr := c.Send(dst, path, buf)
	if cerr != nil {
		return NewClientError(cerr)
	}
	if ret != nil {
		err := protobuf.DecodeWithConstructors(reply, ret,
			network.DefaultConstructors(network.Suite))
		return NewClientError(err)
	}
	return nil
}

// SendToAll sends a message to all ServerIdentities of the Roster and returns
// all errors encountered concatenated together as a string.
func (c *Client) SendToAll(dst *Roster, path string, buf []byte) ([][]byte, ClientError) {
	msgs := make([][]byte, len(dst.List))
	var errstrs []string
	for i, e := range dst.List {
		var err ClientError
		msgs[i], err = c.Send(e, path, buf)
		if err != nil {
			errstrs = append(errstrs, fmt.Sprint(e.String(), err.Error()))
		}
	}
	var err error
	if len(errstrs) > 0 {
		err = errors.New(strings.Join(errstrs, "\n"))
	}
	return msgs, NewClientError(err)
}

// Close sends a close-command to the connection.
func (c *Client) Close() error {
	if c.si != nil && c.conn != nil {
		c.si = nil
		return c.conn.Close()
	}
	return nil
}

// Pass the request to the websocket.
type wsHandler struct {
	ws      *WebSocket
	service string
	path    string
}

// Wrapper-function so that http.Requests get 'upgraded' to websockets
// and handled correctly.
func (t wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: true,
	}
	ws, err := u.Upgrade(w, r, http.Header{"Set-Cookie": {"sessionID=1234"}})
	if err != nil {
		log.Error(err)
		return
	}
	defer ws.Close()
	var ce ClientError
	// Loop as long as we don't return an error.
	for ce == nil {
		mt, buf, err := ws.ReadMessage()
		if err != nil {
			log.Error(err)
			return
		}
		s := t.ws.services[t.service]
		var reply []byte
		reply, ce = s.ProcessClientRequest(t.path, buf)
		if ce == nil {
			err := ws.WriteMessage(mt, reply)
			if err != nil {
				log.Error(err)
				return
			}
			ce = NewClientErrorCode(0, "")
		}
		if ce.ErrorCode() > 0 &&
			(ce.ErrorCode() < 4100 || ce.ErrorCode() >= 5000) {
			ce = NewClientErrorCode(WebSocketErrorInvalidErrorCode, "")
			break
		}
	}
	ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(ce.ErrorCode(), ce.ErrorMsg()),
		time.Now().Add(time.Second))
}

// ClientError allows for returning error-codes and error-messages.
type ClientError interface {
	Error() string
	ErrorCode() int
	ErrorMsg() string
}

type cerror struct {
	code int
	msg  string
}

const wsPrefix = "websocket: close "

// NewClientError takes a standard error and
// - returns a ClientError if it's a standard error
// or
// - parses the wsPrefix to correctly get the id and msg of the error
func NewClientError(e error) ClientError {
	if e == nil {
		return nil
	}
	str := e.Error()
	if strings.HasPrefix(str, wsPrefix) {
		str = str[len(wsPrefix):]
		errMsg := strings.Split(str, ":")
		if len(errMsg) > 1 && len(errMsg[1]) > 0 {
			errMsg[1] = errMsg[1][1:]
		} else {
			errMsg = append(errMsg, "")
		}
		errCode, _ := strconv.Atoi(errMsg[0])
		return &cerror{errCode, errMsg[1]}
	}
	return &cerror{0, e.Error()}
}

// NewClientErrorCode takes an errorCode and an errorMsg and returns the
// corresponding ClientError.
func NewClientErrorCode(code int, msg string) ClientError {
	return &cerror{code, msg}
}

// ErrorCode returns the errorCode.
func (ce *cerror) ErrorCode() int {
	return ce.code
}

// ErrorMsg returns the errorMsg.
func (ce *cerror) ErrorMsg() string {
	return ce.msg
}

// Error makes the cerror-structure confirm to the error-interface.
func (ce *cerror) Error() string {
	if ce == nil {
		return ""
	}
	if ce.code > 0 {
		return fmt.Sprintf(wsPrefix+"%d: %s", ce.code, ce.msg)
	}
	return ce.msg
}

// getWebHost returns the host:port+1 of the serverIdentity.
func getWebHost(si *network.ServerIdentity) (string, error) {
	p, err := strconv.Atoi(si.Address.Port())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", si.Address.Host(), p+1), nil
}
