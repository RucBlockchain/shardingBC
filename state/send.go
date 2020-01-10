package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	myclient "github.com/tendermint/tendermint/client"
	tp "github.com/tendermint/tendermint/identypes"
	"strconv"
	"strings"
	"syscall"
)

func conver2cptx(cpTxs []tp.TX, height int64) tp.TX {

	var content []string
	var contentByte []byte
	fmt.Println("cpTxs length is ", len(cpTxs))
	for i := 0; i < len(cpTxs); i++ {
		marshalTx, _ := json.Marshal(cpTxs[i])
		contentByte = append(contentByte, marshalTx...)
		content = append(content, string(marshalTx))
	}
	cptx := &tp.TX{
		Txtype:   "checkpoint",
		Sender:   strconv.FormatInt(height, 10), //用sender记录高度
		Receiver: "",
		ID:       sha256.Sum256(contentByte),
		Content:  strings.Join(content, ";;")}
	return *cptx
}
func getShard() string {
	v, _ := syscall.Getenv("TASKID")
	return v
}

func Sendcptx(tx tp.TX, flag int) {
	name := "TT" + getShard() + "Node2:26657"
	tx_package := []tp.TX{}
	tx_package = append(tx_package, tx)
	client := *myclient.NewHTTP(name, "/websocket")
	go client.BroadcastTxAsync(tx_package)

}
