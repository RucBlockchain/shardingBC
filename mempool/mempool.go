package mempool

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/checkdb"
	myclient "github.com/tendermint/tendermint/client"
	//cm "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto/bls"
	"github.com/tendermint/tendermint/crypto/merkle"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pkg/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/account"

	cfg "github.com/tendermint/tendermint/config"
	tp "github.com/tendermint/tendermint/identypes"
	auto "github.com/tendermint/tendermint/libs/autofile"
	"github.com/tendermint/tendermint/libs/clist"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	"math/big"
	//"github.com/tendermint/tendermint/state"
)

// PreCheckFunc is an optional filter executed before CheckTx and rejects
// transaction if false is returned. An example would be to ensure that a
// transaction doesn't exceeded the block size.
type PreCheckFunc func(types.Tx) error

// PostCheckFunc is an optional filter executed after CheckTx and rejects
// transaction if false is returned. An example would be to ensure a
// transaction doesn't require more gas than available for the block.
type PostCheckFunc func(types.Tx, *abci.ResponseCheckTx) error

// TxInfo are parameters that get passed when attempting to add a tx to the
// mempool.
type TxInfo struct {
	// We don't use p2p.ID here because it's too big. The gain is to store max 2
	// bytes with each tx to identify the sender rather than 20 bytes.
	PeerID uint16
}
type CmInfo struct {
	// We don't use p2p.ID here because it's too big. The gain is to store max 2
	// bytes with each tx to identify the sender rather than 20 bytes.
	PeerID uint16
}

/*

The mempool pushes new txs onto the proxyAppConn.
It gets a stream of (req, res) tuples from the proxy.
The mempool stores good txs in a concurrent linked-list.

Multiple concurrent go-routines can traverse this linked-list
safely by calling .NextWait() on each element.

So we have several go-routines:
1. Consensus calling Update() and Reap() synchronously
2. Many mempool reactor's peer routines calling CheckTx()
3. Many mempool reactor's peer routines traversing the txs linked list
4. Another goroutine calling GarbageCollectTxs() periodically

To manage these goroutines, there are three methods of locking.
1. Mutations to the linked-list is protected by an internal mtx (CList is goroutine-safe)
2. Mutations to the linked-list elements are atomic
3. CheckTx() calls can be paused upon Update() and Reap(), protected by .proxyMtx

Garbage collection of old elements from mempool.txs is handlde via
the DetachPrev() call, which makes old elements not reachable by
peer broadcastTxRoutine() automatically garbage collected.

TODO: Better handle abci client errors. (make it automatically handle connection errors)

*/

var (
	// ErrTxInCache is returned to the client if we saw tx earlier
	ErrTxInCache = errors.New("Tx already exists in cache")

	// ErrTxTooLarge means the tx is too big to be sent in a message to other peers
	ErrTxTooLarge = fmt.Errorf("Tx too large. Max size is %d", maxTxSize)
)

// ErrMempoolIsFull means Tendermint & an application can't handle that much load
type ErrMempoolIsFull struct {
	numTxs int
	maxTxs int

	txsBytes    int64
	maxTxsBytes int64
}

func (e ErrMempoolIsFull) Error() string {
	s := "Mempool is full"
	return s
}

// ErrPreCheck is returned when tx is too big
type ErrPreCheck struct {
	Reason error
}

func (e ErrPreCheck) Error() string {
	return e.Reason.Error()
}

// IsPreCheckError returns true if err is due to pre check failure.
func IsPreCheckError(err error) bool {
	_, ok := err.(ErrPreCheck)
	return ok
}

//取模运算，模n输出。
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
func TimePhase(phase string, tx_id [sha256.Size]byte, time string) string {
	if PrintLog(tx_id) {
		fmt.Printf("[tx_phase] index:%s id:%X time:%s\n", phase, tx_id, time)
	}
	return fmt.Sprintf("[tx_phase%d] tx_id:%X time:%s", phase, tx_id, time)
}

// PreCheckAminoMaxBytes checks that the size of the transaction plus the amino
// overhead is smaller or equal to the expected maxBytes.
func PreCheckAminoMaxBytes(maxBytes int64) PreCheckFunc {
	return func(tx types.Tx) error {
		// We have to account for the amino overhead in the tx size as well
		// NOTE: fieldNum = 1 as types.Block.Data contains Txs []Tx as first field.
		// If this field order ever changes this needs to updated here accordingly.
		// NOTE: if some []Tx are encoded without a parenting struct, the
		// fieldNum is also equal to 1.
		aminoOverhead := types.ComputeAminoOverhead(tx, 1)
		txSize := int64(len(tx)) + aminoOverhead
		if txSize > maxBytes {
			return fmt.Errorf("Tx size (including amino overhead) is too big: %d, max: %d",
				txSize, maxBytes)
		}
		return nil
	}
}

// PostCheckMaxGas checks that the wanted gas is smaller or equal to the passed
// maxGas. Returns nil if maxGas is -1.
func PostCheckMaxGas(maxGas int64) PostCheckFunc {
	return func(tx types.Tx, res *abci.ResponseCheckTx) error {
		if maxGas == -1 {
			return nil
		}
		if res.GasWanted < 0 {
			return fmt.Errorf("gas wanted %d is negative",
				res.GasWanted)
		}
		if res.GasWanted > maxGas {
			return fmt.Errorf("gas wanted %d is greater than max gas %d",
				res.GasWanted, maxGas)
		}
		return nil
	}
}

// TxID is the hex encoded hash of the bytes as a types.Tx.
func TxID(tx []byte) string {
	return fmt.Sprintf("%X", types.Tx(tx).Hash())
}
func Sum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}
func CmID(cm *tp.CrossMessages) string {
	heightHash := Sum([]byte(strconv.FormatInt(cm.Height, 10)))
	ID := append(heightHash, cm.CrossMerkleRoot...)
	return fmt.Sprintf("%X", ID)
}
func CmId(cm []byte) [sha256.Size]byte {
	ID := sha256.Sum256(cm)
	return ID
}

// txKey is the fixed length array sha256 hash used as the key in maps.
func txKey(tx types.Tx) [sha256.Size]byte {
	return sha256.Sum256(tx)
}

// Mempool is an ordered in-memory pool for transactions before they are proposed in a consensus
// round. Transaction validity is checked using the CheckTx abci message before the transaction is
// added to the pool. The Mempool uses a concurrent list structure for storing transactions that
// can be efficiently accessed by multiple concurrent readers.
type Mempool struct {
	config *cfg.MempoolConfig

	proxyMtx     sync.Mutex
	proxyAppConn proxy.AppConnMempool
	txs          *clist.CList // concurrent linked-list of good txs
	Cms          *clist.CList

	preCheck  PreCheckFunc
	postCheck PostCheckFunc
	//rDB          relaytxDB //relaylist
	cmDB   CrossMessagesDB
	RlDB   []RelationTable
	cmChan chan *tp.CrossMessages
	// Track whether we're rechecking txs.
	// These are not protected by a mutex and are expected to be mutated
	// in serial (ie. by abci responses which are called in serial).
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	// notify listeners (ie. consensus) when txs are available
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty

	// Map for quick access to txs to record sender in CheckTx.
	// txsMap: txKey -> cmCElement
	txsMap sync.Map

	cmsMap sync.Map
	// Atomic integers
	height     int64 // the last block Update()'d to
	rechecking int32 // for re-checking filtered txs on Update()
	txsBytes   int64 // total size of mempool, in bytes

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cache txCache

	// A log of mempool txs
	wal *auto.AutoFile

	logger log.Logger

	metrics *Metrics
}

