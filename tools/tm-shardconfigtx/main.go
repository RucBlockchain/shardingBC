package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls"
	tp "github.com/tendermint/tendermint/identypes"
	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

var logger = log.NewNopLogger()
var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(bls.PubKeyBLS{},
		"tendermint/PubKeyBLS", nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(bls.PrivKeyBLS{},
		"tendermint/PrivKeyBLS", nil)
}

func main() {
	var addListstr, removeListstr string
	randSource := rand.NewSource(time.Now().UnixNano())
	newRand := rand.New(randSource)

	flagSet := flag.NewFlagSet("sendtx", flag.ExitOnError)

	//初始化数值
	flagSet.StringVar(&addListstr, "a", "", "addlist")
	flagSet.StringVar(&removeListstr, "r", "", "removelist")
	flagSet.Usage = func() {
		fmt.Println(`no usage!`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}
	flagSet.Parse(os.Args[1:])

	if flagSet.NArg() == 0 {
		flagSet.Usage()
		os.Exit(1)
	}

	var (
		endpoint = flagSet.Arg(0)
	)

	sendBuf := make([][]byte, 0)
	// 建立client连接准备发送tx
	conn, _, err := connect(endpoint)
	defer conn.Close()

	if err != nil {
		fmt.Println(err)
		return
	}

	addlist := strings.Split(addListstr, ",")
	for _, shard := range addlist {
		if shard == "" {
			continue
		}
		content := fmt.Sprintf(
			"%v_%v_%v_%v",
			shard,
			0.1,
			1,
			newRand.Int63(),
		)
		tx := generateNormalTx("0", content)
		sendBuf = append(sendBuf, tx)
	}
	removelist := strings.Split(removeListstr, ",")
	for _, shard := range removelist {
		if shard == "" {
			continue
		}
		content := fmt.Sprintf(
			"%v_%v_%v_%v",
			shard,
			0.1,
			0,
			newRand.Int63(),
		)
		tx := generateNormalTx("0", content)
		sendBuf = append(sendBuf, tx)
	}

	for _, tx := range sendBuf {
		fmt.Println(string(tx))
		paramsJSON, err := json.Marshal(map[string]interface{}{"tx": tx})
		if err != nil {
			fmt.Printf("failed to encode params: %v\n", err)
			continue
		}
		rawParamsJSON := json.RawMessage(paramsJSON) //把交易转换成
		// 通过rpc发送交易
		conn.WriteJSON(rpctypes.RPCRequest{
			JSONRPC: "2.0",
			ID:      rpctypes.JSONRPCStringID("sendtx"),
			Method:  "broadcast_tx_async",
			Params:  rawParamsJSON,
		})
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		fmt.Println(err)
	}
}

// 返回一个与指定tendermint节点的websocket连接
func connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

// 适配为shardingbc的tx
func generateNormalTx(recv, content string) []byte {
	tx := &tp.TX{
		Txtype:      tp.ShardConfigTx,
		Sender:      "",
		Receiver:    recv,
		ID:          sha256.Sum256([]byte(content)),
		Content:     content,
		TxSignature: "",
		Operate:     0,
	}
	res, _ := json.Marshal(tx)
	return res
}
