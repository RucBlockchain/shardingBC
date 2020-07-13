package identypes

type CrossMessages struct{
	Txlist []*TX
	Sig []byte//转出方所有节点对该交易包的merkle_root进行门限签名
	Pubkeys []byte //只有一把公钥
	CrossMerkleRoot []byte//merkle root
	TreePath [][]byte//从该包的txlist生成的hash值到root的路径
	SrcZone string//发送方
	DesZone string//接收方
	Height int64//标志时刻
	Packages []Package//应该回复删除什么包
	ConfirmPackSigs []byte//对于这些包的签名
}
type PartSig struct {

	PeerCrossSig []byte
	Id          int64
}
type Package struct {
	CrossMerkleRoot []byte
	Height int64
	CmID   [32]byte
}

func (cm *CrossMessages)CheckMessages()bool{
	return true
}