//--------------------------------------------
//新增的跨片交易的状态数据库
type CrossMessagesDB struct {
	CrossMessages []*CrossMessage
}
type CrossMessage struct {
	Content *tp.CrossMessages
	Height  int
}

//type relaytxDB struct {
//	relaytx []RTx //RelayTx结构的切片
//}
//
//type RTx struct {
//	Tx     tp.TX
//	Height int
//	CrossMessageIndex tp.CrossMessageIndex
//}

//-------------------------------------------

// MempoolOption sets an optional parameter on the Mempool.
type MempoolOption func(*Mempool)

// NewMempool returns a new Mempool with the given configuration and connection to an application.
func NewMempool(
	config *cfg.MempoolConfig,
	proxyAppConn proxy.AppConnMempool,
	height int64,
	options ...MempoolOption,
) *Mempool {
	mempool := &Mempool{
		config:        config,
		proxyAppConn:  proxyAppConn,
		txs:           clist.New(),
		height:        height,
		rechecking:    0,
		recheckCursor: nil,
		recheckEnd:    nil,
		logger:        log.NewNopLogger(),
		metrics:       NopMetrics(),
		cmDB:          newcmDB(),
		cmChan:        make(chan *tp.CrossMessages, 1), //开启容量为1的通道
	}
	if config.CacheSize > 0 {
		mempool.cache = newMapTxCache(config.CacheSize)
	} else {
		mempool.cache = nopTxCache{}
	}
	proxyAppConn.SetResponseCallback(mempool.globalCb)
	for _, option := range options {
		option(mempool)
	}
	return mempool
}

//--------------------------------------------------------
//新增函数
func newcmDB() CrossMessagesDB {
	var cmdb CrossMessagesDB
	var cm []*CrossMessage
	cmdb.CrossMessages = cm
	return cmdb
}
func (mem *Mempool) AddCrossMessagesDB(tcm *tp.CrossMessages) {
	cm := &CrossMessage{
		Content: tcm,
		Height:  0,
	}
	//fmt.Println("加入cmdb","root:",cm.Content.CrossMerkleRoot," height",cm.Content.Height,"对方要删除的packages",cm.Content.Packages)
	mem.cmDB.CrossMessages = append(mem.cmDB.CrossMessages, cm)
	//fmt.Println("添加CmDB，容量为",len(mem.cmDB.CrossMessages))
}
func (mem *Mempool) RemoveCrossMessagesDB(tcm *tp.CrossMessages) {
	//删除交易包
	//对package进行解析
	packs := tcm.Packages
	for j := 0; j < len(packs); j++ {
		//fmt.Println("收到要删除的包",packs[j])
		for i := 0; i < len(mem.cmDB.CrossMessages); i++ {
			if mem.cmDB.CrossMessages[i].Content.Height == packs[j].Height && bytes.Equal(mem.cmDB.CrossMessages[i].Content.CrossMerkleRoot, packs[j].CrossMerkleRoot) {
				mem.cmDB.CrossMessages = append(mem.cmDB.CrossMessages[:i], mem.cmDB.CrossMessages[i+1:]...)
				i--

				break
			}
		}
	}
	//fmt.Println("CmDB还剩:",len(mem.cmDB.CrossMessages))

}
func (mem *Mempool) UpdatecmDB() []*tp.CrossMessages {
	//检查cmDB中的状态，如果有一个区块高度是20，还没有被删除，那么需要重新发送交易包，让其被确认

	var scm []*tp.CrossMessages
	for i := 0; i < len(mem.cmDB.CrossMessages); i++ {
		if mem.cmDB.CrossMessages != nil {
			if mem.cmDB.CrossMessages[i].Height == 0 { //第一次高度为0就发布
				scm = append(scm, mem.cmDB.CrossMessages[i].Content)
				mem.cmDB.CrossMessages[i].Height += 1 //增加高度
				//考虑如何进行合理分化
			} else if mem.cmDB.CrossMessages[i].Height == 10 {
				scm = append(scm, mem.cmDB.CrossMessages[i].Content)
				mem.cmDB.CrossMessages[i].Height = 0
			} else {
				mem.cmDB.CrossMessages[i].Height = mem.cmDB.CrossMessages[i].Height + 1
			}
		}
	}
	return scm

}
func (mem *Mempool) GetAllCrossMessages() []*tp.CrossMessages {
	var allcm []*tp.CrossMessages
	for i := 0; i < len(mem.cmDB.CrossMessages); i++ {
		if mem.cmDB.CrossMessages != nil {
			allcm = append(allcm, mem.cmDB.CrossMessages[i].Content)
		}
	}
	return allcm
}

//func newrDB() relaytxDB {
//	var rdb relaytxDB
//	var rtx []RTx
//	rdb.relaytx = rtx
//	return rdb
//}
//
//func (mem *Mempool) AddRelaytxDB(tx tp.TX,CIndex tp.CrossMessageIndex) {
//	var rtx RTx
//	rtx.Tx = tx
//	rtx.Height = 0
//	rtx.CrossMessageIndex = CIndex
//	mem.rDB.relaytx = append(mem.rDB.relaytx, rtx)
//}
//
//func (mem *Mempool) RemoveRelaytxDB(tx tp.TX) {
//	for i := 0; i < len(mem.rDB.relaytx); i++ {
//		if mem.rDB.relaytx[i].Tx.ID == tx.ID {
//			mem.rDB.relaytx = append(mem.rDB.relaytx[:i], mem.rDB.relaytx[i+1:]...)
//			i--
//
//			break
//		}
//	}
//}
//func (mem *Mempool) UpdaterDB() []tp.TX {
//	//检查rDB中的状态，如果有一个区块高度是20，还没有被删除，那么需要重新发送tx，让其被确认
//
//	var stx []tp.TX
//	for i := 0; i < len(mem.rDB.relaytx); i++ {
//		if mem.rDB.relaytx != nil {
//			if mem.rDB.relaytx[i].Height == 0 { //第一次高度为0就发布
//				stx = append(stx, mem.rDB.relaytx[i].Tx)
//				mem.rDB.relaytx[i].Height += 1 //增加高度
//			} else if (mem.rDB.relaytx[i].Height == 10) || (mem.rDB.relaytx[i].Height == 20) {
//				stx = append(stx, mem.rDB.relaytx[i].Tx)
//				mem.rDB.relaytx[i].Height = 0
//			} else {
//				mem.rDB.relaytx[i].Height = mem.rDB.relaytx[i].Height + 1
//			}
//		}
//	}
//	return stx
//
//}
//func (mem *Mempool) GetAllTxs() []tp.TX {
//	var alltx []tp.TX
//	for i := 0; i < len(mem.rDB.relaytx); i++ {
//		if mem.rDB.relaytx != nil {
//			alltx = append(alltx, mem.rDB.relaytx[i].Tx)
//		}
//	}
//	return alltx
//}

