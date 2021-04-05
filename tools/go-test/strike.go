package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

type TX struct {
	Txtype      string
	Sender      string
	Receiver    string
	ID          [sha256.Size]byte
	Content     string
	TxSignature string
	Operate     int

	// 当交易类型为relayTX时有用，其余类型为空跳过即可
	// AggSig AggregateSig
	Height int // 记录该条跨片交易被共识的区块高度
}
type Participant struct {
	Address []byte
	Pubkey  []byte
}
type AggregateSig struct {
	Signature []byte // sign

	// 潜在的问题：传递的公钥中是否包含协议信息，毕竟验证方需要将byte构建为PubKey；
	// 目前实现可以约定某种加密协议，也只能是bls。。
	Participants []Participant // 参与聚合签名生成的验证者的公钥，对Participant进行修改。第一个是address的大小
}
type plist []*ecdsa.PrivateKey

func createCount() plist {
	var pl plist
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	pl = append(pl, priv)
	return pl
}

type ecdsaSignature struct {
	X, Y *big.Int
}

func pub2string(pub ecdsa.PublicKey) string {

	coor := ecdsaSignature{X: pub.X, Y: pub.Y}
	b, _ := asn1.Marshal(coor)

	return hex.EncodeToString(b)
}
func createPrivContent(rate int) (string, string) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	pub_s := priv.PublicKey
	tx_content := "_" + pub2string(pub_s) + "_" + strconv.Itoa(rate) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sig := "sig"
	return tx_content, sig
}
func createStrikeTx(rate int, relay bool) []byte {
	content, sig := createPrivContent(rate)
	var txtype string
	if relay {
		txtype = "CrossSendStrikeTx"
		fmt.Println("relay")
	} else {
		txtype = "CrossReceiveStrikeTx"
	}
	tx := &TX{
		Txtype:      txtype,
		Sender:      "0",
		Receiver:    "",
		ID:          sha256.Sum256([]byte(content)),
		Content:     content,
		TxSignature: sig,
		Operate:     0}
	res, _ := json.Marshal(tx)
	return res
}
func connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

type RPCRequest struct {
	JSONRPC  string          `json:"jsonrpc"`
	Sender   string          `json:"Sender"`   //添加发送者
	Receiver string          `json:"Receiver"` //添加接受者
	Flag     int             `json:"Flag"`
	ID       jsonrpcid       `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}

}
type jsonrpcid interface {
	isJSONRPCID()
}
type JSONRPCStringID string

func (JSONRPCStringID) isJSONRPCID() {}
func SendTx(ip string, rate int, relay bool) {
	var c *websocket.Conn
	var ntx []byte
	var err error
	c, _, err = connect(ip)
	if err != nil {
		fmt.Println("链接失败")
	}

	ntx = createStrikeTx(rate, relay)
	paramsJSON, err := json.Marshal(map[string]interface{}{"tx": ntx})
	rawParamsJSON := json.RawMessage(paramsJSON)
	err = c.WriteJSON(RPCRequest{
		JSONRPC: "2.0",
		ID:      JSONRPCStringID("StrikeTest"),
		Method:  "broadcast_tx_async",
		Params:  rawParamsJSON,
	})
	if err != nil {
		fmt.Println("发送失败")
	}
	c.Close()
}
func main() {
	args := os.Args
	ip := args[1]
	relay := args[2]
	rate := args[3]
	var isrelay bool
	if relay == "relay" {
		isrelay = true
	} else {
		isrelay = false
	}
	rateint, _ := strconv.Atoi(rate)
	SendTx(ip, rateint, isrelay)
}
