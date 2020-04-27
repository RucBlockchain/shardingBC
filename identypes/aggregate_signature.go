package identypes

import "crypto/sha256"

// 聚合签名结构定义
type AggregateSig struct {
	Signature []byte // sign

	// 潜在的问题：传递的公钥中是否包含协议信息，毕竟验证方需要将byte构建为PubKey；
	// 目前实现可以约定某种加密协议，也只能是bls。。
	Participants [][]byte // 参与聚合签名生成的验证者的公钥
}

// 定义投票过程中传递的签名
type VoteCrossTxSig struct {
	TxId       [sha256.Size]byte `json:"tx_id"` // 这里使用的是identyps.TX.ID的值，是否唯一不确定
	CrossTxSig []byte            `json:"cross_tx_sig"`
}
