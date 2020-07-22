package merkle

import (
	"bytes"
	"fmt"
	"math/bits"
)

func SimpleTreeFromByteSlices(items [][]byte) (smt *SimpleMerkleTree) {
	if items == nil || len(items) == 0 {
		return nil
	}
	trails, root := trailsFromByteSlices(items)
	smt = &SimpleMerkleTree{
		Total:  len(items),
		Root:   root,
		Leaves: trails,
	}
	return
}

type SimpleMerkleTree struct {
	Total  int                `json:"total"` // Total number of items.
	Root   *SimpleProofNode   // 树根
	Leaves []*SimpleProofNode // 叶子节点
}

func (smt *SimpleMerkleTree) ComputeRootHash() []byte {
	if smt.Root != nil {
		return smt.Root.Hash
	}
	return nil
}

// 直接通过叶子原始数据获得path
func (smt *SimpleMerkleTree) GetPathByValue(tx []byte) (string, error) {
	if tx == nil || len(tx) == 0 {
		return "", nil
	}
	hashVal := leafHash(tx)
	return smt.getPathByHash(hashVal)
}

func (smt *SimpleMerkleTree) GetPathByIndex(ith int) (string, error) {
	ith -= 1
	if ith < 0 || ith > smt.Total {
		return "", nil
	}

	node := smt.Leaves[ith]
	if node == nil {
		return "", nil
	}
	return smt.getPathByHash(node.Hash)
}

// 返回的KeyPath是从叶子节点到树根的tree path
func (smt *SimpleMerkleTree) getPathByHash(item []byte) (string, error) {
	// finc the leafnode
	var node *SimpleProofNode
	for _, leaf := range (smt.Leaves) {
		if bytes.Equal(item, leaf.Hash) == true {
			node = leaf
			break
		}
	}
	var res KeyPath
	directs := []byte{}
	//res = res.AppendKey(node.Hash, KeyEncodingHex)
	// 从叶子节点回溯到树根，找出tree path
	for node != smt.Root {
		if node.Left != nil {
			res = res.AppendKey(node.Left.Hash, KeyEncodingHex)
			directs = append(directs, PathRight)
		} else if node.Right != nil {
			res = res.AppendKey(node.Right.Hash, KeyEncodingHex)
			directs = append(directs, PathLeft)
		}
		node = node.Parent
	}
	//directs = append(directs, PathHead)

	prefix := "/" + fmt.Sprintf("%x", directs)
	return prefix + res.String(), nil
}

// SimpleHashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return leafHash(items[0])
	default:
		k := getSplitPoint(len(items))
		left := SimpleHashFromByteSlices(items[:k])
		right := SimpleHashFromByteSlices(items[k:])
		return innerHash(left, right)
	}
}

// SimpleHashFromMap computes a Merkle tree from sorted map.
// Like calling SimpleHashFromHashers with
// `item = []byte(Hash(key) | Hash(value))`,
// sorted by `item`.
func SimpleHashFromMap(m map[string][]byte) []byte {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}

// getSplitPoint returns the largest power of 2 less than length
func getSplitPoint(length int) int {
	if length < 1 {
		panic("Trying to split a tree with size < 1")
	}
	uLength := uint(length)
	bitlen := bits.Len(uLength)
	k := 1 << uint(bitlen-1)
	if k == length {
		k >>= 1
	}
	return k
}
