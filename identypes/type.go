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
	"math/big"
	"strings"
)

const (
	SepTxContent = "_"
)

/*
Tx的type有五种种：
类型是string
普通tx：
	tx
跨片tx：
	relaytx
	addtx
检查点tx：
	checkpoint
初始化tx:
	init

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

type TX struct {
	Txtype      string
	Sender      string
	Receiver    string
	ID          [sha256.Size]byte
	Content     string
	TxSignature string
	Operate     int
}

//这格式只在relay tx相关的内容中使用
func NewTX(data []byte) (*TX, error) {
	tx := new(TX)
	err := json.Unmarshal(data, tx)
	if err != nil {
		return nil, err
	}
	return tx, err
}

func (tx *TX) VerifySig() bool {
	// 只有addTx & relayTx才需要验证交易签名
	if tx.Txtype != "addTx" && tx.Txtype != "relayTx" {
		fmt.Println("该交易类型检查， txtype: ", tx.Txtype)
		return true
	}

	// 从content中提取出公钥信息
	pub, err := Content2PubKey(tx.Content)

	if err != nil {
		fmt.Println("verify failed because of pubkey err, more info: ", err)
		return false
	}

	// 将signature转换为两个大整数
	coor, err := sig2bigInt(tx.TxSignature)
	if err != nil {
		fmt.Println("verify failed because of sig err, more info: ", err)
		return false
	}

	return ecdsa.Verify(pub, tx.digest(), coor.X, coor.Y)
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
