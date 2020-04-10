package account

import (
	"fmt"
    dbm "github.com/tendermint/tendermint/libs/db"
    "github.com/tendermint/tendermint/libs/log"
	"testing"
)

func TestNewAccountLog(t *testing.T) {
    InitDBForTest(dbm.NewMemDB(), log.TestingLogger())

    txStr := "{\"TxType\":\"tx\", \"Sender\":\"a\", \"Receiver\":\"b\", \"Content\":\"_b_100\"}"
    tx := []byte(txStr)
    accountLog := NewAccountLog(tx)
    if accountLog == nil {
        t.Error("解析失败")
    } else {
        fmt.Println(accountLog)
    }
}

func TestAccountLog_Check(t *testing.T) {
    db := dbm.NewMemDB()
    InitDBForTest(db, log.TestingLogger())
    txStr := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"b\", \"Content\":\"_c_50\"}"
    accountLog := NewAccountLog([]byte(txStr))
    res := accountLog.Check()
    fmt.Println(res)
}

func TestAccountLog_Save(t *testing.T) {
    db := dbm.NewMemDB()
    InitDBForTest(db, log.TestingLogger())
    txStr := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"b\", \"Content\":\"a_b_100\"}"
    accountLog := NewAccountLog([]byte(txStr))
    accountLog.Save()
    fmt.Println("b的余额为: " + getState("b", db))
}

func TestAccountLog_Check2(t *testing.T) {
    db := dbm.NewMemDB()
    InitDBForTest(db, log.TestingLogger())
    txStr1 := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"b\", \"Content\":\"_b_100\"}"
    accountLog1 := NewAccountLog([]byte(txStr1))
    txStr2 := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"a\", \"Content\":\"_a_50\"}"
    accountLog2 := NewAccountLog([]byte(txStr2))
    accountLog1.Save()
    accountLog2.Save()

    // 转账
    txStr3 := "{\"TxType\":\"tx\", \"Sender\":\"b\", \"Receiver\":\"a\", \"Content\":\"b_a_30\"}"
    accountLog3 := NewAccountLog([]byte(txStr3))
    accountLog3.Check()
}


func TestAccountLog_Save2(t *testing.T) {
    db := dbm.NewMemDB()
    InitDBForTest(db, log.TestingLogger())
    txStr1 := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"b\", \"Content\":\"_a_50\"}"
    accountLog1 := NewAccountLog([]byte(txStr1))
    txStr2 := "{\"TxType\":\"tx\", \"Sender\":\"\", \"Receiver\":\"a\", \"Content\":\"a_c_50\"}"
    accountLog2 := NewAccountLog([]byte(txStr2))
    accountLog1.Save()
    accountLog2.Save()
    fmt.Println("转账前: a的余额为: " + getState("a", db) + "  b的余额为: " + getState("b", db))

    // 转账
    txStr3 := "{\"TxType\":\"tx\", \"Sender\":\"a\", \"Receiver\":\"b\", \"Content\":\"c_a_20\"}"
    accountLog3 := NewAccountLog([]byte(txStr3))
    res := accountLog3.Check()
    if !res {
        t.Error("校验不通过")
    }
    accountLog3.Save()
    fmt.Println("转账后: a的余额为: " + getState("a", db) + "  b的余额为: " + getState("b", db))
}






func getState(account string, db dbm.DB) string {
    return string(db.Get([]byte(account)))
}
