// bls包是对prysmaticlabs/prysm/shared/bls的封装
package bls

import (
	"errors"
	"fmt"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/util/random"
	"io"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

//-------------------------------------

var (
	ErrBLSSignature       = errors.New("Invalid BLS signature")
	ErrAggregateSignature = errors.New("Error in BLS aggregate process")
	bn256_suite           = bn256.NewSuite()
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


	cdc.RegisterInterface((*kyber.Point)(nil), nil)
	cdc.RegisterInterface((*kyber.Scalar)(nil), nil)

}

// PrivKeyBLS implements crypto.PrivKey.
type PrivKeyBLS struct {
	Priv kyber.Scalar
}

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey.Bytes())
}

func (privKey PrivKeyBLS) Sign(msg []byte) ([]byte, error) {
	if privKey.Priv == nil {
		return nil, errors.New("private key is empty")
	}
	if len(msg) == 0 {
		return nil, errors.New("msg is empty")
	}

	if sig, err := bls.Sign(bn256_suite, privKey.Priv, msg); err != nil {
		return nil, errors.New("bls sign msg failed.")
	} else {
		return sig, nil
	}
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyBLS) PubKey() crypto.PubKey {
	pub := bn256_suite.G2().Point().Mul(privKey.Priv, nil)
	return PubKeyBLS{pub}
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
	priv, _ := bls.NewKeyPair(bn256_suite, random.New(rander))
	return PrivKeyBLS{Priv: priv}
}

//-------------------------------------

var _ crypto.PubKey = PubKeyBLS{}

type PubKeyBLS struct {
	Pubkey kyber.Point
}

func (pubKey PubKeyBLS) Address() crypto.Address {
	b, _ := pubKey.Pubkey.Data()
	return crypto.Address(tmhash.SumTruncated(b))
}

// 返回公钥完整的byte
func (pubKey PubKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(pubKey)
	//if b, err := pubKey.Pubkey.MarshalBinary(); err == nil {
	//	return cdc.MustMarshalBinaryBare(b)
	//} else {
	//	fmt.Println(err)
	//	return nil
	//}
}

func (pubKey PubKeyBLS) VerifyBytes(msg []byte, sig []byte) bool {
	if len(sig) == 0 {
		return false
	}

	// byte还原为bls的Sign结构
	if err := bls.Verify(bn256_suite, pubKey.Pubkey, msg, sig); err == nil {
		return true
	} else {
		return false
	}
}

func (pubKey PubKeyBLS) String() string {
	return fmt.Sprintf("PubKeyBLS{%X}", pubKey.Pubkey.String())
}

// nolint: golint
// TODO
func (pubKey PubKeyBLS) Equals(other crypto.PubKey) bool {
	return true
}

func GetPubkeyFromByte(data []byte) (*PubKeyBLS, error) {

	pub := bn256_suite.G2().Point()

	if err := cdc.UnmarshalBinaryBare(data, pub); err != nil {
		return nil, err
	} else {
		return &PubKeyBLS{Pubkey: pub}, nil
	}
	//if err := pub.UnmarshalBinary(newdata); err == nil {
	//	return &PubKeyBLS{Pubkey: pub}, err
	//} else {
	//	return nil, err
	//}
}
