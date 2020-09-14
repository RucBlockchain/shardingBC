package types

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/identypes"
	"github.com/tendermint/tendermint/types/time"
	"io/ioutil"
	"math/big"
	"math/rand"
	"strconv"
	"testing"
)

func int2byte(n int) []byte {
	bytebuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytebuffer, binary.LittleEndian, int32(n))
	return bytebuffer.Bytes()
}

var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

func TestGenerateMerkleTree(t *testing.T) {
	TxNum := 30
	txs := randTX(TxNum)
	smt, err := GenerateMerkleTree(txs) // todo 类型装换
	assert.Nil(t, err, "生成merkle tree错误, err:", err)
	assert.NotNil(t, smt)
	assert.NotNil(t, smt.RootTree)
	assert.NotNil(t, smt.RootTree.ComputeRootHash())
	t.Log("tree: ", smt)
	t.Log("root hash: ", smt.RootTree.ComputeRootHash())
	totaltxs := 0
	for _, val := range smt.ShardTrees {
		totaltxs += val.Total
	}
	assert.True(t, TxNum == totaltxs)

	for i := 0; i < 100; i++ {
		tmp, _ := GenerateMerkleTree(txs)
		assert.True(t, bytes.Equal(tmp.RootTree.ComputeRootHash(), smt.RootTree.ComputeRootHash()), i)
	}
}

func TestHandleSortTxInConsenses(t *testing.T) {
	txNum := 100

	// 随机生成并且设置交易
	randTxs := randTX(txNum)

	t.Log("[handleTxSorts] 排序前")
	for _, data := range randTxs {
		tx, _ := identypes.NewTX(data)
		t.Log(tx)
	}

	randTxs1 := HandleSortTx(randTxs)

	t.Log("[handleTxSorts] 排序后")
	for _, data := range randTxs1 {
		tx, _ := identypes.NewTX(data)
		t.Log(tx)
	}

	assert.Equal(t, txNum, len(randTxs1))
}

func TestClassifyTx(t *testing.T) {
	tuNum := 30
	txs := randTX(tuNum)

	start := time.Now()
	buckets := ClassifyTx(txs) // todo 类型转换
	t.Logf("%v个交易分类耗时：%v", tuNum, time.Now().Sub(start).String())

	for shard, data := range buckets {
		for _, tx := range data {
			tx, _ := identypes.NewTX(tx)
			assert.True(t, tx.Receiver == shard, "%v tx in %v shard", tx.Receiver, shard)
			t.Log(tx)
		}
	}
}

func TestClassifyTxFromBlock(t *testing.T) {
	txNum := 20
	txs := randTX1(txNum)
	buckets := ClassifyTx(txs)

	mts, err := GenerateMerkleTree(txs)
	assert.Nil(t, err, "生成merkle树出错，err: ", err)
	cms := ClassifyTxFromBlock(mts, txs, []byte{0}, []byte{1}, 10)
	assert.NotNil(t, cms)
	assert.True(t, len(cms) == len(buckets)-1)
}

func TestCrossMessageCompressAndDecom(t *testing.T) {
	txNum := 5000
	txs := randTX1(txNum)
	buckets := ClassifyTx(txs)

	mts, err := GenerateMerkleTree(txs)
	assert.Nil(t, err, "生成merkle树出错，err: ", err)
	cms := ClassifyTxFromBlock(mts, txs, []byte{0}, []byte{1}, 10)

	assert.NotNil(t, cms)
	assert.True(t, len(cms) == len(buckets))

	for ith, cm := range cms {
		txlist := cm.Txlist

		res := cm.Compress()
		assert.Truef(t, res, "%v cm compress fail", ith)
		assert.NotNil(t, cm.CompressedData)
		assert.Nil(t, cm.Txlist)
		assert.True(t, cm.IsCompressed)
		res = cm.Compress()
		assert.Truef(t, res, "%v cm compress fail", ith)
		assert.NotNil(t, cm.CompressedData)
		assert.Nil(t, cm.Txlist)
		assert.True(t, cm.IsCompressed)

		res = cm.Decompression()
		assert.Truef(t, res, "%v cm decompression fail", ith)
		assert.NotNil(t, cm.Txlist)
		assert.Nil(t, cm.CompressedData)
		assert.False(t, cm.IsCompressed)
		res = cm.Decompression()
		assert.Truef(t, res, "%v cm decompression fail", ith)
		assert.NotNil(t, cm.Txlist)
		assert.Nil(t, cm.CompressedData)
		assert.False(t, cm.IsCompressed)

		for i, tx := range txlist {
			assert.True(t, bytes.Equal(tx, cm.Txlist[i]))
		}
	}

}

func TestGzip(t *testing.T) {
	txNums := []int{50, 200, 600, 1000, 2000, 3000, 4000, 5000}
	origin_sizes := [8]float64{}
	// 单位为s
	compress_times := [8]float64{}
	decompression_times := [8]float64{}
	rates := [8]float64{}
	SIZE := func(size int) float64 {
		return float64(size) / 1024.0 / 1024.0
	}

	for i, txNum := range txNums {
		txs := randTX1(txNum)
		mts, err := GenerateMerkleTree(txs)
		assert.Nil(t, err, "生成merkle树出错，err: ", err)
		cm := ClassifyTxFromBlock(mts, txs, []byte{0}, []byte{1}, 10)[0]

		origin_data, err := json.Marshal(cm)
		data := make([]byte, len(origin_data))
		copy(data, origin_data)
		var buf bytes.Buffer

		start := time.Now()
		zw := gzip.NewWriter(&buf)

		_, err = zw.Write(data)
		zw.Flush()
		compress_times[i] = time.Now().Sub(start).Seconds()
		err = zw.Close()

		compressed := buf.Bytes()
		var newbuf bytes.Buffer
		newbuf.Write(compressed)

		start = time.Now()
		zr, _ := gzip.NewReader(&newbuf)
		data_de, err := ioutil.ReadAll(zr)
		decompression_times[i] = time.Now().Sub(start).Seconds()
		rates[i] = SIZE(len(compressed)) / SIZE(len(origin_data))
		origin_sizes[i] = SIZE(len(origin_data))
		assert.True(t, bytes.Equal(data_de, origin_data))
	}

	t.Log("压缩用时：", compress_times)
	t.Log("解压缩用时：", decompression_times)
	t.Log("压缩比", rates)
	t.Log("原始大小", origin_sizes)
}