//----------------------------------------------------------
// EnableTxsAvailable initializes the TxsAvailable channel,
// ensuring it will trigger once every height when transactions are available.
// NOTE: not thread safe - should only be called once, on startup
func (mem *Mempool) EnableTxsAvailable() {
	mem.txsAvailable = make(chan struct{}, 1)
}

// SetLogger sets the Logger.
func (mem *Mempool) SetLogger(l log.Logger) {
	mem.logger = l
}

// WithPreCheck sets a filter for the mempool to reject a tx if f(tx) returns
// false. This is ran before CheckTx.
func WithPreCheck(f PreCheckFunc) MempoolOption {
	return func(mem *Mempool) { mem.preCheck = f }
}

// WithPostCheck sets a filter for the mempool to reject a tx if f(tx) returns
// false. This is ran after CheckTx.
func WithPostCheck(f PostCheckFunc) MempoolOption {
	return func(mem *Mempool) { mem.postCheck = f }
}

// WithMetrics sets the metrics.
func WithMetrics(metrics *Metrics) MempoolOption {
	return func(mem *Mempool) { mem.metrics = metrics }
}

// InitWAL creates a directory for the WAL file and opens a file itself.
//
// *panics* if can't create directory or open file.
// *not thread safe*
func (mem *Mempool) InitWAL() {
	walDir := mem.config.WalDir()
	err := cmn.EnsureDir(walDir, 0700)
	if err != nil {
		panic(errors.Wrap(err, "Error ensuring Mempool WAL dir"))
	}
	af, err := auto.OpenAutoFile(walDir + "/wal")
	if err != nil {
		panic(errors.Wrap(err, "Error opening Mempool WAL file"))
	}
	mem.wal = af
}

// CloseWAL closes and discards the underlying WAL file.
// Any further writes will not be relayed to disk.
func (mem *Mempool) CloseWAL() {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	if err := mem.wal.Close(); err != nil {
		mem.logger.Error("Error closing WAL", "err", err)
	}
	mem.wal = nil
}

// Lock locks the mempool. The consensus must be able to hold lock to safely update.
func (mem *Mempool) Lock() {
	mem.proxyMtx.Lock()
}

// Unlock unlocks the mempool.
func (mem *Mempool) Unlock() {
	mem.proxyMtx.Unlock()
}

// Size returns the number of transactions in the mempool.
func (mem *Mempool) Size() int {
	return mem.txs.Len()
}

// TxsBytes returns the total size of all txs in the mempool.
func (mem *Mempool) TxsBytes() int64 {
	return atomic.LoadInt64(&mem.txsBytes)
}

// FlushAppConn flushes the mempool connection to ensure async reqResCb calls are
// done. E.g. from CheckTx.
func (mem *Mempool) FlushAppConn() error {
	return mem.proxyAppConn.FlushSync()
}

// Flush removes all transactions from the mempool and cache
func (mem *Mempool) Flush() {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	mem.cache.Reset()

	for e := mem.txs.Front(); e != nil; e = e.Next() {
		mem.txs.Remove(e)
		e.DetachPrev()
	}

	mem.txsMap = sync.Map{}
	_ = atomic.SwapInt64(&mem.txsBytes, 0)
}

// TxsFront returns the first transaction in the ordered list for peer
// goroutines to call .NextWait() on.
func (mem *Mempool) TxsFront() *clist.CElement {
	return mem.txs.Front()
}
func (mem *Mempool) CmsFront() *clist.CElement {
	return mem.Cms.Front()
}

// TxsWaitChan returns a channel to wait on transactions. It will be closed
// once the mempool is not empty (ie. the internal `mem.txs` has at least one
// element)
func (mem *Mempool) TxsWaitChan() <-chan struct{} {
	return mem.txs.WaitChan()
}
func (mem *Mempool) CmsWaitChan() <-chan struct{} {
	return mem.Cms.WaitChan()
}

// CheckTx executes a new transaction against the application to determine its validity
// and whether it should be added to the mempool.
// It blocks if we're waiting on Update() or Reap().
// cb: A callback from the CheckTx command.
//     It gets called from another goroutine.
// CONTRACT: Either cb will get called, or err returned.
func (mem *Mempool) CheckDB(tx types.Tx) string {
	if cm := ParseData(tx); cm != nil {
		if cm.SrcZone == getShard() {
			//收到状态数据库的回执f
			//fmt.Println("收到回执", string(tx))
			// fmt.Println("收到回执并且执行删除",cm.Packages)
			mem.ModifyCrossMessagelist(cm)
			return "回执"
		}
		// fmt.Println("收到", "SrcZone",cm.SrcZone,"DesZone",cm.DesZone,"Height",cm.Height,"root",string(cm.CrossMerkleRoot))

		cmid := CmID(cm)
		dbtx := checkdb.Search([]byte(cmid))
		if dbtx != nil {
			name := dbtx.SrcZone + "S" + cm.SrcIndex + ":26657"
			//	fmt.Println("发送",name)
			// fmt.Println("回执crossmessage"," 对方的height",dbtx.Height," cmroot", "SrcZone",dbtx.SrcZone,"DesZone",dbtx.DesZone,
			//)
			tx_package := []*tp.CrossMessages{}
			tx_package = append(tx_package, dbtx)
			for i := 0; i < len(tx_package); i++ {
				client := *myclient.NewHTTP(name, "/websocket")

				go client.BroadcastCrossMessageAsync(tx_package)
			}
			//fmt.Println("状态数据库返回")
			return "状态数据库返回"
		} else {
			return ""
		}

	}

	return ""
}
func (mem *Mempool) CheckTx(tx types.Tx, cb func(*abci.Response)) (err error) {
	status := mem.CheckDB(tx)
	if status == "回执" {
		//fmt.Println("回执同步")
		return mem.CheckTxWithInfo(tx, cb, TxInfo{PeerID: UnknownPeerID}, true)
	} else if status == "" {
		//fmt.Println("cm处理")
		return mem.CheckTxWithInfo(tx, cb, TxInfo{PeerID: UnknownPeerID}, false)
	} else {
		//fmt.Println("状态数据库返回")
		return errors.New("状态数据库返回")
	}
}

//添加新的函数------------------------------------------------------------------
//为了投票使用
func (mem *Mempool) SearchPackageExist(pack tp.Package) bool {
	_, ok := mem.txsMap.Load(pack.CmID)
	return ok
}
func (mem *Mempool) SyncRelationTable(pack tp.Package, height int64) {
	var rl RelationTable
	rl = RelationTable{
		CsHeight: height,
		CmHeight: pack.Height,
		CmHash:   pack.CrossMerkleRoot,
		CmID:     pack.CmID,
		SrcZone:  pack.SrcZone,
		DesZone:  pack.DesZone,
	}
	rl.RlID = RlID(rl)
	mem.RlDB = append(mem.RlDB, rl)
}

