package identypes

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"syscall"
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
	SrcIndex        string

	// 压缩相关的字段
	// CompressedData 和 Txlist两者在任意时刻有且只有一个会存放有数据
	CompressedData []byte // txlist压缩后的数据存放在这里
	IsCompressed   bool   //
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
	DesZone         string
	SrcIndex        string
}

func GetIndex() string {
	g, _ := syscall.Getenv("TASKINDEX")
	return g
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

func (cm *CrossMessages) Compress() bool {
	if cm.IsCompressed {
		return true
	}
	cm.IsCompressed = true

	origin_data, err := json.Marshal(cm.Txlist)
	if err != nil {
		identypesLogger.Error("compress crossmessage failed, err: " + err.Error())
		cm.IsCompressed = false
		return false
	}

	var buf bytes.Buffer
	tmp := make([]byte, len(origin_data))
	copy(tmp, origin_data)
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(tmp)
	if err != nil {
		identypesLogger.Error("compress crossmessage failed, err: " + err.Error())
		cm.IsCompressed = false
		return false
	}

	zw.Flush()
	cm.CompressedData = buf.Bytes()
	cm.Txlist = nil
	zw.Close()
	return true
}

func (cm *CrossMessages) Decompression() bool {
	if cm.IsCompressed == false {
		return true
	}
	var err error
	cm.IsCompressed = false

	tmp := make([]byte, len(cm.CompressedData))
	copy(tmp, cm.CompressedData)
	buf := bytes.NewReader(tmp)

	zr, err := gzip.NewReader(buf)
	if err != nil {
		identypesLogger.Error("New gzip reader failed, err: " + err.Error())
		cm.IsCompressed = true
		return false
	}

	data_de, err := ioutil.ReadAll(zr)
	if err != nil && err != io.ErrUnexpectedEOF {
		identypesLogger.Error("read compressed data failed, err: " + err.Error())
		cm.IsCompressed = true
		return false
	}

	err = json.Unmarshal(data_de, &cm.Txlist)
	if err != nil {
		identypesLogger.Error("Unmarshal data failed, err: " + err.Error())
		cm.IsCompressed = true
		return false
	}
	cm.CompressedData = nil
	return true
}

func NewCrossMessage(txs [][]byte,
	signature []byte,
	pubkey []byte,
	crossMerkleRoot []byte,
	treepath string,
	srcZone, DesZone string,
	Height int64) *CrossMessages {
	cm := &CrossMessages{
		Txlist:          txs,
		Sig:             make([]byte, len(signature)),
		Pubkeys:         make([]byte, len(pubkey)),
		CrossMerkleRoot: make([]byte, len(crossMerkleRoot)),
		TreePath:        treepath,
		SrcZone:         srcZone,
		DesZone:         DesZone,
		Height:          Height,
		Packages:        make([]Package, 0, 10),
		SrcIndex:        GetIndex(),
		ConfirmPackSigs: nil,
		CompressedData:  nil,
		IsCompressed:    false,
	}
	copy(cm.Sig, signature)
	copy(cm.Pubkeys, pubkey)
	copy(cm.CrossMerkleRoot, crossMerkleRoot)

	return cm
}

func CrossMessageFromByteSlices(msg []byte) (*CrossMessages, error) {
	cm := new(CrossMessages)
	if err := json.Unmarshal(msg, cm); err == nil {
		return cm, nil
	} else {
		return nil, errors.New("CrossMessageFromByteSlices failed. err: " + err.Error())
	}
}
