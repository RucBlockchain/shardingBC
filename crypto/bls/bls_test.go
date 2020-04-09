package bls_test

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/types/time"
	"testing"
)

func TestAggragatePubKey(t *testing.T) {
	totaln := 10

	pubs := make([][]byte, 0, totaln)

	for i := 0; i < totaln; i++ {

		priv := bls.GenPrivKey()
		pub := priv.PubKey()
		pubs = append(pubs, pub.Bytes())
	}

	aggpub, err := bls.AggragatePubKey(pubs)
	if err != nil {
		t.Error(err)
	}
	t.Log("aggegate pubkey: ", aggpub)
}

func TestBLSSignAndVerify(t *testing.T) {
	priv := bls.GenPrivKey()
	pub := priv.PubKey()
	t.Log("priv: ", priv.Bytes())
	t.Log("pub: ", pub.Bytes())

	msg := []byte("test BLS signature")

	sig, err := priv.Sign(msg)
	if err != nil {
		t.Error(err)
	}
	t.Log("sig: ", sig)

	res := pub.VerifyBytes(msg, sig)
	if !res {
		t.Error("verify signature failed.")
	}
}

func TestAggragateSignature(t *testing.T) {
	totaln := 10

	pubs := make([][]byte, 0, totaln)
	sigs := make([][]byte, 0, totaln)
	msg := [32]byte{'t', 'e', 's', 't'}

	for i := 0; i < totaln; i++ {

		priv := bls.GenPrivKey()
		pub := priv.PubKey()
		pubs = append(pubs, pub.Bytes())
		sig, err := priv.Sign(msg[:])
		if err != nil {
			t.Error("generate signature failed.")
		}
		sigs = append(sigs, sig)
	}

	t.Log("generate aggragate signature")
	aggsig, err := bls.AggragateSignature(sigs)
	if err != nil {
		t.Error("generate aggragate signature failed, err: ", err)
	}
	t.Log("aggragate signature: ", aggsig)
	t.Log("verify aggragate signature")
	res := bls.AggragateVerify(aggsig, msg, pubs)
	if !res {
		t.Error("verify aggragate signature failed.")
	}
}

func TestAggragateCost(t *testing.T) {
	totaln := []int{20, 40, 60, 80, 100, 200, 400, 600, 800, 1000}

	privs := make([]crypto.PrivKey, 0, 1000)
	pubs := make([][]byte, 0, 1000)
	msg := [32]byte{'t', 'e', 's', 't'}

	// 提前生成好private key
	for i := 0; i < 1000; i++ {
		priv := bls.GenPrivKey()
		privs = append(privs, priv)
		pubs = append(pubs, priv.PubKey().Bytes())
	}

	// 多次试验计时，每轮重复3次取平均值
	for _, n := range (totaln) {
		aggSigTimes := make([]float64, 3, 3)
		aggVerTimes := make([]float64, 3, 3)
		for i := 0; i < 3; i++ {

			// 产生n个签名用来聚合测试计时
			sigs := make([][]byte, 0, n)
			for txn := 0; txn < n; txn++ {
				sig, err := privs[txn].Sign(msg[:])
				if err != nil {
					t.Fail()
				}
				sigs = append(sigs, sig)
			}

			// 统计聚合用时
			agg_start := time.Now()
			aggsig, err := bls.AggragateSignature(sigs)
			if err != nil {
				t.Fail()
			}
			aggSigTimes[i] = time.Now().Sub(agg_start).Seconds()

			verify_start := time.Now()
			res := bls.AggragateVerify(aggsig, msg, pubs[0:n])
			if !res {
				t.Fail()
			}
			aggVerTimes[i] = time.Now().Sub(verify_start).Seconds()
		}
		t.Logf("%v tx, aggrate signature cost: %v", n, avg(aggSigTimes)*1000)
		t.Logf("%v tx, verify  signature cost: %v", n, avg(aggVerTimes)*1000)
	}
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
