package types

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/identypes"
	"github.com/tendermint/tendermint/types/time"
	"sort"
	"strings"
)

// TODO
// 跨片的交易集只
// 考虑片内交易
// 片内交易是由到本片的交易和目的地为空的交易合一起
func ClassifyTxbySingle(txs Txs) map[string]Txs {
	// TODO string类型转换为枚举型
	buckets := make(map[string]Txs)
	for _, txbyte := range txs {
		//共识完成加入第三阶段的代码
		tx, err := identypes.NewTX(txbyte)
		if err != nil {
			continue
		}

		// 更新交易数据，可优化
		tx.UpdateTx()

		des := ""

		if tx.Txtype == "addtx" {
			des = "des_"+tx.Sender+"_"+tx.Content
		} else if tx.Txtype == "relaytx" {
			des = "des_"+tx.Receiver+"_"+tx.Content
		}
		if _, ok := buckets[des]; ok {
			buckets[des] = append(buckets[des], tx.Data())
		} else {
			//	键值不存在，新建
			buckets[des] = Txs{tx.Data()}
		}
	}

	// 将当前分片的交易和目的地为空的交易合并到一起
	if txlists, ok := buckets[""]; ok {
		cShard := getShard()
		buckets[cShard] = append(buckets[cShard], txlists...)
		delete(buckets, "")
	}

	return buckets
}

func ClassifyTx(txs Txs) map[string]Txs {
	// TODO string类型转换为枚举型
	buckets := make(map[string]Txs)
	for _, txbyte := range txs {
		//共识完成加入第三阶段的代码
		tx, err := identypes.NewTX(txbyte)
		if err != nil {
			continue
		}

		// 更新交易数据，可优化
		tx.UpdateTx()

		des := ""
		if tx.Txtype == "addtx" {
			des = tx.Sender
		} else if tx.Txtype == "relaytx" {
			des = tx.Receiver
		}

		if _, ok := buckets[des]; ok {
			buckets[des] = append(buckets[des], tx.Data())
		} else {
			//	键值不存在，新建
			buckets[des] = Txs{tx.Data()}
		}
	}

	// 将当前分片的交易和目的地为空的交易合并到一起
	if txlists, ok := buckets[""]; ok {
		cShard := getShard()
		buckets[cShard] = append(buckets[cShard], txlists...)
		delete(buckets, "")
	}

	return buckets
}

// 排序结果的顺序必须是abcd吗
// bug - cs.ProposalBlock在函数中会被修改为空，目前原因未知
// 独立出来一个函数来处理operator & txtype
func HandleSortTx(txs Txs) Txs {
	buckets := ClassifyTx(txs)
	cShard := getShard()

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))

	// newTxs先放入跨片交易
	newTxs := txs[:0]
	for _, shard := range keys {
		if shard == cShard {
			continue
		}
		listval := buckets[shard]
		for _, tx := range listval {
			newTxs = append(newTxs, tx)
		}
	}

	// 将当前分片的片内交易放在txs末尾
	for _, tx := range buckets[cShard] {
		newTxs = append(newTxs, tx)
	}

	return newTxs
}