//检查验证Cm的正确性
func (mem *Mempool) ModifyCrossMessagelist(cm *tp.CrossMessages) {
	mem.RemoveCrossMessagesDB(cm)
}
func (mem *Mempool) CheckCrossMessage(cm *tp.CrossMessages) error {
	//TODO:CheckMessages还未完善
	return mem.CheckCrossMessageWithInfo(cm)
}
func (mem *Mempool) CheckCrossMessageWithInfo(cm *tp.CrossMessages) (err error) {
	//由于上层已经锁了，在这里不用锁
	//检验包的正确性
	//mem.logger.Error("检验cm")
	if res := cm.Decompression(); !res {
		return errors.New("decompression failed.")
	}
	defer cm.Compress()
	//mem.logger.Error("解压完成")
	if result := mem.CheckCrossMessageSig(cm); !result {
		//fmt.Println("路径验证失败")
		return errors.New("聚合签名或者检验失败")
	}
	if len(cm.Packages) > 0 {
		//mem.logger.Error("待删除包数量非0，删除对应的包")
		mem.ModifyCrossMessagelist(cm)
	}
	//fmt.Println("聚合签名与解压完成")
	//mem.logger.Error("修改cmdb完成")
	//交易合法性检验
	for i := 0; i < len(cm.Txlist); i++ {
		//fmt.Println("交易合法性验证")
		accountLog := account.NewAccountLog(cm.Txlist[i])
		if accountLog == nil {
			fmt.Println("交易解析失败")
			return errors.New("交易解析失败")
		}
		//fmt.Printf("[tx_phase] index:tCheckCM id:%X time:%s\n", CmID(cm), strconv.FormatInt(time.Now().UnixNano(), 10))
		checkRes := accountLog.Check() //开始打印日志 sha256.Sum256()
		//fmt.Println("交易合法验证通过")
		if !checkRes {
			fmt.Println("不合法的交易")
			return errors.New("不合法的交易")
		}
	}
	//mem.logger.Error("交易合法性检验通过")
	//删除对应的CrossList内容
	//fmt.Println("完成验证")
	return nil
}

type RelationTable struct {
	CsHeight       int64
	CmHeight       int64
	CmHash         []byte
	CmID           [32]byte
	RlID           string
	Packages       []byte
	ComfirmPackSig []byte
	SrcZone        string
	DesZone        string
	SrcIndex       string
}

func (mem *Mempool) AddRelationTable(cm *tp.CrossMessages, height int64, cmID [32]byte) {
	//打包区块的高度Cs.Height|Cm.Height|Cm.Hash|Cm.ID

	rl := RelationTable{
		CsHeight: height,
		CmHeight: cm.Height,
		CmHash:   cm.CrossMerkleRoot,
		CmID:     cmID,
		SrcZone:  cm.SrcZone,
		DesZone:  cm.DesZone,
		SrcIndex: cm.SrcIndex,
	}

	rl.RlID = RlID(rl)
	//fmt.Println("新添加的映射表"," csheight",rl.CsHeight," cmheight",rl.CmHeight," cmhash:",
	//	rl.CmHash," 发送方: ",rl.SrcZone," 接受方:",rl.DesZone," rlid: ",rl.RlID," packages:",tp.ParsePackages(rl.Packages),
	//)
	mem.RlDB = append(mem.RlDB, rl)
}

//todo
func (mem *Mempool) SearchRelationTable(Height int64) []tp.Package {
	var packlist []tp.Package
	for i := 0; i < len(mem.RlDB); i++ {
		if mem.RlDB[i].CsHeight == Height {
			pack := tp.Package{
				CrossMerkleRoot: mem.RlDB[i].CmHash,
				Height:          mem.RlDB[i].CmHeight,
				CmID:            mem.RlDB[i].CmID,
				SrcZone:         mem.RlDB[i].SrcZone,
				DesZone:         mem.RlDB[i].DesZone,
				SrcIndex:        mem.RlDB[i].SrcIndex,
			}
			packlist = append(packlist, pack)
		}
	}
	return packlist
}
func (mem *Mempool) ModifyRelationTable(packages []byte, ConfirmPackSig []byte, Height int64) {
	for i := 0; i < len(mem.RlDB); i++ {
		if mem.RlDB[i].CsHeight == Height {
			mem.RlDB[i].Packages = packages
			mem.RlDB[i].ComfirmPackSig = ConfirmPackSig
			//fmt.Println("修改映射表"," csheight",mem.RlDB[i].CsHeight," cmheight",mem.RlDB[i].CmHeight," cmhash:",
			//	mem.RlDB[i].CmHash," 发送方: ",mem.RlDB[i].SrcZone," 接受方:",mem.RlDB[i].DesZone,"rlid",mem.RlDB[i].RlID,
			//	"packages",tp.ParsePackages(mem.RlDB[i].Packages),
			//)
		}
	}

}

//删除映射表关系
func (mem *Mempool) RemoveRelationTable(rlid string) {
	//fmt.Println("映射表容量", len(mem.RlDB))
	//fmt.Println("本次删除映射表的选项是", rlid)
	for i := 0; i < len(mem.RlDB); i++ {
		if mem.RlDB[i].RlID == rlid {
			//先固化到磁盘
			cm := &tp.CrossMessages{
				Txlist:          nil,
				Sig:             nil,
				Pubkeys:         nil,
				CrossMerkleRoot: mem.RlDB[i].CmHash,
				TreePath:        "",
				SrcZone:         mem.RlDB[i].SrcZone,
				DesZone:         mem.RlDB[i].DesZone,
				SrcIndex:        mem.RlDB[i].SrcIndex,
				Height:          mem.RlDB[i].CmHeight,
				Packages:        tp.ParsePackages(mem.RlDB[i].Packages),
				ConfirmPackSigs: mem.RlDB[i].ComfirmPackSig,
			}
			//fmt.Println(string(cm))
			mem.RlDB = append(mem.RlDB[:i], mem.RlDB[i+1:]...)
			//cmid,_:=json.Marshal(CmID(cm)

			checkdb.Save([]byte(CmID(cm)), cm) //存储
			//fmt.Println("存储", "CrossMerkleRoot root", cm.CrossMerkleRoot, "height", cm.Height, "id", []byte(CmID(cm)),"packages",cm.Packages,"Srczone",cm.SrcZone,"Deszone",cm.DesZone)
			i--
			break
		}
	}
	//fmt.Println("删除完映射表容量", len(mem.RlDB))
}
func ParseData(data types.Tx) *tp.CrossMessages {
	cm := new(tp.CrossMessages)
	err := json.Unmarshal(data, &cm)
	if err != nil {
		fmt.Println("ParseData Wrong")
	}
	if cm.CrossMerkleRoot == nil {

		return nil
	} else {
		return cm
	}
}
func ParseData1(data types.Tx) *tp.CrossMessages {
	cm := new(tp.CrossMessages)
	err := json.Unmarshal(data, &cm)
	if err != nil {
		fmt.Println("ParseData Wrong")
	}
	if cm.CrossMerkleRoot == nil {

		return nil
	} else {
		return cm
	}
}

//函数添加结束------------------------------------------------------------------
// CheckTxWithInfo performs the same operation as CheckTx, but with extra meta data about the tx.
// Currently this metadata is the peer who sent it,
// used to prevent the tx from being gossiped back to them.
func getShard() string {
	v, _ := syscall.Getenv("TASKID")
	return v
}

