package identypes
type CrossMessages struct{
	Txlist []TX
	Sig []byte//转出方所有节点对该交易包的merkle_root进行门限签名
	Pubkeys []byte //只有一把公钥
	CrossMerkleRoot []byte//merkle root
	TreePath [][]byte//从该包的txlist生成的hash值到root的路径
	SrcZone string//发送方
	DesZone string//接收方
	Height int//标志时刻
}
type PartSig struct {

PeerCrossSig []byte
Id          int64
}