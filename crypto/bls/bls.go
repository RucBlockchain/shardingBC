// bls包是对prysmaticlabs/prysm/shared/bls的封装
package bls

import (
	"errors"
	"fmt"

	ethbls "github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
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
}

// PrivKeyBLS implements crypto.PrivKey.
type PrivKeyBLS struct {
	Priv *ethbls.SecretKey
}

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

// 如果产生的签名用于聚合签名，那么msg的大小必须限制在32位以内
func (privKey PrivKeyBLS) Sign(msg []byte) ([]byte, error) {
	if privKey.Priv == nil {
		return nil, errors.New("private key is empty")
	}
	if len(msg) == 0 {
		return nil, errors.New("msg is empty")
	}
	sig := privKey.Priv.Sign(msg, 0)
	return sig.Marshal(), nil
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyBLS) PubKey() crypto.PubKey {
	return PubKeyBLS{*privKey.Priv.PublicKey()}
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
	return genPrivKey()
}

// genPrivKey generates a new ed25519 private key using the provided reader.
func genPrivKey() PrivKeyBLS {
	priv := ethbls.RandKey()
	return PrivKeyBLS{Priv: priv}
}

//-------------------------------------

var _ crypto.PubKey = PubKeyBLS{}

type PubKeyBLS struct {
	Pubkey ethbls.PublicKey
}

// 这里的address和下面的Bytes分不清，可能存在隐患
func (pubKey PubKeyBLS) Address() crypto.Address {
	b := pubKey.Pubkey.Marshal()
	return crypto.Address(tmhash.SumTruncated(b))
}

// 必须返回48位的byte
func (pubKey PubKeyBLS) Bytes() []byte {
	b := pubKey.Pubkey.Marshal()
	return cdc.MustMarshalBinaryBare(b)
}

func (pubKey PubKeyBLS) VerifyBytes(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	signature_bls, err := ethbls.SignatureFromBytes(sig)
	if err != nil {
		return false
	}
	return signature_bls.Verify(msg, &pubKey.Pubkey, 0)
}

func (pubKey PubKeyBLS) String() string {
	return fmt.Sprintf("PubKeyBLS{%X}", pubKey.Pubkey.Marshal())
}

// nolint: golint
// TODO
func (pubKey PubKeyBLS) Equals(other crypto.PubKey) bool {
	return true
}

// 输入聚合签名和已经聚合过的公钥，返回验证结果
func AggragateVerify(asign []byte, msg [32]byte, pubkeys [][]byte) bool {
	blspubkeys := make([]*ethbls.PublicKey, 0, 100)
	for _, p := range pubkeys {
		tmpbyte := make([]byte, len(p)-1, len(p)-1)
		if err := cdc.UnmarshalBinaryBare(p, &tmpbyte); err != nil {
			return false
		}
		tmppub, err := ethbls.PublicKeyFromBytes(tmpbyte)
		if err != nil {
			fmt.Println(err)
			return false
		}
		blspubkeys = append(blspubkeys, tmppub)
	}

	aggsig, err := ethbls.SignatureFromBytes(asign)
	if err != nil {
		return false
	}

	return aggsig.VerifyAggregateCommon(blspubkeys, msg, 0)
}

func AggragateSignature(sig []byte, aggSigByte []byte) ([]byte, error) {
	if aggSigByte == nil {
		return sig, nil
	}

	// 就两个签名合并：原始的聚合签名，准备合并的签名
	sigs_inner := make([]*ethbls.Signature, 0, 2)

	if tmpsig, err := ethbls.SignatureFromBytes(aggSigByte); err == nil {
		sigs_inner = append(sigs_inner, tmpsig)
	} else {
		return nil, ErrBLSSignature
	}

	if tmpsig, err := ethbls.SignatureFromBytes(sig); err == nil {
		sigs_inner = append(sigs_inner, tmpsig)
	} else {
		return nil, ErrBLSSignature
	}

	return ethbls.AggregateSignatures(sigs_inner).Marshal(), nil
}

// 将相同消息的BLS签名聚合为一个签名
func AggragateAllSignature(sigs [][]byte) ([]byte, error) {
	sigs_inner := make([]*ethbls.Signature, 0, len(sigs))

	for i := 0; i < len(sigs); i++ {
		tmp, err := ethbls.SignatureFromBytes(sigs[i])
		if err != nil {
			return nil, err
		}
		sigs_inner = append(sigs_inner, tmp)
	}

	asig := ethbls.AggregateSignatures(sigs_inner)
	return asig.Marshal(), nil
}

// 将多个BLS的公钥聚合成一把公钥
func AggragatePubKey(pubkeys [][]byte) (crypto.PubKey, error) {
	aggPub := ethbls.NewAggregatePubkey()
	for i := 0; i < len(pubkeys); i++ {
		tmpbyte := make([]byte, len(pubkeys[i])-1, len(pubkeys[i])-1)
		if err := cdc.UnmarshalBinaryBare(pubkeys[i], &tmpbyte); err != nil {
			return nil, err
		}
		tmppub, err := ethbls.PublicKeyFromBytes(tmpbyte)
		if err != nil {
			return nil, err
		}
		aggPub.Aggregate(tmppub)
	}
	return PubKeyBLS{Pubkey: *aggPub}, nil
}
