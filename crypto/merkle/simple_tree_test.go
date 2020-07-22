package merkle

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/types/time"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	. "github.com/tendermint/tendermint/libs/test"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestSimpleProof(t *testing.T) {
	total := 100

	items := make([][]byte, total)
	for i := 0; i < total; i++ {
		items[i] = testItem(cmn.RandBytes(tmhash.Size))
	}

	rootHash := SimpleHashFromByteSlices(items)

	rootHash2, proofs := SimpleProofsFromByteSlices(items)

	require.Equal(t, rootHash, rootHash2, "Unmatched root hashes: %X vs %X", rootHash, rootHash2)

	// For each item, check the trail.
	for i, item := range items {
		proof := proofs[i]

		// Check total/index
		require.Equal(t, proof.Index, i, "Unmatched indicies: %d vs %d", proof.Index, i)

		require.Equal(t, proof.Total, total, "Unmatched totals: %d vs %d", proof.Total, total)

		// Verify success
		err := proof.Verify(rootHash, item)
		require.NoError(t, err, "Verification failed: %v.", err)

		// Trail too long should make it fail
		origAunts := proof.Aunts
		proof.Aunts = append(proof.Aunts, cmn.RandBytes(32))
		err = proof.Verify(rootHash, item)
		require.Error(t, err, "Expected verification to fail for wrong trail length")

		proof.Aunts = origAunts

		// Trail too short should make it fail
		proof.Aunts = proof.Aunts[0 : len(proof.Aunts)-1]
		err = proof.Verify(rootHash, item)
		require.Error(t, err, "Expected verification to fail for wrong trail length")

		proof.Aunts = origAunts

		// Mutating the itemHash should make it fail.
		err = proof.Verify(rootHash, MutateByteSlice(item))
		require.Error(t, err, "Expected verification to fail for mutated leaf hash")

		// Mutating the rootHash should make it fail.
		err = proof.Verify(MutateByteSlice(rootHash), item)
		require.Error(t, err, "Expected verification to fail for mutated root hash")
	}
}

func Test_getSplitPoint(t *testing.T) {
	tests := []struct {
		length int
		want   int
	}{
		{1, 0},
		{2, 1},
		{3, 2},
		{4, 2},
		{5, 4},
		{6, 4},
		{9, 8},
		{10, 8},
		{20, 16},
		{100, 64},
		{255, 128},
		{256, 128},
		{257, 256},
	}
	for _, tt := range tests {
		got := getSplitPoint(tt.length)
		require.Equal(t, tt.want, got, "getSplitPoint(%d) = %v, want %v", tt.length, got, tt.want)
	}
}

func TestSimpleTree(t *testing.T) {
	total := 5

	items := make([][]byte, total)
	for i := 0; i < total; i++ {
		items[i] = []byte{uint8(i)} //testItem(cmn.RandBytes(tmhash.Size))
	}

	smt := SimpleTreeFromByteSlices(items)
	for i, leaf := range (smt.Leaves) {
		assert.Equal(t, leaf.Hash, leafHash(items[i]))
	}
	for _, leaf := range (smt.Leaves) {
		node := leaf

		for node != smt.Root {
			fmt.Print(" ", fmt.Sprintf("%X", node.Hash))
			node = node.Parent
		}
		fmt.Println()
	}

	pathByindex, _ := smt.GetPathByIndex(3)
	pathByTx, _ := smt.GetPathByValue(items[2])
	assert.True(t, pathByTx == pathByindex, "pathByHash != pathByindex")
	assert.True(t, VerifyTreePath(items[2], pathByindex, smt.ComputeRootHash()))

}

func TestVerifyTreePath(t *testing.T) {
	total := 103

	items := make([][]byte, total)
	for i := 0; i < total; i++ {
		items[i] = []byte{uint8(i)} //testItem(cmn.RandBytes(tmhash.Size))
	}

	smt := SimpleTreeFromByteSlices(items)
	maxLen, avgLen := 0, 0.0
	start := time.Now()
	for i := 0; i < total; i++ {
		path, _ := smt.GetPathByValue(items[i])
		assert.True(t,
			VerifyTreePath(items[i], path, smt.ComputeRootHash()),
			fmt.Sprintf("第%v个交易检验失败", i+1))
		maxLen = max(maxLen, len(path))
		avgLen = (avgLen*float64(i) + float64(len(path))) / float64(i+1)
	}
	t.Logf("%v个交易merkle验证耗时:%v", total, time.Now().Sub(start).Seconds())
	t.Logf("%v个交易merkle tree path最大值为: %v, 平均值为: %v", total, maxLen, avgLen)
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
