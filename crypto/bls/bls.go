// bls包是对prysmaticlabs/prysm/shared/bls的封装
package bls

import (
	"errors"
	"fmt"
	"io"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"

	"github.com/herumi/bls-eth-go-binary/bls"
)

//-------------------------------------

var (
	ErrBLSSignature       = errors.New("Invalid BLS signature")
	ErrAggregateSignature = errors.New("Error in BLS aggregate process")
)

var _ crypto.PrivKey = PrivKeyBLS{}

const (
	PrivKeyBLSName = "tendermint/PrivKeyBLS"
	PubKeyBLSName  = "tendermint/PubKeyBLS"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeyBLS{},
		PubKeyBLSName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeyBLS{},
		PrivKeyBLSName, nil)

	bls.Init(bls.BLS12_381)
}

// PrivKeyBLS implements crypto.PrivKey.
type PrivKeyBLS struct {
	Priv *bls.SecretKey
}

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

func (privKey PrivKeyBLS) Sign(msg []byte) ([]byte, error) {
	if privKey.Priv == nil {
		return nil, errors.New("private key is empty")
	}
	if len(msg) == 0 {
		return nil, errors.New("msg is empty")
	}

	sig := privKey.Priv.SignByte(msg)
	if sig == nil {
		return nil, errors.New("bls sign msg failed.")
	}

	return sig.Serialize(), nil
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyBLS) PubKey() crypto.PubKey {
	return PubKeyBLS{privKey.Priv.GetPublicKey()}
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
// TODO
func (privKey PrivKeyBLS) Equals(other crypto.PrivKey) bool {
	return true
}

// GenPrivKey generates a new BLS private key.
// It uses OS randomness in conjunction with the current global random seed
func GenPrivKey() PrivKeyBLS {
	return genPrivKey(crypto.CReader())
}

// genPrivKey generates a new bls private key using the provided reader.
func genPrivKey(rander io.Reader) PrivKeyBLS {
	bls.SetRandFunc(rander)

	priv := &bls.SecretKey{} // 此时为空
	priv.SetByCSPRNG()

	return PrivKeyBLS{Priv: priv}
}

//-------------------------------------

var _ crypto.PubKey = PubKeyBLS{}

type PubKeyBLS struct {
	Pubkey *bls.PublicKey
}

func (pubKey PubKeyBLS) Address() crypto.Address {
	b := pubKey.Pubkey.Serialize()
	return crypto.Address(tmhash.SumTruncated(b))
}

// 返回公钥完整的byte
func (pubKey PubKeyBLS) Bytes() []byte {
	b := pubKey.Pubkey.Serialize()
	return cdc.MustMarshalBinaryBare(b)
}

func (pubKey PubKeyBLS) VerifyBytes(msg []byte, sig []byte) bool {
	if len(sig) == 0 {
		return false
	}

	// byte还原为bls的Sign结构
	blsSig := &bls.Sign{}
	err := blsSig.Deserialize(sig)
	if err != nil {
		return false
	}
	return blsSig.VerifyByte(pubKey.Pubkey, msg)
}

func (pubKey PubKeyBLS) String() string {
	return fmt.Sprintf("PubKeyBLS{%X}", pubKey.Pubkey.Serialize())
}

// nolint: golint
// TODO
func (pubKey PubKeyBLS) Equals(other crypto.PubKey) bool {
	return true
}

// 不需要聚合公钥的验证方法
func AggregateVerifyNP(asign []byte, msg []byte, pubkey []byte) bool {
	if len(asign) == 0 {
		return false
	}

	// 还原公钥
	pub := GetPubkeyFromByte(pubkey)
	if pub == nil {
		return false
	}

	return pub.VerifyBytes(msg, asign)
}

// 输入聚合签名和已经聚合过的公钥，返回验证结果
func AggregateVerify(asign []byte, msg []byte, pubkeys [][]byte) bool {
	if len(asign) == 0 {
		return false
	}
	if len(pubkeys) == 0 {
		return false
	}

	// 还原signature
	sig := bls.Sign{}
	if err := sig.Deserialize(asign); err != nil {
		return false
	}

	// 还原公钥
	blsPubkeys := make([]bls.PublicKey, 0, len(pubkeys))
	for _, p := range pubkeys {
		if tmppub := getPubkeyFromByte(p); tmppub != nil {
			blsPubkeys = append(blsPubkeys, *tmppub)
		} else {
			return false
		}
	}
	pubkey, _ := aggregatePubKey(pubkeys)

	return sig.VerifyByte(pubkey, msg)
}

func AggregateSignature(sig []byte, aggSigByte []byte) ([]byte, error) {
	if aggSigByte == nil {
		return sig, nil
	}

	aggSig := &bls.Sign{}

	// 就两个签名合并：原始的聚合签名，准备合并的签名

	if err := aggSig.Deserialize(aggSigByte); err != nil {
		return nil, ErrBLSSignature
	}

	tmpsig := &bls.Sign{}
	if err := tmpsig.Deserialize(sig); err == nil {
		aggSig.Add(tmpsig)
	} else {
		return nil, ErrBLSSignature
	}
	return aggSig.Serialize(), nil
}

// 将相同消息的BLS签名聚合为一个签名，sigs含有2个以上的签名
func AggregateAllSignature(sigs [][]byte) ([]byte, error) {
	aggSig := &bls.Sign{}

	if len(sigs) == 0 {
		return nil, errors.New("signature is empty")
	} else if len(sigs) == 1 {
		return sigs[0], nil
	}

	for i := 0; i < len(sigs); i++ {
		tmpSig := &bls.Sign{}
		err := tmpSig.Deserialize(sigs[i])
		if err != nil {
			return nil, err
		}
		aggSig.Add(tmpSig)
	}

	return aggSig.Serialize(), nil
}

func AggregatePubkey(pubkeys [][]byte) (*PubKeyBLS, error) {
	p, err := aggregatePubKey(pubkeys)
	if err != nil {
		return nil, err
	}
	return &PubKeyBLS{Pubkey: p}, nil
}

// 将多个BLS的公钥聚合成一把公钥
func aggregatePubKey(pubkeys [][]byte) (*bls.PublicKey, error) {
	if len(pubkeys) == 0 {
		return nil, errors.New("public keys is empty")
	}

	aggPub := &bls.PublicKey{}
	for i := 0; i < len(pubkeys); i++ {
		tmppub := getPubkeyFromByte(pubkeys[i])
		if tmppub == nil {
			return nil, errors.New("recover BLS public key failed.")
		}
		aggPub.Add(tmppub)
	}
	return aggPub, nil
}

func GetPubkeyFromByte(pub []byte) *PubKeyBLS {
	blspub := getPubkeyFromByte(pub)
	if blspub == nil {
		return nil
	}

	return &PubKeyBLS{Pubkey: blspub}
}

func getPubkeyFromByte(pub []byte) *bls.PublicKey {
	if len(pub) == 0 {
		return nil
	}

	tmpbyte := make([]byte, len(pub)-1, len(pub)-1)
	// 首先反序列
	if err := cdc.UnmarshalBinaryBare(pub, &tmpbyte); err != nil {
		return nil
	}

	blspub := &bls.PublicKey{}

	if err := blspub.Deserialize(tmpbyte); err != nil {
		return nil
	}

	return blspub
}