//传入是否是leader
func (mem *Mempool) CheckTxWithInfo(tx types.Tx, cb func(*abci.Response), txInfo TxInfo, checkdb bool) (err error) {
	mem.proxyMtx.Lock()
	// use defer to unlock mutex because application (*local client*) might panic
	defer mem.proxyMtx.Unlock()

	var (
		memSize  = mem.Size()
		txsBytes = mem.TxsBytes()
	)
	if memSize >= mem.config.Size ||
		int64(len(tx))+txsBytes > mem.config.MaxTxsBytes {

		return ErrMempoolIsFull{
			memSize, mem.config.Size,
			txsBytes, mem.config.MaxTxsBytes}
	}
	// The size of the corresponding amino-encoded TxMessage
	// can't be larger than the maxMsgSize, otherwise we can't
	// relay it to peers.
	if len(tx) > maxTxSize {

		return ErrTxTooLarge
	}
	if mem.preCheck != nil {
		if err := mem.preCheck(tx); err != nil {

			return ErrPreCheck{err}
		}
	}
	if cm := ParseData(tx); cm != nil {
		if txInfo.PeerID == UnknownPeerID { //说明是第一次接受
			//t := time.Now()
			//fmt.Printf("[tx_phase] index:tPreCheck1CM id:%X time:%s\n", CmID(cm), strconv.FormatInt(t.UnixNano(), 10))
		} else { //说明来自其他节点的同步
			//t := time.Now()
			//fmt.Printf("[tx_phase] index:tPreCheckCM id:%X time:%s\n", CmID(cm), strconv.FormatInt(t.UnixNano(), 10))
		}
		//mem.logger.Error("接受到Cm消息")
		if !checkdb {
			//begin_time := time.Now()
			if result := mem.CheckCrossMessage(cm); result != nil {
				return result
			}
			//end_time := time.Now()
			//phase40 := end_time.Sub(begin_time)
			//fmt.Printf("[tx_phase] index:periodCheck id:%X time:%s\n", CmID(cm), strconv.FormatInt(phase40.Nanoseconds(), 10))
			//fmt.Printf("[tx_phase] index:tPostCheck id:%X time:%s\n", CmID(cm), strconv.FormatInt(time.Now().UnixNano(), 10))
		}
	} else {
		if txInfo.PeerID == UnknownPeerID { //说明是第一次接受
			t := time.Now()
			tmp_tx, _ := tp.NewTX(tx)
			if PrintLog(tmp_tx.ID) {
				fmt.Printf("[tx_phase] index:tPreCheck1TX id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(t.UnixNano(), 10))
			}
		} else { //说明来自其他节点的同步
			t := time.Now()
			tmp_tx, _ := tp.NewTX(tx)
			if PrintLog(tmp_tx.ID) {
				fmt.Printf("[tx_phase] index:tPreCheckTX id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(t.UnixNano(), 10))

			}

		}
		accountLog := account.NewAccountLog(tx) //判断是否是leader再输出
		if accountLog == nil {
			return errors.New("交易解析失败")
		}
		checkRes := accountLog.Check()

		if !checkRes {
			return errors.New("不合法的交易")
		}
	}
	//mem.logger.Error("交易合法性检验通过")
	// CACHE
	if !mem.cache.Push(tx) {
		// Record a new sender for a tx we've already seen.
		// Note it's possible a tx is still in the cache but no longer in the mempool
		// (eg. after committing a block, txs are removed from mempool but not cache),
		// so we only record the sender for txs still in the mempool.
		if e, ok := mem.txsMap.Load(txKey(tx)); ok {
			memTx := e.(*clist.CElement).Value.(*mempoolTx)
			if _, loaded := memTx.senders.LoadOrStore(txInfo.PeerID, true); loaded {
				// TODO: consider punishing peer for dups,
				// its non-trivial since invalid txs can become valid,
				// but they can spam the same tx with little cost to them atm.
			}
		}
		//检验是否交易已经放入状态数据库或者区块之中

		return ErrTxInCache
	}
	// END CACHE
	/*
	 * @Author: zyj
	 * @Desc: check tx
	 * @Date: 19.11.10
	 */
	//检验cm的合法性

	//对addtx进行检验，如果是已经加入则直接返回
	// WAL
	if mem.wal != nil {
		// TODO: Notify administrators when WAL fails
		_, err := mem.wal.Write([]byte(tx))
		if err != nil {
			mem.logger.Error("Error writing to WAL", "err", err)
		}
		_, err = mem.wal.Write([]byte("\n"))
		if err != nil {
			mem.logger.Error("Error writing to WAL", "err", err)
		}
	}
	// END WAL

	// NOTE: proxyAppConn may error if tx buffer is full
	if err = mem.proxyAppConn.Error(); err != nil {
		return err
	}
	reqRes := mem.proxyAppConn.CheckTxAsync(tx)

	reqRes.SetCallback(mem.reqResCb(tx, txInfo.PeerID, cb))
	return nil
}

// Global callback that will be called after every ABCI response.
// Having a single global callback avoids needing to set a callback for each request.
// However, processing the checkTx response requires the peerID (so we can track which txs we heard from who),
// and peerID is not included in the ABCI request, so we have to set request-specific callbacks that
// include this information. If we're not in the midst of a recheck, this function will just return,
// so the request specific callback can do the work.
// When rechecking, we don't need the peerID, so the recheck callback happens here.
func (mem *Mempool) globalCb(req *abci.Request, res *abci.Response) {
	if mem.recheckCursor == nil {
		return
	}

	mem.metrics.RecheckTimes.Add(1)
	mem.resCbRecheck(req, res)

	// update metrics
	mem.metrics.Size.Set(float64(mem.Size()))
}

// Request specific callback that should be set on individual reqRes objects
// to incorporate local information when processing the response.
// This allows us to track the peer that sent us this tx, so we can avoid sending it back to them.
// NOTE: alternatively, we could include this information in the ABCI request itself.
//
// External callers of CheckTx, like the RPC, can also pass an externalCb through here that is called
// when all other response processing is complete.
//
// Used in CheckTxWithInfo to record PeerID who sent us the tx.
func (mem *Mempool) reqResCb(tx []byte, peerID uint16, externalCb func(*abci.Response)) func(res *abci.Response) {

	return func(res *abci.Response) {
		if mem.recheckCursor != nil {
			// this should never happen
			panic("recheck cursor is not nil in reqResCb")
		}

		mem.resCbFirstTime(tx, peerID, res)

		// update metrics
		mem.metrics.Size.Set(float64(mem.Size()))

		// passed in by the caller of CheckTx, eg. the RPC
		if externalCb != nil {
			externalCb(res)
		}
	}
}

// Called from:
//  - resCbFirstTime (lock not held) if tx is valid
func (mem *Mempool) addTx(memTx *mempoolTx) {
	e := mem.txs.PushBack(memTx)
	mem.txsMap.Store(txKey(memTx.tx), e)
	atomic.AddInt64(&mem.txsBytes, int64(len(memTx.tx)))
	mem.metrics.TxSizeBytes.Observe(float64(len(memTx.tx)))
}

// Called from:
//  - Update (lock held) if tx was committed
// 	- resCbRecheck (lock not held) if tx was invalidated
func (mem *Mempool) removeTx(tx types.Tx, elem *clist.CElement, removeFromCache bool) {
	mem.txs.Remove(elem)
	elem.DetachPrev()
	mem.txsMap.Delete(txKey(tx))
	atomic.AddInt64(&mem.txsBytes, int64(-len(tx)))

	if removeFromCache {
		mem.cache.Remove(tx)
	}
}

// callback, which is called after the app checked the tx for the first time.
//
// The case where the app checks the tx for the second and subsequent times is
// handled by the resCbRecheck callback.
func time2string(t int64) string {
	return strconv.FormatInt(t, 10)
}
func (mem *Mempool) resCbFirstTime(tx []byte, peerID uint16, res *abci.Response) {
	switch r := res.Value.(type) {
	case *abci.Response_CheckTx:
		var postCheckErr error
		if mem.postCheck != nil {
			postCheckErr = mem.postCheck(tx, r.CheckTx)
		}
		if (r.CheckTx.Code == abci.CodeTypeOK) && postCheckErr == nil {
			memTx := &mempoolTx{
				height:    mem.height,
				gasWanted: r.CheckTx.GasWanted,
				tx:        tx,
			}
			memTx.senders.Store(peerID, true)
			btime := time.Now()

			mem.addTx(memTx)
			etime := time.Now()
			if cm := ParseData(memTx.tx); cm != nil {
				//fmt.Printf("[tx_phase] index:periodAddCM id:%X time:%s\n", CmID(cm), strconv.FormatInt(etime.Sub(btime).Nanoseconds(), 10))
				//fmt.Printf("[tx_phase] index:tInsideMemCM id:%X time:%s\n", CmID(cm), strconv.FormatInt(time.Now().UnixNano(), 10))
			} else {

				tmp_tx, _ := tp.NewTX(memTx.tx)
				if PrintLog(tmp_tx.ID) {
					fmt.Printf("[tx_phase] index:periodAddTX id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(etime.Sub(btime).Nanoseconds(), 10))
					fmt.Printf("[tx_phase] index:tInsideMemTX id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(time.Now().UnixNano(), 10))

				}

			}

			mem.logger.Info("Added good transaction",
				"tx", TxID(tx),
				"res", r,
				"height", memTx.height,
				"total", mem.Size(),
			)
			mem.notifyTxsAvailable()

		} else {
			// ignore bad transaction
			mem.logger.Info("Rejected bad transaction", "tx", TxID(tx), "res", r, "err", postCheckErr)
			mem.metrics.FailedTxs.Add(1)
			// remove from cache (it might be good later)
			mem.cache.Remove(tx)
		}
	default:
		// ignore other messages
	}
}

// callback, which is called after the app rechecked the tx.
//
// The case where the app checks the tx for the first time is handled by the
// resCbFirstTime callback.
func (mem *Mempool) resCbRecheck(req *abci.Request, res *abci.Response) {
	switch r := res.Value.(type) {
	case *abci.Response_CheckTx:
		tx := req.GetCheckTx().Tx
		memTx := mem.recheckCursor.Value.(*mempoolTx)
		if !bytes.Equal(tx, memTx.tx) {
			panic(fmt.Sprintf(
				"Unexpected tx response from proxy during recheck\nExpected %X, got %X",
				memTx.tx,
				tx))
		}
		var postCheckErr error
		if mem.postCheck != nil {
			postCheckErr = mem.postCheck(tx, r.CheckTx)
		}
		if (r.CheckTx.Code == abci.CodeTypeOK) && postCheckErr == nil {
			// Good, nothing to do.
		} else {
			// Tx became invalidated due to newly committed block.
			mem.logger.Info("Tx is no longer valid", "tx", TxID(tx), "res", r, "err", postCheckErr)
			// NOTE: we remove tx from the cache because it might be good later
			mem.removeTx(tx, mem.recheckCursor, true)
		}
		if mem.recheckCursor == mem.recheckEnd {
			mem.recheckCursor = nil
		} else {
			mem.recheckCursor = mem.recheckCursor.Next()
		}
		if mem.recheckCursor == nil {
			// Done!
			atomic.StoreInt32(&mem.rechecking, 0)
			mem.logger.Info("Done rechecking txs")

			// incase the recheck removed all txs
			if mem.Size() > 0 {
				mem.notifyTxsAvailable()
			}
		}
	default:
		// ignore other messages
	}
}

// TxsAvailable returns a channel which fires once for every height,
// and only when transactions are available in the mempool.
// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
func (mem *Mempool) TxsAvailable() <-chan struct{} {
	return mem.txsAvailable
}

func (mem *Mempool) notifyTxsAvailable() {
	if mem.Size() == 0 {
		panic("notified txs available but mempool is empty!")
	}
	if mem.txsAvailable != nil && !mem.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		mem.notifiedTxsAvailable = true
		select {
		case mem.txsAvailable <- struct{}{}:
		default:
		}
	}

}

// ReapMaxBytesMaxGas reaps transactions from the mempool up to maxBytes bytes total
// with the condition that the total gasWanted must be less than maxGas.
// If both maxes are negative, there is no cap on the size of all returned
// transactions (~ all available transactions).
//func Printcmlog(phase string,id string,t time.Time){
//	fmt.Printf("[tx_phase] index:tReapMemDone1 id:%X time:%s\n", CmID(cm), strconv.FormatInt(t.Now().UnixNano(), 10))
//
//}
func (mem *Mempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64, height int64) types.Txs {
	//区块能取到最大的数量，最大gas值
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()
	//mem.logger.Error("reap txs")
	for atomic.LoadInt32(&mem.rechecking) > 0 {
		// TODO: Something better?
		time.Sleep(time.Millisecond * 10)
	}

	var totalBytes int64
	var totalGas int64
	// TODO: we will get a performance boost if we have a good estimate of avg
	// size per tx, and set the initial capacity based off of that.
	// txs := make([]types.Tx, 0, cmn.MinInt(mem.txs.Len(), max/mem.avgTxSize))
	txs := make([]types.Tx, 0, mem.txs.Len())

	for e := mem.txs.Front(); e != nil; e = e.Next() {
		//取交易
		memTx := e.Value.(*mempoolTx)

		// Check total size requirement
		// 删除对应的txlist的relay_out交易
		//parse_time := time.Now()                   //解析时间
		if cm := ParseData1(memTx.tx); cm != nil { //拿到交易，并且是cm类型的
			//t := time.Now()
			//fmt.Printf("[tx_phase] index:tReapMem1 id:%X time:%s\n", CmID(cm), strconv.FormatInt(t.UnixNano(), 10))
			if cm.SrcZone == getShard() {
				//fmt.Println("移除回执")
				var txs types.Txs
				txs = append(txs, memTx.tx)
				mem.removeTxs(txs)
				continue
			}
			if res := cm.Decompression(); !res {
				mem.logger.Error("cross message decompression failed.")
				return nil
			}
			//添加映射关系
			//对txlist进行遍历，将relay_in的tx加入区块之中
			var txlist []*tp.TX
			var byte_txlist []types.Tx
			for i := 0; i < len(cm.Txlist); i++ {
				tmp_tx, err := tp.NewTX(cm.Txlist[i])
				if err != nil {
					mem.logger.Error("Unmarshall tp.TX error, err: ", err)
				}

				if tmp_tx.Txtype == "relaytx" {
					byte_txlist = append(byte_txlist, cm.Txlist[i])
				}
			}
			if res := cm.Compress(); !res {
				mem.logger.Error("cross message compress failed.")
				return nil
			}

			total_data, err := json.Marshal(txlist)
			if err != nil {
				mem.logger.Error("reap 聚合交易解析错误")
				return nil
			}
			aminoOverhead := types.ComputeAminoOverhead(total_data, 1)
			if maxBytes > -1 && totalBytes+int64(len(total_data))+aminoOverhead > maxBytes {
				//fmt.Println("区块最大容量为：", maxBytes, "本次要打包交易的容量为", int64(len(total_data))+aminoOverhead, "因此打包失败")
				return txs
			}

			//在这里如果交易通过核验，那么就可以确定要取该交易。那么此时，我们需要在这里对交易进行判断,加入映射表之中
			//TODO:映射表的完善
			mem.AddRelationTable(cm, height, txKey(memTx.tx))
			totalBytes += int64(len(total_data)) + aminoOverhead
			//fmt.Println("区块最大容量为：", maxBytes, "本次要打包交易的容量为", int64(len(total_data))+aminoOverhead, "因此打包成功")
			// Check total gas requirement.
			// If maxGas is negative, skip this check.
			// Since newTotalGas < masGas, which
			// must be non-negative, it follows that this won't overflow.
			// gasWanted是什么意思？？？是否需要生成所有的gasWanted？
			//todo:GasWanted调研
			newTotalGas := totalGas + memTx.gasWanted
			if maxGas > -1 && newTotalGas > maxGas {
				return txs
			}
			totalGas = newTotalGas
			//做测试
			//for i := 0; i < len(byte_txlist); i++ {
			//	//tmp_tx1, err := tp.NewTX(byte_txlist[i])
			//	if err != nil {
			//		mem.logger.Error("Unmarshall tp.TX error, err: ", err)
			//	}
			//	//mem.logger.Info(TimePhase(41, tmp_tx1.ID, strconv.FormatInt(time.Now().UnixNano(), 10))) //第41阶段打印
			//
			//}
			//parseend_time := time.Now()
			//fmt.Printf("[tx_phase] index:pickCM id:%X time:%s\n", CmID(cm), strconv.FormatInt(parseend_time.Sub(parse_time).Nanoseconds(), 10))
			//fmt.Printf("[tx_phase] index:tReapMemDone1 id:%X time:%s\n", CmID(cm), strconv.FormatInt(time.Now().UnixNano(), 10))

			txs = append(txs, byte_txlist...)
		} else {
			t := time.Now()
			tmp_tx, _ := tp.NewTX(memTx.tx)
			if PrintLog(tmp_tx.ID) {
				fmt.Printf("[tx_phase] index:tReapMem1 id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(t.UnixNano(), 10))
			}
			aminoOverhead := types.ComputeAminoOverhead(memTx.tx, 1)
			if maxBytes > -1 && totalBytes+int64(len(memTx.tx))+aminoOverhead > maxBytes {
				return txs
			}
			//在这里如果交易通过核验，那么就可以确定要取该交易。那么此时，我们需要在这里对交易进行判断

			totalBytes += int64(len(memTx.tx)) + aminoOverhead
			// Check total gas requirement.
			// If maxGas is negative, skip this check.
			// Since newTotalGas < masGas, which
			// must be non-negative, it follows that this won't overflow.
			newTotalGas := totalGas + memTx.gasWanted
			if maxGas > -1 && newTotalGas > maxGas {
				return txs
			}
			totalGas = newTotalGas
			tmp_tx, err := tp.NewTX(memTx.tx)
			if err != nil {
				mem.logger.Error("Unmarshall tp.TX error, err: ", err)
			}
			if PrintLog(tmp_tx.ID) {
				fmt.Printf("[tx_phase] index:tReapMemDone1 id:%X time:%s\n", tmp_tx.ID, strconv.FormatInt(t.UnixNano(), 10))
			}

			//mem.logger.Info(TimePhase(21, tmp_tx.ID, strconv.FormatInt(time.Now().UnixNano(), 10))) //第21阶段打印
			txs = append(txs, memTx.tx)
		}

	}
	return txs
}

// ReapMaxTxs reaps up to max transactions from the mempool.
// If max is negative, there is no cap on the size of all returned
// transactions (~ all available transactions).
func (mem *Mempool) ReapMaxTxs(max int) types.Txs {

	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	if max < 0 {
		max = mem.txs.Len()
	}

	for atomic.LoadInt32(&mem.rechecking) > 0 {
		// TODO: Something better?
		time.Sleep(time.Millisecond * 10)
	}
	txs := make([]types.Tx, 0, cmn.MinInt(mem.txs.Len(), max))
	for e := mem.txs.Front(); e != nil && len(txs) <= max; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}

// Update informs the mempool that the given txs were committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
// NOTE: unsafe; Lock/Unlock must be managed by caller
func DetectTx(tx types.Tx) bool {
	parsetx, _ := tp.NewTX(tx)
	//fmt.Println("该区块的tx",parsetx)
	//说明是relay_out交易
	if parsetx.Txtype == "relaytx" && parsetx.Receiver == getShard() {
		//将该条交易删除
		return true
	} else {
		return false
	}
}
func RlID(rl RelationTable) string {
	heightHash := Sum([]byte(strconv.FormatInt(rl.CmHeight, 10)))
	ID := append(heightHash, rl.CmHash...)
	return fmt.Sprintf("%X", ID)
}
func (mem *Mempool) RemoveCm(height int64) {
	CmsMap := make(map[string]struct{}, len(mem.RlDB))
	for i := 0; i < len(mem.RlDB); i++ {
		if mem.RlDB[i].CsHeight == height {
			//说明在本次区块，该cm被打包了
			//聚合
			CmsMap[RlID(mem.RlDB[i])] = struct{}{}
		}
	}
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		// Remove the tx if it's already in a block.
		if cm := ParseData(memTx.tx); cm != nil {
			if _, ok := CmsMap[CmID(cm)]; ok {
				// NOTE: we don't remove committed txs from the cache.
				mem.removeTx(memTx.tx, e, false)
				//固化到磁盘中

				mem.RemoveRelationTable(CmID(cm))
				continue
			}

		} else {

		}
	}

}
func (mem *Mempool) Update(
	height int64,
	txs types.Txs,
	preCheck PreCheckFunc,
	postCheck PostCheckFunc,
) error {
	// Set height
	mem.height = height
	mem.notifiedTxsAvailable = false

	if preCheck != nil {
		mem.preCheck = preCheck
	}
	if postCheck != nil {
		mem.postCheck = postCheck
	}

	//	// Add committed transactions to cache (if missing).
	for _, tx := range txs {
		if DetectTx(tx) {
			continue
		}
		//fmt.Println("push交易")

		_ = mem.cache.Push(tx)
	}
	//删除对应交易包
	mem.RemoveCm(height)
	// Remove committed transactions.
	txsLeft := mem.removeTxs(txs)
	// Either recheck non-committed txs to see if they became invalid
	// or just notify there're some txs left.
	if len(txsLeft) > 0 {
		if mem.config.Recheck {
			mem.logger.Info("Recheck txs", "numtxs", len(txsLeft), "height", height)
			mem.recheckTxs(txsLeft)
			// At this point, mem.txs are being rechecked.
			// mem.recheckCursor re-scans mem.txs and possibly removes some txs.
			// Before mem.Reap(), we should wait for mem.recheckCursor to be nil.
		} else {
			mem.notifyTxsAvailable()
		}
	}

	// Update metrics
	mem.metrics.Size.Set(float64(mem.Size()))

	return nil
}

func (mem *Mempool) removeTxs(txs types.Txs) []types.Tx {
	// Build a map for faster lookups.
	txsMap := make(map[string]struct{}, len(txs))

	for _, tx := range txs {
		//主要是避免由cm产生的tx
		if DetectTx(tx) {
			continue
		}
		tx1, _ := tp.NewTX(tx)
		//如果operate=1且relaytx且还是本分片的交易，则是在本次区块产生的时候
		if tx1.Operate == 1 && tx1.Txtype == "relaytx" && tx1.Sender == getShard() {
			tx1.Operate = 0
			txdata, _ := json.Marshal(tx1)
			txsMap[string(txdata)] = struct{}{}
		} else {
			txsMap[string(tx)] = struct{}{}
		}
	}
	//A tm-bench A->B op=0
	txsLeft := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)

		// Remove the tx if it's already in a block.
		if _, ok := txsMap[string(memTx.tx)]; ok {
			// NOTE: we don't remove committed txs from the cache.
			mem.removeTx(memTx.tx, e, false)

			continue
		}

		txsLeft = append(txsLeft, memTx.tx)
	}
	return txsLeft
}

// NOTE: pass in txs because mem.txs can mutate concurrently.
func (mem *Mempool) recheckTxs(txs []types.Tx) {
	if len(txs) == 0 {
		return
	}
	atomic.StoreInt32(&mem.rechecking, 1)
	mem.recheckCursor = mem.txs.Front()
	mem.recheckEnd = mem.txs.Back()

	// Push txs to proxyAppConn
	// NOTE: globalCb may be called concurrently.
	for _, tx := range txs {
		mem.proxyAppConn.CheckTxAsync(tx)
	}
	mem.proxyAppConn.FlushAsync()
}

//--------------------------------------------------------------------------------

// mempoolTx is a transaction that successfully ran
type mempoolTx struct {
	height    int64    // height that this tx had been validated in
	gasWanted int64    // amount of gas this tx states it will require
	tx        types.Tx //

	// ids of peers who've sent us this tx (as a map for quick lookups).
	// senders: PeerID -> bool
	senders sync.Map
}
type mempoolCm struct {
	height    int64             // height that this tx had been validated in
	gasWanted int64             // amount of gas this tx states it will require
	cm        *tp.CrossMessages //

	// ids of peers who've sent us this tx (as a map for quick lookups).
	// senders: PeerID -> bool
	senders sync.Map
}

// Height returns the height for this transaction
func (memTx *mempoolTx) Height() int64 {
	return atomic.LoadInt64(&memTx.height)
}
func (memCm *mempoolCm) Height() int64 {
	return atomic.LoadInt64(&memCm.height)
}

//--------------------------------------------------------------------------------

type txCache interface {
	Reset()
	Push(tx types.Tx) bool
	Remove(tx types.Tx)
}

// mapTxCache maintains a LRU cache of transactions. This only stores the hash
// of the tx, due to memory concerns.
type mapTxCache struct {
	mtx  sync.Mutex
	size int
	map_ map[[sha256.Size]byte]*list.Element
	list *list.List
}

var _ txCache = (*mapTxCache)(nil)

// newMapTxCache returns a new mapTxCache.
func newMapTxCache(cacheSize int) *mapTxCache {
	return &mapTxCache{
		size: cacheSize,
		map_: make(map[[sha256.Size]byte]*list.Element, cacheSize),
		list: list.New(),
	}
}

// Reset resets the cache to an empty state.
func (cache *mapTxCache) Reset() {
	cache.mtx.Lock()
	cache.map_ = make(map[[sha256.Size]byte]*list.Element, cache.size)
	cache.list.Init()
	cache.mtx.Unlock()
}

// Push adds the given tx to the cache and returns true. It returns
// false if tx is already in the cache.
func (cache *mapTxCache) Push(tx types.Tx) bool {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	// Use the tx hash in the cache
	txHash := txKey(tx)
	if moved, exists := cache.map_[txHash]; exists {
		cache.list.MoveToBack(moved)
		return false
	}

	if cache.list.Len() >= cache.size {
		popped := cache.list.Front()
		poppedTxHash := popped.Value.([sha256.Size]byte)
		delete(cache.map_, poppedTxHash)
		if popped != nil {
			cache.list.Remove(popped)
		}
	}
	e := cache.list.PushBack(txHash)
	cache.map_[txHash] = e
	return true
}

// Remove removes the given tx from the cache.
func (cache *mapTxCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	txHash := txKey(tx)
	popped := cache.map_[txHash]
	delete(cache.map_, txHash)
	if popped != nil {
		cache.list.Remove(popped)
	}

	cache.mtx.Unlock()
}

type nopTxCache struct{}

var _ txCache = (*nopTxCache)(nil)

func (nopTxCache) Reset()             {}
func (nopTxCache) Push(types.Tx) bool { return true }
func (nopTxCache) Remove(types.Tx)    {}

// 检查CrossMessage交易，检查tree path，但不验证tree root签名
func (mem *Mempool) CheckCrossMessageSig(cm *tp.CrossMessages) bool {
	// 检验参数
	if cm == nil {
		return true
	}
	//fmt.Println("收到的cm待检验",*cm)
	pubkey, err := bls.GetPubkeyFromByte(cm.Pubkeys)
	if err != nil {
		mem.logger.Error("公钥还原出错，", cm.Pubkeys, ", err: ", err)
		return false
	}
	//fmt.Println("sig: ", cm.Sig)
	//fmt.Println("root: ", cm.CrossMerkleRoot)
	//fmt.Println("pub: ", cm.Pubkeys)
	if res := pubkey.VerifyBytes(cm.CrossMerkleRoot, cm.Sig); !res {
		mem.logger.Error("验证CrossMessage的signature出错")
		fmt.Println(cm.Sig)
		fmt.Println(cm.CrossMerkleRoot)
		fmt.Println(cm.Pubkeys)
		return false
	}
	//fmt.Println("门限签名验证通过！")
	// 根据交易重构当前交易包的tree root，该root也是CrossMerkle tree的一个叶子节点
	txs := cm.Txlist

	// 生成分片的tree root，获得该分片的merkle tree root来验证路径的正确性
	smt := merkle.SimpleTreeFromByteSlices(txs)
	//fmt.Println("生成树")
	if smt == nil {
		fmt.Println("生成树失败")
		return false
	}
	//fmt.Println("生成树成功")
	currentRoot := smt.ComputeRootHash()
	if currentRoot == nil || len(currentRoot) == 0 {
		return false
	}
	return merkle.VerifyTreePath(currentRoot, cm.TreePath, cm.CrossMerkleRoot)
}
