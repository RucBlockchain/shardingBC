package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/btcsuite/websocket"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tp "github.com/tendermint/tendermint/identypes"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

var logger = log.NewNopLogger()
var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(ed25519.PubKeyEd25519{},
		"tendermint/PubKeyEd25519", nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(ed25519.PrivKeyEd25519{},
		"tendermint/PrivKeyEd25519", nil)
}

func main() {
	var keypath string
	var Power int

	flagSet := flag.NewFlagSet("sendtx", flag.ExitOnError)

	//初始化数值
	flagSet.StringVar(&keypath, "k", "key.json", "")
	flagSet.IntVar(&Power, "p", 10, "")
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

	fmt.Println(keypath, Power)
	// 根据path加载公钥
	fpv := loadFilePV(keypath)

	// 建立client连接准备发送tx
	conn, _, err := connect(endpoint)
	if err != nil {
		fmt.Println(err)
		return
	}

	tx := generateNormalTx(generateChangeTx(fpv.PubKey, Power))
	paramsJSON, err := json.Marshal(map[string]interface{}{"tx": tx})
	if err != nil {
		fmt.Printf("failed to encode params: %v\n", err)
		os.Exit(1)
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

	defer conn.Close()
}

// 返回一个与指定tendermint节点的websocket连接
func connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

// 从文件中加载validator密钥数据
func loadFilePV(keyFilePath string) *privval.FilePVKey {
	keyJSONBytes, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pvKey := &privval.FilePVKey{}

	err = cdc.UnmarshalJSON(keyJSONBytes, &pvKey)
	if err != nil {
		cmn.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}

	// overwrite pubkey and address for convenience
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = pvKey.PubKey.Address()

	return pvKey
}

// 生成tendermint格式的update tx
// TODO 适配为shardingbc的tx
func generateChangeTx(pubkey crypto.PubKey, power int) string {
	tmpubkey := types.TM2PB.PubKey(pubkey)
	return fmt.Sprintf("val:%X/%d", tmpubkey.Data, power)
	//return []byte(fmt.Sprintf("val:%X/%d", tmpubkey.Data, power))
}

// 适配为shardingbc的tx
func generateNormalTx(content string) []byte {
	tx := &tp.TX{
		Txtype:      "relaytx",
		Sender:      "topo",
		Receiver:    "0",
		ID:          sha256.Sum256([]byte(content)),
		Content:     content,
		TxSignature: nil,
		Operate:     0}

	res, _ := json.Marshal(tx)
	return res
}
