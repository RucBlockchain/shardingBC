package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type TX struct {
	Txtype   string
	Sender   string
	Receiver string
	ID       [sha256.Size]byte
	Content  []string
}
type Request struct {
	Params json.RawMessage
	Sender string
}

var tx [4][]TX
var tx_package []TX
var tx2 TX

func main() {

	for i := 0; i < 10; i++ {
		a := TX{
			Txtype:   "relaytx",
			Receiver: "a",
			Sender:   string(i),
		}
		tx[0] = append(tx[0], a)
	}

	tx_package = tx[0]
	res, _ := json.Marshal(tx_package)
	rawParamsJSON := json.RawMessage(res)
	//第一层打包结束
	b := &Request{
		Sender: "a",
		Params: rawParamsJSON,
	}

	res1, _ := json.Marshal(b)
	rawParamsJSON1 := json.RawMessage(res1)
	//第二层打包
	fmt.Println("打包过程到此结束!!!!!!!!")
	//打包过程到此结束
	//开始解包
	//第一层解包
	var res2 Request
	json.Unmarshal(rawParamsJSON1, &res2)
	//第二层解包
	var m []TX
	json.Unmarshal(res2.Params, &m)

	for i := range m {
		a := m[i]
		fmt.Println(a.Receiver)
	}
	time.Sleep(time.Second * 5)
}
