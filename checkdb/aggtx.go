package checkdb

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/identypes"
)

var db dbm.DB
var logger log.Logger

func InitAddDB(db1 dbm.DB, logger1 log.Logger) {
	db = db1
	logger = logger1
}

type TX struct {
	ID          [sha256.Size]byte
	TxSignature string
	Content     string
	Sender      string
}

func Save(Key [sha256.Size]byte, Value *identypes.TX) {
	res, _ := json.Marshal(Value) //对值进行解析
	db.Set(calcAddTxMetaKey(Height, Key), res)
}
func Search(Key [sha256.Size]byte) *identypes.TX {
	txArgs := new(TX)

	result := db.Get(calcAddTxMetaKey(Key))
	err := json.Unmarshal(result, txArgs)
	if err != nil {
		logger.Error("获取交易失败")
	}
	return txArgs

}
func calcAddTxMetaKey(height int64, id [sha256.Size]byte) []byte {
	//将key变成height+id
	return []byte(fmt.Sprintf("ID:%v",, id))
}
