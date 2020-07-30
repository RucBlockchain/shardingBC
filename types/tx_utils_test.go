package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/identypes"
	"github.com/tendermint/tendermint/types/time"
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
	txs := randTX(txNum)
	buckets := ClassifyTx(txs)

	mts, err := GenerateMerkleTree(txs)
	assert.Nil(t, err, "生成merkle树出错，err: ", err)
	cms := ClassifyTxFromBlock(mts, txs, []byte{0}, []byte{1}, 10)
	assert.NotNil(t, cms)
	assert.True(t, len(cms) == len(buckets)-1)
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

func randTXContent() string {
	//产生转化交易金额
	num := rand.Intn(100)
	//最后加入时间戳信息
	tx_content := strconv.FormatInt(rand.Int63(), 10) +
		"_" + strconv.FormatInt(rand.Int63(), 10) +
		"_" + strconv.Itoa(num) +
		"_" + strconv.FormatInt(time.Now().UnixNano(), 10) //添加时间戳来唯一标识该交易

	return tx_content
}
