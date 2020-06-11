package checkdb

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/tendermint/tendermint/identypes"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
)

var db dbm.DB
var logger log.Logger

func InitAddDB(db1 dbm.DB, logger1 log.Logger) {
	db = db1
	logger = logger1
}
func Save(Key [sha256.Size]byte, Value *identypes.TX) {
	res, _ := json.Marshal(Value) //对值进行解析
	db.Set(calcAddTxMetaKey(Key), res)

}
func Search(Key [sha256.Size]byte) *identypes.TX {
	txArgs := new(identypes.TX)

	result := db.Get(calcAddTxMetaKey(Key))
	err := json.Unmarshal(result, txArgs)
	if err != nil {
		return nil
	}
	return txArgs

}
func calcAddTxMetaKey(id [sha256.Size]byte) []byte {
	//将key变成height+id
	return []byte(fmt.Sprintf("ID:%v", id))
}
func SaveTestData(Key [sha256.Size]byte, Value *identypes.RelayLive){
	fmt.Println(Value)
	fmt.Println("固化测试数据1")
	res, _ := json.Marshal(Value) //对值进行解析
	db.Set(calcTestDataMetaKey(Key), res)
}
func calcTestDataMetaKey(id [sha256.Size]byte) []byte {
	//将key变成height+id
	return []byte(fmt.Sprintf("Test:%v", id))
}