// bls包是对prysmaticlabs/prysm/shared/bls的封装
package bls

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/util/random"
	"golang.org/x/exp/rand"
	"io"
	"strconv"
	"syscall"
)

//-------------------------------------

var (
	ErrBLSSignature       = errors.New("Invalid BLS signature")
	ErrAggregateSignature = errors.New("Error in BLS aggregate process")
	bn256_suite           = bn256.NewSuite()
	test_seed             = "#### ##.Reader ##### ## be #### for testin"
)

var _ crypto.PrivKey = PrivKeyBLS{}

const (
	PrivKeyAminoName = "tendermint/PrivKeyBLS"
	PubKeyAminoName  = "tendermint/PubKeyBLS"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeyBLS{},
		PubKeyAminoName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeyBLS{},
		PrivKeyAminoName, nil)

}

// PrivKeyBLS implements crypto.PrivKey.
//type PrivKeyBLS struct {
//	Priv []byte
//	//Priv kyber.Scalar
//}

type PrivKeyBLS []byte

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

func (privKey PrivKeyBLS) Sign(msg []byte) ([]byte, error) {
	if len(msg) == 0 {
		return nil, errors.New("msg is empty")
	}

	priv := bn256_suite.G2().Scalar().One()
	if err := priv.UnmarshalBinary(privKey); err != nil {
		return nil, err
	}
	if sig, err := bls.Sign(bn256_suite, priv, msg); err != nil {
		return nil, errors.New("bls sign msg failed.")
	} else {
		return sig, nil
	}
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyBLS) PubKey() crypto.PubKey {
	pubkeyBytes := make([]byte, len(privKey))
	copy(pubkeyBytes, privKey)
	priv := bn256_suite.G2().Scalar().One()
	if err := priv.UnmarshalBinary(pubkeyBytes); err != nil {
		return nil
	}
	pub := bn256_suite.G2().Point().Mul(priv, nil)
	if data, err := pub.MarshalBinary(); err != nil {
		return nil
	} else {
		return PubKeyBLS(data)
	}
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeyBLS) Equals(other crypto.PrivKey) bool {
	return bytes.Equal(privKey.Bytes(), other.Bytes())
}

// GenPrivKey generates a new BLS private key.
// It uses OS randomness in conjunction with the current global random seed
func GenPrivKey() PrivKeyBLS {
	return genPrivKey(crypto.CReader())
}

func GenSubPrivKey(threshold int) PrivKeyBLS {
	priv_byte := genPrivKey(crypto.CReader())

	priv := bn256_suite.G2().Scalar().One()
	if err := priv.UnmarshalBinary(priv_byte); err != nil {
		return nil
	}
	polynome := Master(priv, threshold)
	if x, err := polynome.GetValue(parseId()); err == nil {
		data, _ := x.MarshalBinary()
		return PrivKeyBLS(data)
	} else {
		fmt.Println(err)
	}

	return genPrivKey(crypto.CReader())
}

// genPrivKey generates a new bls private key using the provided reader.
func genPrivKey(rander io.Reader) PrivKeyBLS {
	v, _ := syscall.Getenv("TASKID")
	Shard, _ := strconv.Atoi(v)
	var id int64
	id = int64(Shard * 500)
	tmp := rand.New(rand.NewSource(uint64(id)))
	cipher1 := random.New(tmp)

	priv, _ := bls.NewKeyPair(bn256_suite, cipher1)
	data, _ := priv.MarshalBinary()
	return PrivKeyBLS(data)
}

func GetShardPubkey() []byte {
	priv_byte := genPrivKey(crypto.CReader())

	priv := bn256_suite.G2().Scalar().One()
	if err := priv.UnmarshalBinary(priv_byte); err != nil {
		return nil
	}
	data, _ := priv.MarshalBinary()

	return PrivKeyBLS(data).PubKey().Bytes()
}

func TmpGetSign(msg []byte) ([]byte, error) {
	priv := genPrivKey(crypto.CReader())
	sig, err := priv.Sign(msg)
	return sig, err
}

//-------------------------------------

var _ crypto.PubKey = PubKeyBLS{}

//type PubKeyBLS struct {
//	Pubkey kyber.Point
//}
type PubKeyBLS []byte

func (pubKey PubKeyBLS) Address() crypto.Address {
	b := pubKey.Bytes()
	return crypto.Address(tmhash.SumTruncated(b))
}

// 返回公钥完整的byte
func (pubKey PubKeyBLS) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(pubKey)
}

func (pubKey PubKeyBLS) VerifyBytes(msg []byte, sig []byte) bool {
	if len(sig) == 0 {
		return false
	}
	pub := bn256_suite.G2().Point()
	if err := pub.UnmarshalBinary(pubKey); err != nil {
		return false
	}

	// byte还原为bls的Sign结构
	if err := bls.Verify(bn256_suite, pub, msg, sig); err == nil {
		return true
	} else {
		return false
	}
}

func (pubKey PubKeyBLS) String() string {
	return fmt.Sprintf("PubKeyBLS{%X}", pubKey.Bytes())
}

// nolint: golint
func (pubKey PubKeyBLS) Equals(other crypto.PubKey) bool {
	return bytes.Equal(pubKey.Bytes(), other.Bytes())
}

func GetPubkeyFromByte(data []byte) (*PubKeyBLS, error) {
	//pub := bn256_suite.G2().Point()
	pub := PubKeyBLS{}
	cdc.MustUnmarshalBinaryBare(data, &pub)
	return &pub, nil
}
func GetPubkeyFromByte2(data []byte) (PubKeyBLS, error) {
	//pub := bn256_suite.G2().Point()
	pub := PubKeyBLS{}
	cdc.MustUnmarshalBinaryBare(data, &pub)
	return pub, nil
}
//分割字符串得到相应的内容,默认容器名为：1_0 分片名_分片的index
func parseId() int64 {
	v, _ := syscall.Getenv("TASKID")
	g, _ := syscall.Getenv("TASKINDEX")
	Shard, _ := strconv.Atoi(v)
	Index, _ := strconv.Atoi(g)
	var id int64
	id = int64(Shard*500 + Index)
	return id
}
