package bls_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/types/time"
	"testing"
)

func TestGetPubkeyFromByte(t *testing.T) {
	priv := bls.GenPrivKey()
	pub := priv.PubKey()

	newpub := bls.GetPubkeyFromByte(pub.Bytes())

	t.Log(pub)
	t.Log(newpub)
}

func TestAggragatePubKey(t *testing.T) {
	totaln := 10

	pubs := make([][]byte, 0, totaln)

	for i := 0; i < totaln; i++ {
		priv := bls.GenPrivKey()
		pub := priv.PubKey()
		pubs = append(pubs, pub.Bytes())
	}

	aggpub, err := bls.AggregatePubkey(pubs)
	if err != nil {
		t.Error(err)
	}
	t.Log("aggegate pubkey: ", aggpub)
}

func TestBLSSignAndVerify(t *testing.T) {
	priv := bls.GenPrivKey()
	pub := priv.PubKey()

	msg := []byte("test BLS signature")
	tt := time.Now()
	for i := 0; i < 1000; i += 1 {
		_, err := priv.Sign(msg)
		if err != nil {
			t.Error(err)
		}
	}
	t.Log("generate sign 1000 times cost: ", time.Now().Sub(tt).Seconds()/1000, "s")

	sig, err := priv.Sign(msg)
	if err != nil {
		t.Error(err)
	}

	assert.True(t, pub.VerifyBytes(msg, sig), "verify signature failed.")

	tt = time.Now()
	for i := 0; i < 1000; i += 1 {
		pub.VerifyBytes(msg, sig)
	}
	t.Log("verify sign 1000 times cost: ", time.Now().Sub(tt).Seconds()/1000, "s")

}

func TestAggragateSignature(t *testing.T) {
	totaln := 1000

	pubs := make([][]byte, 0, totaln)
	sigs := make([][]byte, 0, totaln)
	msg := []byte("test alsk okjoizxcv 098u32 vcnl234 234 123sdf**$#%@# #szdlgfjalksdjf lasd asdfu aisduf oasdufao9sdf ")

	for i := 0; i < totaln; i++ {
		priv := bls.GenPrivKey()
		pub := priv.PubKey()
		pubs = append(pubs, pub.Bytes())
		sig, err := priv.Sign(msg)
		if err != nil {
			t.Error("generate signature failed.")
		}
		assert.True(t, pub.VerifyBytes(msg, sig))
		sigs = append(sigs, sig)
	}

	as, err := bls.AggregateAllSignature(sigs)

	assert.Nil(t, err)

	//pubs[0], pubs[1] = pubs[1], pubs[0]

	pub, err := bls.AggregatePubkey(pubs)
	assert.Nil(t, err)

	// 三种验证聚合签名的方式
	assert.True(t, bls.AggregateVerify(as, msg, pubs))
	assert.True(t, bls.AggregateVerifyNP(as, msg, pub.Bytes()))
	assert.True(t, pub.VerifyBytes(msg, as))

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
