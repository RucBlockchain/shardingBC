package identypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/libs/log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	SepTxContent = "_"
	ConfigTx     = "configtx"
	DeliverTx    = "DeliverTx"
	RelayTx      = "relaytx"
	AddTx        = "addtx"
)

var (
	logger          = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	identypesLogger = logger.With("module", "identypes")
	ConsensusBegin  = time.Now()
	CurrentHeight   int64
)

/*
Tx的type有五种：
类型是string
普通tx：
	tx
跨片tx：
	relaytx
	addtx
共识tx
	aggregate
	confirmmessage
检查点tx：
	checkpoint
初始化tx:
	init
tendermint 原始tx：
- deliver tx DeliverTx

content内容g格式
{sender}_{receiver}_{amount)

摘要格式：
md5({Content})

签名格式：ANS1格式 转换结构体
struct ecdsaSign { R,S *big.Int }

pubkey格式: ANS1格式 转换结构体
struct ecdsaSign { X,Y *big.Int }
账号/公钥影藏在content内

ps: TxSignature …& 公钥都是将两个大整数转换为byte数组后，按16进制编码
*/

// asn1转换格式
type EcdsaCoordinate struct {
	X, Y *big.Int
}

// TODO
// 重命名结构体，容易与types.Tx混淆
// Txtype变更为使用enum类型
type TX struct {
	Txtype      string
	Sender      string
	Receiver    string
	ID          [sha256.Size]byte
	Content     string
	TxSignature string
	Operate     int // 查部分的定义 0为转出方 1为转入方

	// 当交易类型为relayTX时有用，其余类型为空跳过即可

	//AggSig AggregateSig
	Height int // 记录该条跨片交易被共识的区块高度
}

func NewTX(data []byte) (*TX, error) {
	tx := new(TX)

	tmp := make([]byte, len(data), len(data))
	copy(tmp, data)

	err := json.Unmarshal(tmp, tx)
	if err != nil {
		return nil, err
	}
	return tx, err
}
func TimePhase(phase int, tx_id [sha256.Size]byte, t string) string {
	return fmt.Sprintf("[tx_phase] index:phase%d id:%X time:%s", phase, tx_id, t)
}

// 处理跨片交易的后程，修改交易属性
func (tx *TX) UpdateTx() {
	if tx == nil {
		return
	}
	// 如果交易类型为relaytx 且 发送方是本分片，则该交易为relay_in，修改operate值为1
	//if tx.Txtype=="tx" || tx.Txtype=="init"{//说明是本片交易，那么此时输出第三阶段
	//	logger.Info(TimePhase(3,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第三阶段信息打印
	//}
	if tx.Txtype == "relaytx" && tx.Sender == getShard() {
		//
		//logger.Info(TimePhase(3,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第三阶段信息打印
		tx.Operate = 1
	}

	// 如果交易类型为relayTx 且 接收方是本分片，则修改交易类型为addtx
	if tx.Txtype == "relaytx" && tx.Receiver == getShard() {
		//logger.Info(TimePhase(5,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第五阶段信息打印
		tx.Txtype = "addtx"
	}
}
func PrintLog(ID [sha256.Size]byte) bool {
	OXstring := fmt.Sprintf("%X", ID)
	BigInt, err := new(big.Int).SetString(OXstring, 16)
	if !err {
		fmt.Println("生成大整数错误")
	}
	shd := big.NewInt(int64(100)) //取100模运算
	mod := new(big.Int)
	_, mod = BigInt.DivMod(BigInt, shd, mod)
	if mod.String() == "0" {
		return true
	} else {
		return false
	}
}
func (tx *TX) PrintInfo() {
	if tx == nil {
		return
	}
	// 如果交易类型为relaytx 且 发送方是本分片，则该交易为relay_in，修改operate值为1
	if tx.Txtype == "tx" || tx.Txtype == "init" { //说明是本片交易，那么此时输出第三阶段

		if PrintLog(tx.ID) {
			logger.Info(TimePhase(3, tx.ID, strconv.FormatInt(time.Now().UnixNano(), 10))) //第三阶段信息打印
		}
		// logger.Info(TimePhase(3,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第三阶段信息打印
	}
	if tx.Txtype == "relaytx" && tx.Sender == getShard() {
		//
		if PrintLog(tx.ID) {
			logger.Info(TimePhase(3, tx.ID, strconv.FormatInt(time.Now().UnixNano(), 10))) //第三阶段信息打印
		}
		// logger.Info(TimePhase(3,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第三阶段信息打印
	}

	// 如果交易类型为relayTx 且 接收方是本分片，则修改交易类型为addtx
	if tx.Txtype == "addtx" && tx.Receiver == getShard() {
		if PrintLog(tx.ID) {
			logger.Info(TimePhase(5, tx.ID, strconv.FormatInt(time.Now().UnixNano(), 10))) //第三阶段信息打印
		}
		// logger.Info(TimePhase(5,tx.ID,strconv.FormatInt(time.Now().UnixNano(), 10)))//第五阶段信息打印
	}
}
func (tx *TX) Data() []byte {
	if data, err := json.Marshal(tx); err == nil {
		return data
	} else {
		return nil
	}

}

