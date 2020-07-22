package checkdb

import (
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

func Save(Key []byte, Value *identypes.CrossMessages) {
	res, _ := json.Marshal(Value) //对值进行解析
	db.Set(calcAddTxMetaKey(Key), res)

}
func Search(Key []byte) *identypes.CrossMessages {
	txArgs := new(identypes.CrossMessages)

	result := db.Get(calcAddTxMetaKey(Key))
	err := json.Unmarshal(result, txArgs)
	if err != nil {
		return nil
	}
	return txArgs

}

func calcAddTxMetaKey(id []byte) []byte {
	//将key变成height+id
	return []byte(fmt.Sprintf("ID:%v", id))
}