func TestZlib(t *testing.T) {
	txNums := []int{50, 200, 600, 1000, 2000, 3000, 4000, 5000}
	origin_sizes := [8]float64{}
	// 单位为s
	compress_times := [8]float64{}
	decompression_times := [8]float64{}
	rates := [8]float64{}
	SIZE := func(size int) float64 {
		return float64(size) / 1024.0 / 1024.0
	}

	for i, txNum := range txNums {
		txs := randTX1(txNum)
		mts, err := GenerateMerkleTree(txs)
		assert.Nil(t, err, "生成merkle树出错，err: ", err)
		cm := ClassifyTxFromBlock(mts, txs, []byte{0}, []byte{1}, 10)[0]

		origin_data, err := json.Marshal(cm)
		data := make([]byte, len(origin_data))
		copy(data, origin_data)
		var buf bytes.Buffer

		start := time.Now()
		zw := zlib.NewWriter(&buf)

		_, err = zw.Write(data)
		zw.Flush()
		compress_times[i] = time.Now().Sub(start).Seconds()
		err = zw.Close()

		compressed := buf.Bytes()

		start = time.Now()
		zr, _ := zlib.NewReader(&buf)
		data_de, _ := ioutil.ReadAll(zr)
		decompression_times[i] = time.Now().Sub(start).Seconds()
		rates[i] = SIZE(len(compressed)) / SIZE(len(origin_data))
		origin_sizes[i] = SIZE(len(origin_data))
		assert.True(t, bytes.Equal(data_de, origin_data))
	}

	t.Log("压缩用时：", compress_times)
	t.Log("解压缩用时：", decompression_times)
	t.Log("压缩比", rates)
	t.Log("原始大小", origin_sizes)

}

// 随机生成一定数量的relaytx和addtx
func randTX(txNum int) Txs {
	res := make([]Tx, txNum, txNum)
	destination := []string{"A", "B", "C", "D", "E"}
	txtype := []string{"addtx", "relaytx", ""}
	for i := 0; i < txNum; i++ {
		//tmpdes := destination[rand.Intn(5)] // [0-n)
		content := randTXContent()
		tx := &identypes.TX{
			Txtype:      txtype[rand.Intn(len(txtype))],
			Sender:      "Neo",
			Receiver:    destination[rand.Intn(len(destination))],
			ID:          sha256.Sum256([]byte(content)),
			Content:     content,
			TxSignature: "signature",
			Operate:     1,
			Height:      rand.Int(),
		}
		res[i] = tx.Data()
	}

	return res
}

func randTX1(txNum int) Txs {
	res := make([]Tx, txNum, txNum)
	destination := []string{"A", "B", "C", "D", "E"}
	txtype := []string{"relaytx"}
	for i := 0; i < txNum; i++ {
		//tmpdes := destination[rand.Intn(5)] // [0-n)
		content, sig := randTXContent1()
		tx := &identypes.TX{
			Txtype:      txtype[rand.Intn(len(txtype))],
			Sender:      "Neo",
			Receiver:    destination[0],
			ID:          sha256.Sum256([]byte(content)),
			Content:     content,
			TxSignature: sig,
			Operate:     1,
			Height:      rand.Int(),
		}
		res[i] = tx.Data()
	}

	return res
}

func randTXContent() string {
	return ""
}

func randTXContent1() (string, string) {
	//rate指的是该分片账户数量
	source := rand.NewSource(time.Now().Unix())
	newrand := rand.New(source)
	//产生转化交易金额
	num := newrand.Intn(100)

	//拿取支付方账户信息
	// rand.New(rand.NewSource(int64(1)))
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	pub_s := priv.PublicKey

	//拿取转入方账户信息
	//  rand.New(rand.NewSource(int64(1)))
	priv_r, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	pub_r := priv_r.PublicKey
	//最后加入时间戳信息
	tx_content := pub2string(pub_s) + "_" + pub2string(pub_r) + "_" + strconv.Itoa(num) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10) //添加时间戳来唯一标识该交易

	tr, ts, _ := ecdsa.Sign(crand.Reader, priv, digest(tx_content))
	sig := bigint2str(*tr, *ts)

	return tx_content, sig
}

func bigint2str(r, s big.Int) string {
	coor := ecdsaSignature{X: &r, Y: &s}
	b, _ := asn1.Marshal(coor)
	return hex.EncodeToString(b)
}
func digest(Content string) []byte {
	origin := []byte(Content)

	// 生成md5 hash值
	digest_md5 := md5.New()
	digest_md5.Write(origin)

	return digest_md5.Sum(nil)
}

type ecdsaSignature struct {
	X, Y *big.Int
}

func pub2string(pub ecdsa.PublicKey) string {

	coor := ecdsaSignature{X: pub.X, Y: pub.Y}
	b, _ := asn1.Marshal(coor)

	return hex.EncodeToString(b)
}
