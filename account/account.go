/*
 * @Author: zyj
 * @Desc: 账户模型
 * @Date: 19.11.10
 */

package account

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/types"
	"math/big"
	"os"
	"strconv"
	"strings"

	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
)

func PrintLog(ID [sha256.Size]byte) bool {
	OXstring := fmt.Sprintf("%X", ID)
	BigInt, err := new(big.Int).SetString(OXstring, 16)
	if !err {
		fmt.Println("生成大整数错误")
	}
	shd := big.NewInt(int64(500)) //取100模运算
	mod := new(big.Int)
	_, mod = BigInt.DivMod(BigInt, shd, mod)
	if mod.String() == "0" {
		return true
	} else {
		return false
	}
}

/*
 * 交易数据结构
 */
type AccountLog struct {
	ID      [sha256.Size]byte
	TxType  string // 交易类型
	From    string // 支出方
	To      string // 接收方
	Time    string //发送时间
	Amount  int    // 金额
	Operate int    // 支出方: 0,  接收方: 1
	Logtype int    //日志打印级别
}

// 接受到的交易请求，仅供测试使用
type TxArg struct {
	ID          [sha256.Size]byte `json:"id"`
	TxType      string            `json:"txType"`
	Sender      string            `json:"sender"`
	Receiver    string            `json:"receiver"`
	Content     string            `json:"content"`
	TxSignature string            `json:"txSignature"`
	Operate     int               `json:"operate"`
}

func TimePhase(phase string, tx_id [sha256.Size]byte, time string) string {
	return fmt.Sprintf("[tx_phase] index:%s id:%X time:%s\n", phase, tx_id, time)
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
		//if PrintLog(accountLog.ID){
		//	logger.Info(TimePhase(4,accountLog.ID,t),strconv.FormatInt(time.Now().UnixNano(), 10))//第四阶段打印
		//}

		return true
	}
	if PrintLog(accountLog.ID) && (accountLog.Logtype == 1 || accountLog.Logtype == 2) {
		logger.Info(TimePhase("tPoposeTx", accountLog.ID, t)) //第一阶段打印
	}
	//if PrintLog(accountLog.ID){
	//	logger.Info(TimePhase(2,accountLog.ID,strconv.FormatInt(time.Now().UnixNano(), 10))) //第二阶段打印
	//}
	// logger.Info(TimePhase(1,accountLog.ID,t))//第一阶段打印
	// logger.Info(TimePhase(2,accountLog.ID,strconv.FormatInt(time.Now().UnixNano(), 10))) //第二阶段打印
	balanceToStr := _getState([]byte(to))
	balanceFromStr := _getState([]byte(from))

	if len(from) != 0 && balanceFromStr == nil && accountLog.TxType != "init" {
		//fmt.Println("支出方账户不存在")
		// logger.Error("支出方账户不存在")
		// 关闭查雨捷的交易合法性验证
		//return false
		return true
	}
	if len(from) != 0 && balanceToStr == nil && !(accountLog.TxType == "relaytx" && accountLog.Operate == 0) {
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
	//logger.Error("交易完成：")
	//logger.Error("交易完成：" + accountLog.From + " -> " + accountLog.To + "  " + strconv.Itoa(accountLog.Amount))
	//balanceA := _getState([]byte(accountLog.From))
	//balanceB := _getState([]byte(accountLog.To))
	//logger.Error(accountLog.From + "账户余额: " + string(balanceA) + ", " + accountLog.To + "账户余额: " + string(balanceB))
	//statesByte, _ := json.Marshal(GetAllStates())
	//logger.Error(fmt.Sprintf("状态集合为: %v", string(statesByte)))

	// 快照增量缓存
	SetSnapshotCache(accountLog.From, accountLog.Amount, false)
	SetSnapshotCache(accountLog.To, accountLog.Amount, true)

	//cacheByte, _ := json.Marshal(snapshotCache)
	//logger.Error(fmt.Sprintf("快照缓存集合为: %v", string(cacheByte)))
	// logger.Error("交易完成：" +  accountLog.From + " -> " + accountLog.To + "  " + strconv.Itoa(accountLog.Amount))
}

// 快照数据结构
type Snapshot struct {
	Version int64             // 版本
	Content map[string]string // 内容
	ValSet  *types.ValidatorSet // 当前快照的val集合
	NextValSet *types.ValidatorSet
}

/*
 * 静态函数和私有函数
 */

// 全局对象
var db dbm.DB
var logger log.Logger

// 最新快照
var snapshot Snapshot

// 快照缓存
var snapshotCache = make(map[string]int)

var SNAPSHOT_INTERVAL = 10

var IsNewPeer = true

var SnapshotHash string

var SnapshotVersion = ""

