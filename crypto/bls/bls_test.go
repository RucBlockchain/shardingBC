package bls_test

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/types/time"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	kbls "go.dedis.ch/kyber/v3/sign/bls"
	"syscall"
	"testing"
)

var bn256_suite = bn256.NewSuite()

// todo
// 序列化和反序列化重写
func TestGetPubkeyFromByte(t *testing.T) {
	priv := bls.GenPrivKey()
	pub := priv.PubKey()

	t.Log(priv)
	t.Log(pub)

	newpub, err := bls.GetPubkeyFromByte(pub.Bytes())
	assert.Nil(t, err, err)
	assert.NotNil(t, newpub, "公钥byte反序列失败")
	assert.True(t, pub.Equals(newpub), "序列化前后数据不一致")
}

func TestSignatureRecovery(t *testing.T) {
	msg := []byte("test signature for threshold")
	d := 100

	private := bls.GenPrivKey()
	t.Log(len(private))
	pub := (private.PubKey()).(bls.PubKeyBLS)
	public := bn256_suite.G2().Point()
	err := public.UnmarshalBinary(pub)
	assert.Nil(t, err)

	origin_sig, _ := private.Sign(msg)

	priv := bn256_suite.G2().Scalar().One()
	err = priv.UnmarshalBinary(private)
	assert.Nil(t, err)

	polynome := bls.Master(priv, d)

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
	t.Log(polynome)
	sigs := [][]byte{}
	for _, id := range ids {
		x_i, err := polynome.GetValue(id)
		ss, _ := x_i.MarshalBinary()
		t.Log(id, len(ss), x_i)
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

func TestMaster(t *testing.T) {
	d := 3

	for i := 0; i < 3; i++ {
		private := bls.GenPrivKey()
		priv := bn256_suite.G2().Scalar().One()
		err := priv.UnmarshalBinary(private)
		assert.Nil(t, err)
		polynome := bls.Master(priv, d)

		t.Log(polynome)
	}
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

func TestGenPrivKey(t *testing.T) {
	syscall.Setenv("TASKID", "11")
	priv := bls.GenPrivKey()
	t.Log(priv)

	priv1 := bls.GenPrivKey()
	t.Log(priv)

	assert.True(t, priv.Equals(priv1))

	syscall.Setenv("TASKID", "12")

	priv2 := bls.GenPrivKey()
	t.Log(priv2)

	assert.False(t, priv.Equals(priv2))
}

func TestGenSubPrivKey(t *testing.T) {
	syscall.Setenv("TASKID", "1")
	syscall.Setenv("TASKINDEX", "1")
	priv11 := bls.GenSubPrivKey(3)
	t.Log(priv11)

	syscall.Setenv("TASKINDEX", "2")
	priv12 := bls.GenSubPrivKey(3)
	t.Log(priv12)

	syscall.Setenv("TASKID", "2")
	syscall.Setenv("TASKINDEX", "1")
	priv21 := bls.GenSubPrivKey(3)
	t.Log(priv21)

	syscall.Setenv("TASKINDEX", "2")
	priv22 := bls.GenSubPrivKey(3)
	t.Log(priv22)

	assert.False(t, priv11.Equals(priv12))
	assert.False(t, priv11.Equals(priv21))
	assert.False(t, priv11.Equals(priv22))
	assert.False(t, priv12.Equals(priv21))
	assert.False(t, priv12.Equals(priv22))
	assert.False(t, priv21.Equals(priv22))
}

func avg(data []float64) float64 {
	sum := 0.0
	for _, d := range data {
		sum += d
	}
	return sum / float64(len(data))
}