// 首先将所有的交易按照目的地划分出若干包
// 为每一个包生成merkle root
// 把所有包的tree root组合起来生成一颗新的merkle tree
func GenerateMerkleTree(txs Txs) (*merkle.TxMerkleTree, error) {
	cShard := getShard()

	var cShardRoot []byte
	buckets := ClassifyTx(txs)
	roots := make([][]byte, 0, len(buckets))
	shardtrees := make(map[string]*merkle.SimpleMerkleTree)

	// 将buckets的keys按shard排序

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))

	// 依次为每一个分片的交易生成merkle tree
	for _, shard := range keys {
		tmptxs := buckets[shard]
		// 取出交易
		tmplist := make([][]byte, 0, len(tmptxs))
		for _, tx := range tmptxs {
			tmplist = append(tmplist, tx)
		}

		// 生成merkle tree
		merkleTree := merkle.SimpleTreeFromByteSlices(tmplist)
		if merkleTree == nil {
			return nil, errors.New("generate shard merkle tree failed, empty.")
		}

		shardtrees[shard] = merkleTree
		// 先把跨片交易的root hash加到队列里
		if shard != cShard {
			roots = append(roots, merkleTree.ComputeRootHash())
		} else {
			cShardRoot = merkleTree.ComputeRootHash()
		}
	}

	// 片内交易生成的merkle tree的root hash放在roots最后面
	roots = append(roots, cShardRoot)
	rootTree := merkle.SimpleTreeFromByteSlices(roots)
	if rootTree == nil {
		return nil, errors.New("generate root merkle tree failed, empty.")
	}

	return &merkle.TxMerkleTree{
		RootTree:   merkle.SimpleTreeFromByteSlices(roots),
		ShardTrees: shardtrees,
	}, nil
}
//压缩获取id
func GenerateMerkleTreebySingle(txs Txs) (*merkle.TxMerkleTree, error) {
	cShard := getShard()

	var cShardRoot []byte
	buckets := ClassifyTxbySingle(txs)
	roots := make([][]byte, 0, len(buckets))
	shardtrees := make(map[string]*merkle.SimpleMerkleTree)

	// 将buckets的keys按shard排序

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))

	// 依次为每一个分片的交易生成merkle tree
	for _, shard := range keys {
		tmptxs := buckets[shard]
		// 取出交易
		tmplist := make([][]byte, 0, len(tmptxs))
		for _, tx := range tmptxs {
			tmplist = append(tmplist, tx)
		}

		// 生成merkle tree
		merkleTree := merkle.SimpleTreeFromByteSlices(tmplist)
		if merkleTree == nil {
			return nil, errors.New("generate shard merkle tree failed, empty.")
		}

		shardtrees[shard] = merkleTree
		// 先把跨片交易的root hash加到队列里
		if shard != cShard {
			roots = append(roots, merkleTree.ComputeRootHash())
		} else {
			cShardRoot = merkleTree.ComputeRootHash()
		}
	}

	// 片内交易生成的merkle tree的root hash放在roots最后面
	roots = append(roots, cShardRoot)
	rootTree := merkle.SimpleTreeFromByteSlices(roots)
	if rootTree == nil {
		return nil, errors.New("generate root merkle tree failed, empty.")
	}

	return &merkle.TxMerkleTree{
		RootTree:   merkle.SimpleTreeFromByteSlices(roots),
		ShardTrees: shardtrees,
	}, nil
}
func Getbyteid(cm *identypes.CrossMessages)[]byte{
	data,err:=json.Marshal(cm)
	if err!=nil {
		fmt.Println("反序列化CM出错")
		return nil
	}
	return data
}
func CmKey(cm []byte) [sha256.Size]byte {
	return sha256.Sum256(cm)
}
// 在调用该函数前，需处理好tx的operate数值修改
//修改生成cm包的细节
func ClassifyTxFromBlock(mts *merkle.TxMerkleTree,
	txs Txs,
	signature []byte,
	pubkey []byte,
	height int64) []*identypes.CrossMessages {
	if txs == nil || len(txs) == 0 {
		return nil
	}

	cms := make([]*identypes.CrossMessages, 0, len(txs))

	// 分桶
	buckets := ClassifyTxbySingle(txs)
	cShard := getShard()

	// 生成最终的merkle tree
	//mts, err := GenerateMerkleTree(txs)
	target := ""
	for shard, listval := range buckets {

		if shard == cShard || shard == "" {
			// 当前桶为片内交易
			continue
		}
			//对shard进行分解
		args := strings.Split(shard, "_")
		target = args[1]
		//获取该交易的merkle tree
		mt := mts.Find(shard)

		treepath, err := mts.RootTree.GetPathByValue(mt.ComputeRootHash())
		if err != nil || treepath == "" {
			fmt.Println("path为空")
			continue
		}
		cm := identypes.NewCrossMessage(listval.Bytes(),
			signature,
			pubkey,
			mts.RootTree.ComputeRootHash(),
			treepath, cShard, target, height,time.Now().UnixNano())
		cm.ID = CmKey(Getbyteid(cm))
		cms = append(cms, cm)
	}
	return cms
}
