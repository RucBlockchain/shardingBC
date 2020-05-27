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
	logger.Error("保存成功")
	res, _ := json.Marshal(Value) //对值进行解析
	db.Set(calcAddTxMetaKey(Key), res)

}
func Search(Key [sha256.Size]byte) *identypes.TX {
	txArgs := new(identypes.TX)

	result := db.Get(calcAddTxMetaKey(Key))
	err := json.Unmarshal(result, txArgs)
	if err != nil {
		logger.Error("获取交易失败")
		return nil
	}
	return txArgs

}
func calcAddTxMetaKey(id [sha256.Size]byte) []byte {
	//将key变成height+id
	return []byte(fmt.Sprintf("ID:%v", id))
}
