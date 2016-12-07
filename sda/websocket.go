package sda

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"time"

	"reflect"
	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/gorilla/websocket"
	"gopkg.in/tylerb/graceful.v1"
)

// WebSocket handles incoming client-requests using the WebService
// protocol for JavaScript-compatibility.
type WebSocket struct {
	ServerIdentity *network.ServerIdentity
	services       map[string]Service
	server         *graceful.Server
	mux            *http.ServeMux
}

const readSize = 1024
const (
	WebSocketErrorPathNotFound     = 4000
	WebSocketErrorProtobufDecode   = 4001
	WebSocketErrorProtobufEncode   = 4002
	WebSocketErrorInvalidErrorCode = 4003
)

type wsHandler struct {
	ws      *WebSocket
	service string
	path    string
}

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
			(ce.ErrorCode() < 4100 || ce.ErrorCode() >= 4200) {
			ce = NewClientErrorCode(WebSocketErrorInvalidErrorCode, "")
			break
		}
	}
	ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(ce.ErrorCode(), ce.ErrorMsg()),
		time.Now().Add(time.Second))
}

// NewWebSocket opens a webservice-listener one port above the given
// ServerIdentity.
func NewWebSocket(si *network.ServerIdentity) *WebSocket {
	w := &WebSocket{
		ServerIdentity: si,
		services:       make(map[string]Service),
	}
	webHost, err := getWebHost(w.ServerIdentity)
	log.ErrFatal(err)
	w.mux = http.NewServeMux()
	w.server = &graceful.Server{
		Timeout: 100 * time.Millisecond,
		Server: &http.Server{
			Addr:    webHost,
			Handler: w.mux,
		},
	}
	log.Lvl1("Starting to listen on", webHost)
	go w.server.ListenAndServe()
	return w
}

// RegisterService saves the service as being able to handle messages.
func (w *WebSocket) RegisterService(service string, s Service) error {
	w.services[service] = s
	return nil
}

// Register a message-handler for a service.
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

func (w *WebSocket) Stop() {
	w.server.Stop(100 * time.Millisecond)
}

// getWebHost returns the host:port+1 of the serverIdentity.
func getWebHost(si *network.ServerIdentity) (string, error) {
	p, err := strconv.Atoi(si.Address.Port())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", si.Address.Host(), p+1), nil
}

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

func NewClientErrorCode(code int, msg string) ClientError {
	return &cerror{code, msg}
}

func (ce *cerror) ErrorCode() int {
	return ce.code
}
func (ce *cerror) ErrorMsg() string {
	return ce.msg
}
func (ce *cerror) Error() string {
	if ce == nil {
		return ""
	}
	if ce.code > 0 {
		return fmt.Sprintf(wsPrefix+"%d: %s", ce.code, ce.msg)
	} else {
		return ce.msg
	}
}

// Client is a struct used to communicate with a remote Service running on a
// sda.Conode
type Client struct {
	service string
}

// NewClient returns a client using the service s. It uses TCP communication by
// default
func NewClient(s string) *Client {
	return &Client{
		service: s,
	}
}

// Send will marshal the message into a ClientRequest message and send it.
func (c *Client) Send(dst *network.ServerIdentity, path string, buf []byte) ([]byte, ClientError) {
	// Open connection to service.
	url, err := getWebHost(dst)
	if err != nil {
		return nil, NewClientError(err)
	}
	log.Lvlf4("Sending %x to %s/%s/%s", buf, url, c.service, path)
	d := &websocket.Dialer{}
	ws, _, err := d.Dial(fmt.Sprintf("ws://%s/%s/%s", url, c.service, path),
		http.Header{})
	if err != nil {
		return nil, NewClientError(err)
	}
	if err = ws.WriteMessage(websocket.BinaryMessage, buf); err != nil {
		return nil, NewClientError(err)
	}
	_, rcv, err := ws.ReadMessage()
	if err != nil {
		return nil, NewClientError(err)
	}
	log.Lvlf4("Received %x", rcv)
	return rcv, nil
}

// SendReflect wraps protobuf.(En|De)code to the Client.Send-function.
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
		err := protobuf.Decode(reply, ret)
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
