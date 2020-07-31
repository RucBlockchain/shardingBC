package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/tendermint/go-amino"
	tp "github.com/tendermint/tendermint/identypes"
	types "github.com/tendermint/tendermint/rpc/lib/types"
	mytype "github.com/tendermint/tendermint/types"
)

const (
	protoHTTP  = "http"
	protoHTTPS = "https"
	protoWSS   = "wss"
	protoWS    = "ws"
	protoTCP   = "tcp"
)

type RPCFunc struct {
	f        reflect.Value  // underlying rpc function
	args     []reflect.Type // type of each function arg
	returns  []reflect.Type // type of each return arg
	argNames []string       // name of each argument
	ws       bool           // websocket only
}
type JSONRPCClient struct {
	address string
	client  *http.Client
	cdc     *amino.Codec
}
type HTTP struct {
	remote string
	rpc    *JSONRPCClient
}
type ResultBroadcastTx struct {
	Code uint32 `json:"code"`
	Data []byte `json:"data"`
	Log  string `json:"log"`
	Hash []byte `json:"hash"`
}
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      jsonrpcid       `json:"id"`
	Sender  string          `json:"sender"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
}
type Tx []byte
type jsonrpcid interface {
	isJSONRPCID()
}
type JSONRPCStringID string

func (JSONRPCStringID) isJSONRPCID() {}
func Send(funcMap map[string]*RPCFunc, cdc *amino.Codec) {

}
func RegisterAmino(cdc *amino.Codec) {
	mytype.RegisterEventDatas(cdc)
	mytype.RegisterBlockAmino(cdc)
}
func (c *JSONRPCClient) Codec() *amino.Codec {
	return c.cdc
}
func (c *JSONRPCClient) SetCodec(cdc *amino.Codec) {
	c.cdc = cdc
}
func makeHTTPClient(remoteAddr string) (string, *http.Client) {
	protocol, address, dialer := makeHTTPDialer(remoteAddr)
	return protocol + "://" + address, &http.Client{
		Transport: &http.Transport{
			// Set to true to prevent GZIP-bomb DoS attacks
			DisableCompression: false,
			Dial:               dialer,
		},
	}
}
func makeHTTPDialer(remoteAddr string) (string, string, func(string, string) (net.Conn, error)) {
	// protocol to use for http operations, to support both http and https
	clientProtocol := protoHTTP

	parts := strings.SplitN(remoteAddr, "://", 2)
	var protocol, address string
	if len(parts) == 1 {
		// default to tcp if nothing specified
		protocol, address = protoTCP, remoteAddr
	} else if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	} else {
		// return a invalid message
		msg := fmt.Sprintf("Invalid addr: %s", remoteAddr)
		return clientProtocol, msg, func(_ string, _ string) (net.Conn, error) {
			return nil, errors.New(msg)
		}
	}

	// accept http as an alias for tcp and set the client protocol
	switch protocol {
	case protoHTTP, protoHTTPS:
		clientProtocol = protocol
		protocol = protoTCP
	case protoWS, protoWSS:
		clientProtocol = protocol
	}

	// replace / with . for http requests (kvstore domain)
	trimmedAddress := strings.Replace(address, "/", ".", -1)
	return clientProtocol, trimmedAddress, func(proto, addr string) (net.Conn, error) {
		return net.Dial(protocol, address)
	}
}

func NewJSONRPCClient(remote string) *JSONRPCClient {
	address, client := makeHTTPClient(remote)
	return &JSONRPCClient{
		address: address,
		client:  client,
		cdc:     amino.NewCodec(),
	}
}
func NewHTTP(remote, wsEndpoint string) *HTTP {
	rc := NewJSONRPCClient(remote)
	cdc := rc.Codec()
	RegisterAmino(cdc)
	rc.SetCodec(cdc)

	return &HTTP{
		rpc:    rc,
		remote: remote,
	}
}
func (c *JSONRPCClient) Call(method string, params map[string]interface{}, result interface{}) (interface{}, error) {
	request, err := types.MapToRequest(c.cdc, types.JSONRPCStringID("jsonrpc-client"), method, params)
	if err != nil {
		return nil, err
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	//fmt.Println("大小",len(requestBytes))
	// log.Info(string(requestBytes))
	requestBuf := bytes.NewBuffer(requestBytes)
	// log.Info(Fmt("RPC request to %v (%v): %v", c.remote, method, string(requestBytes)))
	httpResponse, err := c.client.Post(c.address, "text/json", requestBuf)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close() // nolint: errcheck

	responseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	// 	log.Info(Fmt("RPC response: %v", string(responseBytes)))
	return unmarshalResponseBytes(c.cdc, responseBytes, result)
}
func unmarshalResponseBytes(cdc *amino.Codec, responseBytes []byte, result interface{}) (interface{}, error) {
	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	// log.Notice("response", "response", string(responseBytes))
	var err error
	response := &types.RPCResponse{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil {
		return nil, errors.Errorf("Error unmarshalling rpc response: %v", err)
	}
	if response.Error != nil {
		return nil, errors.Errorf("Response error: %v", response.Error)
	}
	// Unmarshal the RawMessage into the result.
	err = cdc.UnmarshalJSON(response.Result, result)
	if err != nil {
		return nil, errors.Errorf("Error unmarshalling rpc response result: %v", err)
	}
	return result, nil
}

//对所有的交易包进行发送传入
func (c *HTTP) broadcastCrossMessages(route string, cms []*tp.CrossMessages) {

	for i := 0; i < len(cms); i++ {
		data, _ := json.Marshal(cms[i])
		result := new(ResultBroadcastTx)
		go c.rpc.Call(route, map[string]interface{}{"tx": data}, result)
	}
}

//传入tx数组进行broadcast
func (c *HTTP) BroadcastCrossMessageAsync(cms []*tp.CrossMessages) {
	go c.broadcastCrossMessages("broadcast_tx_async", cms)
}
func (c *HTTP) BroadcastTxAsync(txs []tp.TX) {
	go c.broadcastTX("broadcast_tx_async", txs)
}
func (c *HTTP) broadcastTX(route string, cms []tp.TX) {

	for i := 0; i < len(cms); i++ {
		data, _ := json.Marshal(cms[i])

		result := new(ResultBroadcastTx)
		c.rpc.Call(route, map[string]interface{}{"tx": data}, result)

	}
}
// func (c *HTTP) Status() (*ctypes.ResultStatus, error) {
// 	result := new(ctypes.ResultStatus)
// 	_, err := c.rpc.Call("status", map[string]interface{}{}, result)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "Status")
// 	}
// 	return result, nil
// }
