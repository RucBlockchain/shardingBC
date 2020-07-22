package merkle

type TxMerkleTree struct {
	RootTree   *SimpleMerkleTree
	ShardTrees map[string]*SimpleMerkleTree
}

// todo 改变结构，优化查询
func (tmt *TxMerkleTree) Find(shardname string) *SimpleMerkleTree {
	if tmt.ShardTrees == nil {
		return nil
	}
	if tree, ok := tmt.ShardTrees[shardname]; ok {
		return tree
	}

	return nil
}