var SnapshotOpen = false
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
		fmt.Println("unmarshall交易解析失败")
		// logger.Error("交易解析失败")
		return nil
	}
	if txArgs.TxType == "addtx" || txArgs.TxType == "checkpoint" || txArgs.TxType == "DeliverTx" || txArgs.TxType == "configtx" {
		accountLog.TxType = txArgs.TxType
		return accountLog
	}
	args := strings.Split(string(txArgs.Content), "_")
	// fmt.Println(args)
	if len(args) != 4 { //因为添加时间戳参数，所以个数应该是4个
		fmt.Println("参数个数错误")
		return nil
	}

	amount, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Println("解析失败,金额应为整数")
		// logger.Error("解析失败，金额应为整数")
		return nil
	}
	accountLog.From = args[0]
	accountLog.To = args[1]
	accountLog.Amount = amount
	accountLog.Time = args[3] //时间戳
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
func GenerateValidator(set types.ValidatorSet){

}
// 生成快照 v1.0版本
func GenerateSnapshot(version int64,set *types.ValidatorSet,nextset *types.ValidatorSet) {
	newSnapshot := Snapshot{}
	newSnapshot.Version = version
	// 快照内容，仅供测试
	newSnapshot.Content = GetAllStates()
	snapshot = newSnapshot
	snapshot.ValSet = set.Copy()
	snapshot.NextValSet = nextset.Copy()
}

type Vals struct {

	Validators [][]byte //vals的公钥集合
	Proposer []byte// pro的公钥

}
//生成Validator集合每一个公钥byte
func GeneratePubkey(Validators []*types.Validator,Proposer *types.Validator) Vals{
	var vals Vals
	for i:=0;i<len(Validators);i++{
		vals.Validators = append(vals.Validators, Validators[i].PubKey.Bytes())//得到每个vals的byte
	}
	vals.Proposer = Proposer.PubKey.Bytes()
	return vals
}
//反序列化返回每个vals的集合
func ParsePubSet(vals Vals)([]crypto.PubKey,crypto.PubKey){
	var Validators []crypto.PubKey
	for i:=0;i<len(vals.Validators);i++{
		newpub,err:=bls.GetPubkeyFromByte2(vals.Validators[i])
		Validators = append(Validators, newpub)
		if err!=nil{
			logger.Error("反序列化失败")
		}
	}
	var Proposer crypto.PubKey
	newpub,err:=bls.GetPubkeyFromByte2(vals.Proposer)
	if err!=nil{
		logger.Error("Proposer反序列化失败")
	}
	Proposer =newpub
	return Validators,Proposer
}
func TogetherParseSet(set []byte,pub []byte)*types.ValidatorSet{
	var myVal *types.ValidatorSet
	json.Unmarshal(set,&myVal)
	var vals Vals
	json.Unmarshal(pub,&vals)
	v1,v2:=ParsePubSet(vals)
	for i:=0;i<len(myVal.Validators);i++{
		myVal.Validators[i].PubKey = v1[i]
	}
	myVal.Proposer.PubKey = v2
	return myVal
}
// 生成快照 v2.0版本
func GenerateSnapshotFast(version int64,set *types.ValidatorSet,nextset *types.ValidatorSet) {

	// 如果当前快照不存在，则初始化
	if snapshot.Content == nil {
		snapshot.Content = make(map[string]string)
	}

	tempSnapshotCache := snapshotCache
	// 清空缓存
	snapshotCache = make(map[string]int)
	// 当前快照内容

	// 增量合并快照
	for k, v := range tempSnapshotCache {
		oldVal, _ := strconv.Atoi(snapshot.Content[k])
		snapshot.Content[k] = strconv.Itoa(oldVal + v)
	}
	snapshot.Version = version
	snapshot.ValSet = set.Copy()
	snapshot.NextValSet = nextset.Copy()

	//snapshotByte, _ := json.Marshal(snapshot)
	//logger.Error(fmt.Sprintf("快照生成: %v", string(snapshotByte)))
}

// 生成快照 v3.0版本
func GenerateSnapshotWithSecurity(version int64,set *types.ValidatorSet,nextset *types.ValidatorSet) {
	// 如果当前快照不存在，则初始化
	if snapshot.Content == nil {
		snapshot.Content = make(map[string]string)
	}
	tempSnapshotCache := snapshotCache
	// 清空缓存
	snapshotCache = make(map[string]int)
	// 当前快照内容

	// 增量合并快照
	for k, v := range tempSnapshotCache {
		oldVal, _ := strconv.Atoi(snapshot.Content[k])
		snapshot.Content[k] = strconv.Itoa(oldVal + v)
	}
	snapshot.Version = version
	snapshot.ValSet = set.Copy()
	snapshot.NextValSet = nextset.Copy()
	snapshotByte, _ := json.Marshal(snapshot)
	//logger.Error(fmt.Sprintf("快照生成: %v", string(snapshotByte)))

	// TODO: 增加签名
	hash := DoHash(string(snapshotByte))
	//logger.Info("快照hash:", "hash", hash)
	SnapshotHash = hash

}

