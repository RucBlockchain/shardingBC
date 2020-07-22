package identypes

import (
	"encoding/json"
	"errors"
	"fmt"
)

type CrossMessages struct {
	Txlist          [][]byte  // [][]byte
	Sig             []byte    //转出方所有节点对该交易包的merkle_root进行门限签名
	Pubkeys         []byte    //只有一把公钥
	CrossMerkleRoot []byte    //merkle root
	TreePath        string    //从该包的txlist生成的hash值到root的路径
	SrcZone         string    //发送方
	DesZone         string    //接收方
	Height          int64     //标志时刻
	Packages        []Package //应该回复删除什么包
	ConfirmPackSigs []byte    //对于这些包的签名
}

type PartSig struct {
	PeerCrossSig []byte
	Id           int64
}

type Package struct {
	CrossMerkleRoot []byte
	Height          int64
	CmID            [32]byte
	SrcZone         string
}

func ParsePackages(data []byte) []Package {
	var packs []Package
	err := json.Unmarshal(data, &packs)
	if len(packs) == 0 {
		return nil
	}
	if err != nil {
		fmt.Println("ParseData Wrong")
	}
	return packs
}

func (cm *CrossMessages) CheckMessages() bool {
	return true
}

func (cm *CrossMessages) Data() []byte {
	if data, err := json.Marshal(cm); err == nil {
		return data
	}
	return nil
}

func NewCrossMessage(txs [][]byte,
	signature []byte,
	pubkey []byte,
	crossMerkleRoot []byte,
	treepath string,
	srcZone, DesZone string,
	Height int64) *CrossMessages {
	return &CrossMessages{
		Txlist:          txs,
		Sig:             signature,
		Pubkeys:         pubkey,
		CrossMerkleRoot: crossMerkleRoot,
		TreePath:        treepath,
		SrcZone:         srcZone,
		DesZone:         DesZone,
		Height:          Height,
		Packages:        make([]Package, 0, 10),
		ConfirmPackSigs: nil,
	}
}

func CrossMessageFromByteSlices(msg []byte) (*CrossMessages, error) {
	cm := new(CrossMessages)
	if err := json.Unmarshal(msg, cm); err == nil {
		return cm, nil
	} else {
		return nil, errors.New("CrossMessageFromByteSlices failed. err: " + err.Error())
	}
}
