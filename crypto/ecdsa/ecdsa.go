package ecdsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

type ecdsaCoordinate struct {
	X, Y *big.Int
}

//-------------------------------------

var _ crypto.PrivKey = PrivKeyEcdsa{}

const (
	PrivKeyEcdsaName = "tendermint/PrivKeyEcdsa"
	PubKeyEcdsaName  = "tendermint/PubKeyEdcdsa"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeyEcdsa{},
		PubKeyEcdsaName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeyEcdsa{},
		PrivKeyEcdsaName, nil)
}

// PrivKeyEcdsa implements crypto.PrivKey.
type PrivKeyEcdsa struct {
	Priv *ecdsa.PrivateKey
}

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyEcdsa) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

// ecdsa签名的结果是放回两个大整数(随机数r、mod值s)
// 转换格式： asn1
func (tuple PrivKeyEcdsa) Sign(msg []byte) ([]byte, error) {

	r, s, err := ecdsa.Sign(rand.Reader, tuple.Priv, msg)
	if err != nil {
		return nil, err
	}
	var coor ecdsaCoordinate
	coor.Y = s
	coor.X = r

	return asn1.Marshal(coor)
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyEcdsa) PubKey() crypto.PubKey {
	return PubKeyEcdsa{Pub: privKey.Priv.PublicKey}
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
// TODO
func (privKey PrivKeyEcdsa) Equals(other crypto.PrivKey) bool {
	return true
}

// GenPrivKey generates a new ecdsa private key.
// It uses OS randomness in conjunction with the current global random seed
func GenPrivKey() PrivKeyEcdsa {
	return genPrivKey(crypto.CReader())
}

// genPrivKey generates a new ed25519 private key using the provided reader.
func genPrivKey(rand io.Reader) PrivKeyEcdsa {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand)
	if err != nil {
		panic("error when generate ecdsa private key.")
	}
	return PrivKeyEcdsa{Priv: priv}
}

//-------------------------------------

var _ crypto.PubKey = PubKeyEcdsa{}

type PubKeyEcdsa struct {
	Pub ecdsa.PublicKey
}

// address 格式, asn1
func (pubKey PubKeyEcdsa) Address() crypto.Address {
	b, _ := asn1.Marshal(ecdsaCoordinate{Y: pubKey.Pub.X, X: pubKey.Pub.X})

	return crypto.Address(tmhash.SumTruncated(b))
}

// Bytes marshals the PubKey using amino encoding.
func (pubKey PubKeyEcdsa) Bytes() []byte {
	b, _ := asn1.Marshal(ecdsaCoordinate{Y: pubKey.Pub.X, X: pubKey.Pub.X})

	return cdc.MustMarshalBinaryBare(b)
}

// ecdsa的签名是一对坐标
// sig格式： {r.len}{s.len}{r}{s}
func (pubKey PubKeyEcdsa) VerifyBytes(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	var coor ecdsaCoordinate
	_, err := asn1.Unmarshal(sig, &coor)
	if err != nil {
		return false
	}

	// 调用ecdsa的验证函数
	return ecdsa.Verify(&pubKey.Pub, msg, coor.X, coor.Y)
}

func (pubKey PubKeyEcdsa) String() string {
	return fmt.Sprintf("PubKeyEcdsa{%X}", pubKey.Pub.X.String()+pubKey.Pub.Y.String())
}

// nolint: golint
// TODO
func (pubKey PubKeyEcdsa) Equals(other crypto.PubKey) bool {
	return true
}
