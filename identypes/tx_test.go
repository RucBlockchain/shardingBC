package identypes_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"

	"github.com/tendermint/tendermint/identypes"
)

func TestNewTX(t *testing.T) {
	tx_bytes := []byte("{\"Txtype\": \"relayTx\", " +
		"\"Sender\": \"sender1\", " +
		"\"Receiver\": \"receiver\", " +
		"\"Content\": \"content\", " +
		"\"TxSigature\": \"sig\"}" )

	tx, err := identypes.NewTX(tx_bytes)

	assert.Nil(t, err)

	t.Log(tx)
}

func TestContent2PubKey(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub_s := priv.PublicKey

	t.Log("sender(str):;;;", pub2string(pub_s), ";;;")
	t.Log("sender:", pub_s)

	priv_r, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub_r := priv_r.PublicKey
	t.Log("receiver(str):;;;", pub2string(pub_r), ";;;")
	t.Log("receiver:", pub_r)
	tx_content := pub2string(pub_s) + "_" + pub2string(pub_r) + "_12.5";
	t.Log("tx_content: ", tx_content)

	s, err := identypes.Content2PubKey(tx_content)
	assert.Nil(t, err)
	t.Log("pub from content: ", s)
}

func TestTX_VerifySig(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub_s := priv.PublicKey

	//t.Log("sender(str):;;;", pub2string(pub_s), ";;;")
	//t.Log("sender:", pub_s)

	priv_r, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub_r := priv_r.PublicKey
	//t.Log("receiver(str):;;;", pub2string(pub_r), ";;;")
	//t.Log("receiver:", pub_r)
	tx_content := pub2string(pub_s) + "_" + pub2string(pub_r) + "_12.5";
	t.Log("tx_content: ", tx_content)

	s, err := identypes.Content2PubKey(tx_content)
	assert.Nil(t, err)
	t.Log("pub from content: ", s)

	tr, ts, err := ecdsa.Sign(rand.Reader, priv, digest(tx_content))
	sig := bigint2str(*tr, *ts)
	fmt.Println("before: ", sig)

	assert.Nil(t, err)
	t.Log(sig)

	tx_bytes := []byte("{\"Txtype\": \"relayTx\", " +
		"\"Sender\": \"sender\", " +
		"\"Receiver\": \"receiver\", " +
		"\"Content\": \"" + tx_content + "\", " +
		"\"TxSignature\": \"" + sig + "\"}"    )

	tx, err := identypes.NewTX(tx_bytes)
	assert.Nil(t, err)
	t.Log(tx)
	//r, s :=
	//	assert.True(ecdsa.Verify(priv.PublicKey, ))

	res := tx.VerifySig()
	assert.True(t, res)
}

type ecdsaSignature struct {
	X, Y *big.Int
}

func TestSig(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub := priv.PublicKey

	msg := []byte("testtest")

	sig, _ := priv.Sign(rand.Reader, msg, nil)
	t.Log("sig: ", sig)
	var sigb ecdsaSignature
	_, err := asn1.Unmarshal(sig, &sigb)
	t.Log(err)
	res := ecdsa.Verify(&pub, msg, sigb.X, sigb.Y)

	assert.True(t, res)
}

func digest(Content string) []byte {
	origin := []byte(Content)

	// 生成md5 hash值
	digest_md5 := md5.New()
	digest_md5.Write(origin)

	return digest_md5.Sum(nil)
}

func pub2string(pub ecdsa.PublicKey) string {

	coor := ecdsaSignature{X: pub.X, Y: pub.Y}
	b, _ := asn1.Marshal(coor)

	return hex.EncodeToString(b)
}

func bigint2str(r, s big.Int) string {
	coor := ecdsaSignature{X: &r, Y: &s}
	b, _ := asn1.Marshal(coor)
	return hex.EncodeToString(b)
}