// 获取所有状态集合
func GetAllStates() map[string]string {
	//n := 10000
	kvMaps := make(map[string]string)
	iter := db.Iterator([]byte("0"), []byte("z"))
	for iter.Valid() {
		key := string(iter.Key())
		val := string(iter.Value())
		kvMaps[key] = val
		//fmt.Println(iter.Valid())
		iter.Next()
	}
	// 测试使用，作为快照集合
	//for i := 0; i < n; i++ {
	//    address := _geneateRandomStr(32)
	//    t := md5.Sum([]byte(address))
	//    md5str := fmt.Sprintf("%x", t)
	//    //amout := rand.Intn(100)
	//    kvMaps[address] = md5str
	//    SetState([]byte(md5str), []byte(md5str))
	//}
	return kvMaps
}

// 获取快照
func GetSnapshot() Snapshot {
	return snapshot
}

// 更新快照
func SetSnapshot(newSnapshot Snapshot) {
	snapshot = newSnapshot
}

// 更新状态
func SetState(key []byte, val []byte) {
	if db != nil {
		//blockExec.db.SetSync(key, val);
		db.Set(key, val)
	}
}

// Hash算法 sha256: 生成的16进制字符串长度为32，大小为256bit
func DoHash(text string) string {
	hash := sha256.New()
	hash.Write([]byte(text))
	res := hash.Sum(nil)
	return hex.EncodeToString(res)
}

// 签名 ECDSA
func Sign(text string, priKeyPath string) ([]byte, []byte) {
	//1，打开私钥文件，读出内容
	file, err := os.Open(priKeyPath)
	if err != nil {
		panic(err)
	}
	info, err := file.Stat()
	buf := make([]byte, info.Size())
	file.Read(buf)
	//2,pem解密
	block, _ := pem.Decode(buf)
	//x509解密
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//数字签名
	r, s, err := ecdsa.Sign(crand.Reader, privateKey, []byte(text))
	if err != nil {
		panic(err)
	}
	rText, err := r.MarshalText()
	if err != nil {
		panic(err)
	}
	sText, err := s.MarshalText()
	if err != nil {
		panic(err)
	}
	defer file.Close()
	return rText, sText
}

// 验证签名
func Verify(rText []byte, sText []byte, text string, pubKeyPath string) bool {
	file, err := os.Open(pubKeyPath)
	if err != nil {
		panic(err)
	}
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	buf := make([]byte, info.Size())
	file.Read(buf)
	//pem解码
	block, _ := pem.Decode(buf)

	//x509
	publicStream, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//接口转换成公钥
	publicKey := publicStream.(*ecdsa.PublicKey)
	var r, s big.Int
	r.UnmarshalText(rText)
	s.UnmarshalText(sText)
	//认证
	res := ecdsa.Verify(publicKey, []byte(text), &r, &s)
	defer file.Close()
	return res
}

// 生成ECDSA密钥对
func GenerateKey(priKeyPath string, pubKeyPath string) error {
	//使用ecdsa生成密钥对
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), crand.Reader)
	if err != nil {
		return err
	}
	//使用509
	private, err := x509.MarshalECPrivateKey(privateKey) //此处
	if err != nil {
		return err
	}
	//pem
	block := pem.Block{
		Type:  "esdsa private key",
		Bytes: private,
	}
	file, err := os.Create(priKeyPath)
	if err != nil {
		return err
	}
	err = pem.Encode(file, &block)
	if err != nil {
		return err
	}
	file.Close()

	//处理公钥
	public := privateKey.PublicKey

	//x509序列化
	publicKey, err := x509.MarshalPKIXPublicKey(&public)
	if err != nil {
		return err
	}
	//pem
	public_block := pem.Block{
		Type:  "ecdsa public key",
		Bytes: publicKey,
	}
	file, err = os.Create(pubKeyPath)
	if err != nil {
		return err
	}
	//pem编码
	err = pem.Encode(file, &public_block)
	if err != nil {
		return err
	}
	return nil
}

/**
 * 设置快照算法版本
 */
func SetSnapshotVersion(version string) {

	if version == "v1.0" || version == "v2.0" || version == "v3.0" {
		SnapshotVersion = version
	}
}
func SetSnapshotopen(Open bool) {
	SnapshotOpen = Open
	SetSnapshotVersion("v2.0")

}
// 保存增量缓存
func SetSnapshotCache(key string, amount int, incr bool) {
	if key == "" {
		return
	}
	// 当该key不存在时，oldVal = 0
	oldVal := snapshotCache[key]
	if incr {
		snapshotCache[key] = oldVal + amount
	} else {
		snapshotCache[key] = oldVal - amount
	}
}
