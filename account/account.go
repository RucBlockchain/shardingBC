/*
 * @Author: zyj
 * @Desc: 账户模型
 * @Date: 19.11.10
 */

package account

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"strconv"
	"strings"
	"time"
)

/*
 * 交易数据结构
 */
type AccountLog struct {
	ID	    [sha256.Size]byte
	TxType  string // 交易类型
	From    string // 支出方
	To      string // 接收方
	Time    string//发送时间
	Amount  int    // 金额
	Operate int    // 支出方: 0,  接收方: 1
}

// 接受到的交易请求，仅供测试使用
type TxArg struct {
	ID	    [sha256.Size]byte`json:"id"`
	TxType      string `json:"txType"`
	Sender      string `json:"sender"`
	Receiver    string `json:"receiver"`
	Content     string `json:"content"`
	TxSignature string `json:"txSignature"`
	Operate     int    `json:"operate"`
}
func TimePhase(phase int,tx_id [sha256.Size]byte,time string)string{
	return fmt.Sprintf("[tx_phase%d] tx_id:%X time:%s",phase,tx_id,time)
}
// 实例化交易
func NewAccountLog(tx []byte) *AccountLog {
	return _parseTx(tx)
}

/*
 * 成员函数
 */

// 校验交易
func (accountLog *AccountLog) Check() bool {
	from := accountLog.From
	to := accountLog.To
	amount := accountLog.Amount
	t := accountLog.Time
	if accountLog.TxType == "checkpoint" || accountLog.TxType == "addtx" {

		return true
	}
	if amount <= 0 {
		// 关闭查雨捷的交易合法性验证
		//return false
		return true
	}
	if accountLog.TxType == "relaytx" && accountLog.Operate == 1 {
		//relay_out阶段
		logger.Info(TimePhase(4,accountLog.ID,t),strconv.FormatInt(time.Now().UnixNano(), 10))//第四阶段打印
		return true
	}
	logger.Info(TimePhase(1,accountLog.ID,t))//第一阶段打印
	logger.Info(TimePhase(2,accountLog.ID,strconv.FormatInt(time.Now().UnixNano(), 10))) //第二阶段打印
	balanceToStr := _getState([]byte(to))
	balanceFromStr := _getState([]byte(from))

	if len(from) != 0 && balanceFromStr == nil && accountLog.TxType!="init"{
		//fmt.Println("支出方账户不存在")
		// logger.Error("支出方账户不存在")
		// 关闭查雨捷的交易合法性验证
		//return false
		return true
	}
	if len(from) != 0 && balanceToStr == nil && !(accountLog.TxType == "relaytx" && accountLog.Operate == 0)  {
		//fmt.Println("接收方账户不存在")
		//logger.Error("接收方账户不存在")
		// 关闭查雨捷的交易合法性验证
		//return false
		return true
	}

	if len(from) != 0 {
		balanceFrom := _byte2digit(balanceFromStr)
		if balanceFrom < amount {
			//fmt.Println(balanceFrom)
			//logger.Error("余额不足")
			// 关闭查雨捷的交易合法性验证
			//return false
			return true
		}
	}
	// logger.Error("交易通过验证：" + from + " -> " + to + "  " + strconv.Itoa(amount))
	return true
}

// 更新状态
func (accountLog *AccountLog) Save() {
	if accountLog.TxType == "checkpoint" || accountLog.TxType == "addtx" {
		return
	}
	// 支出
	if accountLog.TxType != "relaytx" || accountLog.Operate == 0 {
		if len(accountLog.From) != 0 {
			balanceFrom := _byte2digit(_getState([]byte(accountLog.From)))
			balanceFrom -= accountLog.Amount
			_setState([]byte(accountLog.From), _digit2byte(balanceFrom))
		}
	}
	// 收入
	var balanceTo int
	if accountLog.TxType != "relaytx" || accountLog.Operate == 1 {
		if len(accountLog.From) != 0 {
			balanceTo = _byte2digit(_getState([]byte(accountLog.To)))
			balanceTo += accountLog.Amount
		} else {
			balanceTo = accountLog.Amount
		}
		_setState([]byte(accountLog.To), _digit2byte(balanceTo))
	}
	// logger.Error("交易完成：" +  accountLog.From + " -> " + accountLog.To + "  " + strconv.Itoa(accountLog.Amount))
}

