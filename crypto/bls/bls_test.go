package bls_test

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/types/time"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	kbls "go.dedis.ch/kyber/v3/sign/bls"
	"testing"
)

// todo
// 序列化和反序列化重写
func TestGetPubkeyFromByte(t *testing.T) {
	priv := bls.GenPrivKey()
	pub := priv.PubKey()

	newpub, err := bls.GetPubkeyFromByte(pub.Bytes())
	t.Log(err)
	assert.NotNil(t, newpub, "公钥byte反序列失败")

	t.Log(pub)
	t.Log(newpub)
}

func TestSignatureRecovery(t *testing.T) {
	msg := []byte("test signature for threshold")
	d := 1000

	private := bls.GenPrivKey()
	pub := (private.PubKey()).(bls.PubKeyBLS)
	public := pub.Pubkey
	origin_sig, _ := private.Sign(msg)

	polynome := bls.Master(private.Priv, d)

	t.Log("share private: ", private)
	t.Log("polynome: ", polynome)

	//ids := []int{151, 2426, 45, 673, 123, 7564}
	ids := func(lens int) []int64 {
		data := make([]int64, lens, lens)
		for i := 1; i <= lens; i++ {
			data[i-1] = int64(i) //rand.Int() % 90000
		}
		return data
	}(d + 1)

	suite := bn256.NewSuite()

	sigs := [][]byte{}
	for _, id := range ids {
		x_i, err := polynome.GetValue(id)
		assert.Nil(t, err, "计算f(x)函数值出错。err：", err)
		if sig, err := kbls.Sign(suite, x_i, msg); err != nil {
			t.Error(err)
		} else {
			assert.Nil(t, kbls.Verify(suite, suite.G2().Point().Mul(x_i, nil), msg, sig))
			sigs = append(sigs, sig)
		}
	}
	tt := time.Now()
	sig, err := bls.SignatureRecovery(d, sigs, ids)
	t.Log(time.Now().Sub(tt).Seconds())
	t.Log("ids: ", ids)
	t.Log("sigs: ", sigs)
	t.Log("sig: ", sig)
	t.Log("origin sig: ", origin_sig)

	assert.Nil(t, err, "签名还原错误")
	assert.NotNil(t, sig, "还原的签名为空")
	assert.True(t, bytes.Equal(sig, origin_sig), "还原出来的签名和原始签名不相等")
	assert.Nil(t, kbls.Verify(suite, public, msg, sig), "还原出来的签名验证错误")
	assert.Nil(t, kbls.Verify(suite, public, msg, origin_sig), "还原出来的签名验证错误")
}

func TestBlsUse(t *testing.T) {
	// single signature
	msg := []byte("Hello Boneh-Lynn-Shacham")
	private := bls.GenPrivKey()
	public := private.PubKey()

	sig, err := private.Sign(msg)
	assert.Nil(t, err, "签名出错")
	t.Log(sig)

	assert.True(t, public.VerifyBytes(msg, sig), "验证签名失败")
}

func TestAVG(t *testing.T) {
	data := []float64{1.0, 2.0, 3.0, 4.0}
	t.Log(avg(data))
}

func avg(data []float64) float64 {
	sum := 0.0
	for _, d := range (data) {
		sum += d
	}
	return sum / float64(len(data))
}