func (tx *TX) VerifySig() bool {
	// 只有Tx & relayTx才需要验证交易签名
	if tx.Txtype != "tx" && tx.Txtype != "relaytx" {
		return true
	}

	// 从content中提取出公钥信息
	pub, err := Content2PubKey(tx.Content)

	if err != nil {
		identypesLogger.Error("verify failed because of pubkey err, more info: ", err)
		return false
	}

	// 将signature转换为两个大整数
	coor, err := sig2bigInt(tx.TxSignature)
	if err != nil {
		identypesLogger.Error("verify failed because of sig err, more info: ", err)
		return false
	}

	return ecdsa.Verify(pub, tx.digest(), coor.X, coor.Y)
}

func (tx *TX) Digest() []byte {
	return tx.digest()
}

// 摘要格式： {Content}
// 如果有多个content则合并为一个string，中间没有sep - 潜在问题，没有明确使用string[]的原因
// 然后进行md5加密
func (tx *TX) digest() []byte {
	origin := []byte(tx.Content)

	// 生成md5 hash值
	digest_md5 := md5.New()
	digest_md5.Write(origin)

	return digest_md5.Sum(nil)
}

func GetContent(tx []byte) []byte {
	var res []byte
	if tmptx, err := NewTX(tx); err != nil {
		res = tx
	} else {
		res = []byte(tmptx.Content)
	}
	return res
}

func Content2PubKey(tx_content string) (*ecdsa.PublicKey, error) {
	// 从content从提取出转出方公钥信息
	strtmp := strings.Split(tx_content, SepTxContent)
	return string2Pubkey(strtmp[0])
}

func string2Pubkey(pub string) (*ecdsa.PublicKey, error) {
	var pubcoor EcdsaCoordinate
	var err error

	// hex string 2 byte
	b, err := hex.DecodeString(pub)
	if err != nil {
		return nil, err
	}

	_, err = asn1.Unmarshal(b, &pubcoor)
	if err != nil {
		return nil, err
	}

	return &ecdsa.PublicKey{Y: pubcoor.Y, X: pubcoor.X, Curve: elliptic.P256()}, nil
}

// 签名转换为坐标点，R为X， S为Y
func sig2bigInt(sig_str string) (*EcdsaCoordinate, error) {
	var coor EcdsaCoordinate
	var err error

	sig, err := hex.DecodeString(sig_str)
	if err != nil {
		return nil, err
	}
	_, err = asn1.Unmarshal(sig, &coor)
	if err != nil {
		return nil, err
	}

	return &EcdsaCoordinate{X: coor.X, Y: coor.Y}, nil
}
