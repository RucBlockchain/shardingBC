package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/tendermint/go-amino"
	types2 "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls"
	tp "github.com/tendermint/tendermint/identypes"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
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
	fmt.Println(fpv)
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
	fmt.Println("===================")
	fmt.Println([]byte(pubkey.(bls.PubKeyBLS)))
	tmpubkey := types.TM2PB.PubKey(pubkey)
	fmt.Println("===================")
	fmt.Println(fmt.Sprintf("val:%X/%d", tmpubkey.Data, power))
	return fmt.Sprintf("val:%X/%d", tmpubkey.Data, power)
	//return []byte(fmt.Sprintf("val:%X/%d", tmpubkey.Data, power))
}

// 适配为shardingbc的tx
func generateNormalTx(content string) []byte {
	tx := &tp.TX{
		Txtype:      "DeliverTx",
		Sender:      "topo",
		Receiver:    "0",
		ID:          sha256.Sum256([]byte(content)),
		Content:     content,
		TxSignature: "",
		Operate:     0,
	}
	fmt.Println(tx)
	res, _ := json.Marshal(tx)
	return res
}

func mockexec(keypath string) {
	fpv := loadFilePV(keypath)
	fmt.Println("origin pubkey: ", fpv.PubKey.(bls.PubKeyBLS))

	tx := generateChangeTx(fpv.PubKey, 10)[4:]

	//get the pubkey and power
	pubKeyAndPower := strings.Split(string(tx), "/")
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]
	// decode the pubkey
	pubkey, err := hex.DecodeString(pubkeyS)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(pubkey)

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		fmt.Println(err)
	}

	vu := types2.BlsValidatorUpdate(pubkey, power)

	v1, err := types.PB2TM.ValidatorUpdates([]types2.ValidatorUpdate{vu})
	fmt.Println("recover: ", []byte(v1[0].PubKey.(bls.PubKeyBLS)))
}