/*
 * 静态函数和私有函数
 */

// 全局对象
var db dbm.DB
var logger log.Logger

// 获取db和logger句柄
func InitAccountDB(db1 dbm.DB, logger1 log.Logger) {
	db = db1
	logger = logger1
}

// 为单元测试提供的初始化
func InitDBForTest(tdb dbm.DB, tLog log.Logger) {
	db = tdb
	logger = tLog
}

// 查询状态
func _getState(key []byte) []byte {
	return db.Get(key)
}

// 更新状态
func _setState(key []byte, val []byte) {
	//blockExec.db.SetSync(key, val);
	db.Set(key, val)
}

// 解析交易
func _parseTx(tx []byte) *AccountLog {
	accountLog := new(AccountLog)

	txArgs := new(TxArg)
	err := json.Unmarshal(tx, txArgs)
	// logger.Error("交易内容: " + string(tx))
	if err != nil {
		//fmt.Println("交易解析失败")
		// logger.Error("交易解析失败")
		return nil
	}
		if txArgs.TxType == "addtx" || txArgs.TxType == "checkpoint" {
		accountLog.TxType = txArgs.TxType
		return accountLog
	}
	args := strings.Split(string(txArgs.Content), "_")
	// fmt.Println(args)
	if len(args) != 4 { //因为添加时间戳参数，所以个数应该是4个
		// logger.Error("参数个数错误")
		return nil
	}

	amount, err := strconv.Atoi(args[2])
	if err != nil {
		//fmt.Println("解析失败,金额应为整数")
		// logger.Error("解析失败，金额应为整数")
		return nil
	}
	accountLog.From = args[0]
	accountLog.To = args[1]
	accountLog.Amount = amount
	accountLog.Time = args[3]//时间戳
	accountLog.ID = txArgs.ID
	accountLog.Operate = txArgs.Operate
	accountLog.TxType = txArgs.TxType
	return accountLog
}

func _parseTx2(tx []byte) *AccountLog {
	args := strings.Split(string(tx), "&")
	if len(args) != 4 {
		// logger.Error("参数个数错误")
		return nil
	}
	froms := strings.Split(args[0], "=")
	tos := strings.Split(args[1], "=")
	amounts := strings.Split(args[2], "=")

	if froms[0] != "from" || tos[0] != "to" || amounts[0] != "amounts" {
		// logger.Error("参数名称错误")
		return nil
	}

	amount, err := strconv.Atoi(amounts[1])
	if err != nil {
		// logger.Error("解析失败，金额应为整数")
		return nil
	}
	accountLog := new(AccountLog)
	accountLog.From = froms[1]
	accountLog.To = tos[1]
	accountLog.Amount = amount

	return accountLog
}

func _parseTx3(tx []byte) *AccountLog {
	var args []string
	err := json.Unmarshal(tx, args)
	if err != nil {
		// logger.Error("交易格式错误")
		return nil
	}
	if len(args) != 4 {
		// logger.Error("参数个数错误")
		return nil
	}

	amount, err := strconv.Atoi(args[2])
	if err != nil {
		// logger.Error("解析失败，金额应为整数")
		return nil
	}
	accountLog := new(AccountLog)
	accountLog.From = args[0]
	accountLog.To = args[1]
	accountLog.Amount = amount

	return accountLog
}

// 字节数组和数字转换
func _byte2digit(digitByte []byte) int {
	digit, _ := strconv.Atoi(string(digitByte))
	return digit
}

func _digit2byte(num int) []byte {
	return []byte(strconv.Itoa(num))
}
