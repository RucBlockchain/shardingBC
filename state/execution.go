package state

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/account"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"net"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tp "github.com/tendermint/tendermint/identypes"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/fail"
	"github.com/tendermint/tendermint/libs/log"
	myline "github.com/tendermint/tendermint/line"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	myclient "github.com/tendermint/tendermint/client"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.

const (
	sendTimeout = 100 * time.Second
)

type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	db dbm.DB

	// execute the app against this
	proxyApp proxy.AppConnConsensus

	// events
	eventBus types.BlockEventPublisher

	// manage the mempool lock during commit
	// and update both with block results after commit.
	mempool Mempool
	evpool  EvidencePool

	logger log.Logger

	metrics *Metrics
	Client []*myclient.HTTP
}

type BlockExecutorOption func(executor *BlockExecutor)

func BlockExecutorWithMetrics(metrics *Metrics) BlockExecutorOption {
	return func(blockExec *BlockExecutor) {
		blockExec.metrics = metrics
	}
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(db dbm.DB, logger log.Logger, proxyApp proxy.AppConnConsensus, mempool Mempool, evpool EvidencePool, options ...BlockExecutorOption) *BlockExecutor {
	res := &BlockExecutor{
		db:       db,
		proxyApp: proxyApp,
		eventBus: types.NopEventBus{},
		mempool:  mempool,
		evpool:   evpool,
		logger:   logger,
		metrics:  NopMetrics(),
	}
	
	for _, option := range options {
		option(res)
	}

	/*
	 * @Author: zyj
	 * @Desc: transfer the db reference
	 * @Date: 19.11.09
	 */
	account.InitAccountDB(res.db, res.logger)

	return res
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (blockExec *BlockExecutor) SetEventBus(eventBus types.BlockEventPublisher) {
	blockExec.eventBus = eventBus
}

// CreateProposalBlock calls state.MakeBlock with evidence from the evpool
// and txs from the mempool. The max bytes must be big enough to fit the commit.
// Up to 1/10th of the block space is allcoated for maximum sized evidence.
// The rest is given to txs, up to the max gas.
func (blockExec *BlockExecutor) CreateProposalBlock(
	height int64,
	state State, commit *types.Commit,
	proposerAddr []byte,
) (*types.Block, *types.PartSet) {

	maxBytes := state.ConsensusParams.Block.MaxBytes
	maxGas := state.ConsensusParams.Block.MaxGas

	// Fetch a limited amount of valid evidence
	maxNumEvidence, _ := types.MaxEvidencePerBlock(maxBytes)
	evidence := blockExec.evpool.PendingEvidence(maxNumEvidence)

	// Fetch a limited amount of valid txs
	maxDataBytes := types.MaxDataBytes(maxBytes, state.Validators.Size(), len(evidence))
	txs := blockExec.mempool.ReapMaxBytesMaxGas(maxDataBytes, maxGas)

	return state.MakeBlock(height, txs, commit, evidence, proposerAddr)
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(state State, block *types.Block) error {
	return validateBlock(blockExec.evpool, blockExec.db, state, block)
}

// ApplyBlock validates the block against the state, executes it against the app,
// fires the relevant events, commits the app, and saves the new state and responses.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock( /*line *myline.Line,*/ state State, blockID types.BlockID, block *types.Block, flag bool) (State, error) {

	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	startTime := time.Now().UnixNano()
	abciResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, state.LastValidators, blockExec.db)
	endTime := time.Now().UnixNano()
	blockExec.metrics.BlockProcessingTime.Observe(float64(endTime-startTime) / 1000000)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	fail.Fail() // XXX

	// Save the results before we commit.
	saveABCIResponses(blockExec.db, block.Height, abciResponses)

	fail.Fail() // XXX

	// validate the validator updates and convert to tendermint types
	abciValUpdates := abciResponses.EndBlock.ValidatorUpdates
	err = validateValidatorUpdates(abciValUpdates, state.ConsensusParams.Validator)
	if err != nil {
		return state, fmt.Errorf("Error in validator updates: %v", err)
	}
	validatorUpdates, err := types.PB2TM.ValidatorUpdates(abciValUpdates)
	if err != nil {
		return state, err
	}
	if len(validatorUpdates) > 0 {
		blockExec.logger.Info("Updates to validators", "updates", types.ValidatorListString(validatorUpdates))
	}

	// Update the state with the block and responses.
	state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates, block.Height)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}
	// Lock mempool, commit app state, update mempoool.
	appHash, err := blockExec.Commit(state, block)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	// Update evpool with the block and state.
	blockExec.evpool.Update(block, state)

	fail.Fail() // XXX

	// Update the app hash and save the state.
	state.AppHash = appHash
	SaveState(blockExec.db, state)

	fail.Fail() // XXX
	//从这里开始添加新的函数
	//检查自己身份，判断是否是leader,如果是leader再执行检查
	
	blockExec.CheckRelayTxs(block,flag)

	// Events are fired after everything else.
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses, validatorUpdates)

	return state, nil
}

//------------------------------------------------------
//检查是否有跨链交易产生，对其进行后续处理
func (blockExec *BlockExecutor) CheckRelayTxs( /*line *myline.Line,*/ block *types.Block,flag bool) {

	fmt.Println("-------------Begin check Relay Txsc----------")
	resendTxs := blockExec.UpdateRelaytxDB() //检查状态数据库，没有及时确认的relayTxs需要重新发送relaytxs
	if flag{
		//只有leader执行以下代码
		var shard_send [][]tp.TX
		shard_send = make([][]tp.TX, 16)
		//将需要跨片的交易按分片归类
		for i := 0; i < len(resendTxs); i++ {
			index := int(resendTxs[i].Receiver[0]) - 65
			shard_send[index] = append(shard_send[index], resendTxs[i])
		}
		var num int
		num = 0
		var tx_package []tp.TX
		for i := 0; i < len(shard_send); i++ {
			if shard_send[i] != nil {
				num = num + len(shard_send[i]) //发送到某分片所有跨片交易的数量，进行打包
				tx_package = shard_send[i]
				go blockExec.Send_Package(num, i, tx_package)
			}
		}
		fmt.Println("需要发送的CheckRelayTxs交易数量：", num)
	}

	//对当前提交的块检查，看是否有新的relayTxs产生
	sendtxs, receivetxs := blockExec.CheckCommitedBlock(block)
	if flag {
		//只有leader节点发送交易
		if sendtxs != nil {
			blockExec.SendRelayTxs(sendtxs) //如果有relaytx,向其他分区发送交易（地址可以通过relaytx的格式解析）
		}
		if receivetxs != nil {
			blockExec.SendAddedRelayTxs(receivetxs) //如果该分区收到的relaytx已经add，向发送的分区回复
		}
			//每20个，更新一次checkpoint
		if block.Height%20 == 0 {
			cpTxs := blockExec.GetAllTxs()
			cptx := conver2cptx(cpTxs, block.Height)
			Sendcptx(cptx, 0) //TODO 改为一定要加入成功
		}
	}

}

func (blockExec *BlockExecutor) CheckCommitedBlock(block *types.Block) ([]tp.TX, []tp.TX) { //判断relay tx是否存在
	//检查block中所有的tx是否包含relay TX
	//返回两种，新加入到分区的和已被确认的relaytx
	fmt.Println("CheckCommitedBlock")
	var sendtxs []tp.TX
	var receivetxs []tp.TX
	if block.Data.Txs != nil {
		for i := 0; i < len(block.Data.Txs); i++ {

			data := block.Data.Txs[i]
			encodeStr := hex.EncodeToString(data)

			temptx, _ := hex.DecodeString(encodeStr) //得到真实的tx记录
			//fmt.Println(string(temptx))
			var t tp.TX
			json.Unmarshal(temptx, &t)

			if t.Txtype == "tx" {
				continue
			} else if t.Txtype == "relaytx" {
				if t.Sender == block.Shard {
					sendtxs = append(sendtxs, t)
					blockExec.Add2RelaytxDB(t)
				} else if t.Receiver == block.Shard {
					receivetxs = append(receivetxs, t)
				}
			} else if t.Txtype == "addtx" {
				blockExec.RemoveFromRelaytxDB(t)
				//continue

			} else if t.Txtype == "checkpoint" {
				fmt.Println("checkpoint",t)
				continue
			}
		}
		myline.Count = 0
	}
	return sendtxs, receivetxs
}

func (blockExec *BlockExecutor) Add2RelaytxDB(tx tp.TX) {
	//fmt.Println("Add2RelaytxDB")
	blockExec.mempool.AddRelaytxDB(tx)
}
func (blockExec *BlockExecutor) RemoveFromRelaytxDB(tx tp.TX) {
	//fmt.Println("RemoveFromRelaytxDB")
	blockExec.mempool.RemoveRelaytxDB(tx)
}
func (blockExec *BlockExecutor) UpdateRelaytxDB() []tp.TX {
	fmt.Println("UpdateRelaytxDB")
	resendTxs := blockExec.mempool.UpdaterDB()
	return resendTxs
}
func (blockExec *BlockExecutor) GetAllTxs() []tp.TX {
	fmt.Println("GetAllTxs")
	cpTxs := blockExec.mempool.GetAllTxs()
	return cpTxs
}

func (blockExec *BlockExecutor) SendRelayTxs( /*line *myline.Line,*/ txs []tp.TX) {
	fmt.Println("SendRelayTxs")
	//暂时定义分片有4个

	var shard_send [][]tp.TX
	shard_send = make([][]tp.TX, 16)
	//将需要跨片的交易按分片归类
	for i := 0; i < len(txs); i++ {
		flag := int(txs[i].Receiver[0]) - 65
		shard_send[flag] = append(shard_send[flag], txs[i])
	}
	fmt.Println("SendRelayTxs1")
	var tx_package []tp.TX
	//begin := time.Now()
	for i := 0; i < len(shard_send); i++ {
		if shard_send[i] != nil {
			num := len(shard_send[i]) //发送到某分片所有跨片交易的数量，进行打包
			tx_package = shard_send[i]
			go blockExec.Send_Package(num, i, tx_package)

		}
	}
	//end := time.Now().Sub(begin)
	//fmt.Println("Send using time:", end)
}
func (blockExec *BlockExecutor) Send_Package(num int, i int, tx_package []tp.TX) {
	var c2 *websocket.Conn
	var rnd int
	var key string
	var index int
	
	if num > 0 {
		
		if tx_package[0].Txtype == "addtx" {
			key = tx_package[0].Sender

			//fmt.Println("发往分片",key)
			//c2, rnd = myline.UseConnect(key, "ip")
		} else {
			key = tx_package[0].Receiver
			//fmt.Println("发往分片",key)
			//fmt.Println(key,len(tx_package))
			//c2, rnd = myline.UseConnect(key, "ip")
		}
		index = int(key[0]) - 65
		blockExec.SendMessage(index, rnd, c2, tx_package)
		
	}
}
// sending tx to shard x
func (blockExec *BlockExecutor) SendMessage(index int, rnd int, c *websocket.Conn, tx_package []tp.TX){
	name := "TT"+string(index+65)+"Node2:26657"
	client := *myclient.NewHTTP(name,"/websocket")
	client.BroadcastTxAsync(tx_package)
}
func (blockExec *BlockExecutor) Send_Message(index int, rnd int, c *websocket.Conn, tx_package []tp.TX) {
	
	res, _ := json.Marshal(tx_package)
	rawParamsJSON := json.RawMessage(res)
	//第一层打包结束

	//paramsJSON, err := json.Marshal(map[string]interface{}{"tx": res})
	//if err != nil {
	//	fmt.Printf("failed to encode params: %v\n", err)
	//	os.Exit(1)
	//}
	c.SetPingHandler(func(message string) error {
		err := c.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(sendTimeout))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	})

	err1 := c.WriteJSON(rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Sender:  "flag",
		ID:      rpctypes.JSONRPCStringID("relay"),
		Method:  "broadcast_tx_async",
		Params:  rawParamsJSON,
	})
	if err1 != nil {
		fmt.Println("发送err", err1)
		c := myline.ReStart1(string(index+65), rnd)
		c.WriteJSON(rpctypes.RPCRequest{
			JSONRPC: "2.0",
			Sender:  "flag",
			ID:      rpctypes.JSONRPCStringID("relay"),
			Method:  "broadcast_tx_async",
			Params:  rawParamsJSON,
		})
		time.Sleep(time.Millisecond * 100)
		myline.Flag_conn[string(index+65)][rnd] = false
		return
	}
	//fmt.Println("发送完毕",tx_package[0].Txtype,"共",len(tx_package),"条交易")
	time.Sleep(time.Millisecond * 100)
	//time.Sleep(time.Millisecond*100)
	myline.Flag_conn[string(index+65)][rnd] = false //释放资源

}

func (blockExec *BlockExecutor) SendAddedRelayTxs( /*line *myline.Line,*/ txs []tp.TX) {
	//向发送来的分片中返回确认消息
	fmt.Println("SendAddedRelayTxs")
	//暂时定义分片有4个
	var shard_send [][]tp.TX
	shard_send = make([][]tp.TX, 16)
	//将需要跨片的交易按分片归类
	for i := 0; i < len(txs); i++ {
		flag := int(txs[i].Receiver[0]) - 65
		txs[i].Txtype = "addtx"
		shard_send[flag] = append(shard_send[flag], txs[i])
	}
	var tx_package []tp.TX

	for i := 0; i < len(shard_send); i++ {
		if shard_send[i] != nil {
			num := len(shard_send[i])
			tx_package = shard_send[i]
			go blockExec.Send_Package(num, i, tx_package)
		}
	}
}

type RPCRequest struct {
	JSONRPC  string          `json:"jsonrpc"`
	ID       string          `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"`   // must be map[string]interface{} or []interface{}
	Sender   string          `json:"Sender"`   //添加发送者
	Receiver string          `json:"Receiver"` //添加接受者
	Flag     int             `json:"Flag"`
}

//----------------------------------------------------------------------------

// Commit locks the mempool, runs the ABCI Commit message, and updates the
// mempool.
// It returns the result of calling abci.Commit (the AppHash), and an error.
// The Mempool must be locked during commit and update because state is
// typically reset on Commit and old txs must be replayed against committed
// state before new txs are run in the mempool, lest they be invalid.
func (blockExec *BlockExecutor) Commit(
	state State,
	block *types.Block,
) ([]byte, error) {
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// while mempool is Locked, flush to ensure all async requests have completed
	// in the ABCI app before Commit.
	err := blockExec.mempool.FlushAppConn()
	if err != nil {
		blockExec.logger.Error("Client error during mempool.FlushAppConn", "err", err)
		return nil, err
	}

	// Commit block, get hash back
	res, err := blockExec.proxyApp.CommitSync()
	if err != nil {
		blockExec.logger.Error(
			"Client error during proxyAppConn.CommitSync",
			"err", err,
		)
		return nil, err
	}
	// ResponseCommit has no error code - just data

	blockExec.logger.Info(
		"Committed state",
		"height", block.Height,
		"txs", block.NumTxs,
		"appHash", fmt.Sprintf("%X", res.Data),
	)
	// Update mempool.
	err = blockExec.mempool.Update(
		block.Height,
		block.Txs,
		TxPreCheck(state),
		TxPostCheck(state),
	)

	return res.Data, err
}

//---------------------------------------------------------
// Helper functions for executing blocks and updating state

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
func execBlockOnProxyApp(
	logger log.Logger,
	proxyAppConn proxy.AppConnConsensus,
	block *types.Block,
	lastValSet *types.ValidatorSet,
	stateDB dbm.DB,
) (*ABCIResponses, error) {
	var validTxs, invalidTxs = 0, 0

	txIndex := 0
	abciResponses := NewABCIResponses(block)
	//fmt.Println("运行1")
	// Execute transactions and get hash.
	proxyCb := func(req *abci.Request, res *abci.Response) {
		switch r := res.Value.(type) {
		case *abci.Response_DeliverTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			txRes := r.DeliverTx
			if txRes.Code == abci.CodeTypeOK {
				validTxs++
			} else {
				logger.Debug("Invalid tx", "code", txRes.Code, "log", txRes.Log)
				invalidTxs++
			}
			abciResponses.DeliverTx[txIndex] = txRes
			txIndex++
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	commitInfo, byzVals := getBeginBlockValidatorInfo(block, lastValSet, stateDB)
	//fmt.Println("运行2")
	// Begin block
	var err error
	abciResponses.BeginBlock, err = proxyAppConn.BeginBlockSync(abci.RequestBeginBlock{
		Hash:                block.Hash(),
		Header:              types.TM2PB.Header(&block.Header),
		LastCommitInfo:      commitInfo,
		ByzantineValidators: byzVals,
	})
	if err != nil {
		logger.Error("Error in proxyAppConn.BeginBlock", "err", err)
		return nil, err
	}

	// Run txs of block.
	for _, tx := range block.Txs {
		proxyAppConn.DeliverTxAsync(tx)
		if err := proxyAppConn.Error(); err != nil {
			return nil, err
		}

		/*
         * @Author: zyj
         * @Desc: update state
         * @Date: 19.11.10
         */
		accountLog := account.NewAccountLog(tx)
		if accountLog != nil {
			accountLog.Save()
		}
	}

	// End block.
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(abci.RequestEndBlock{Height: block.Height})
	if err != nil {
		logger.Error("Error in proxyAppConn.EndBlock", "err", err)
		return nil, err
	}
	//fmt.Println("运行3")
	logger.Info("Executed block", "height", block.Height, "validTxs", validTxs, "invalidTxs", invalidTxs)

	return abciResponses, nil
}

func getBeginBlockValidatorInfo(block *types.Block, lastValSet *types.ValidatorSet, stateDB dbm.DB) (abci.LastCommitInfo, []abci.Evidence) {

	// Sanity check that commit length matches validator set size -
	// only applies after first block
	if block.Height > 1 {
		precommitLen := len(block.LastCommit.Precommits)
		valSetLen := len(lastValSet.Validators)
		if precommitLen != valSetLen {
			// sanity check
			panic(fmt.Sprintf("precommit length (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				precommitLen, valSetLen, block.Height, block.LastCommit.Precommits, lastValSet.Validators))
		}
	}

	// Collect the vote info (list of validators and whether or not they signed).
	voteInfos := make([]abci.VoteInfo, len(lastValSet.Validators))
	for i, val := range lastValSet.Validators {
		var vote *types.CommitSig
		if i < len(block.LastCommit.Precommits) {
			vote = block.LastCommit.Precommits[i]
		}
		voteInfo := abci.VoteInfo{
			Validator:       types.TM2PB.Validator(val),
			SignedLastBlock: vote != nil,
		}
		voteInfos[i] = voteInfo
	}

	commitInfo := abci.LastCommitInfo{
		Round: int32(block.LastCommit.Round()),
		Votes: voteInfos,
	}

	byzVals := make([]abci.Evidence, len(block.Evidence.Evidence))
	for i, ev := range block.Evidence.Evidence {
		// We need the validator set. We already did this in validateBlock.
		// TODO: Should we instead cache the valset in the evidence itself and add
		// `SetValidatorSet()` and `ToABCI` methods ?
		valset, err := LoadValidators(stateDB, ev.Height())
		if err != nil {
			panic(err) // shouldn't happen
		}
		byzVals[i] = types.TM2PB.Evidence(ev, valset, block.Time)
	}

	return commitInfo, byzVals

}

func validateValidatorUpdates(abciUpdates []abci.ValidatorUpdate,
	params types.ValidatorParams) error {
	for _, valUpdate := range abciUpdates {
		if valUpdate.GetPower() < 0 {
			return fmt.Errorf("Voting power can't be negative %v", valUpdate)
		} else if valUpdate.GetPower() == 0 {
			// continue, since this is deleting the validator, and thus there is no
			// pubkey to check
			continue
		}

		// Check if validator's pubkey matches an ABCI type in the consensus params
		thisKeyType := valUpdate.PubKey.Type
		if !params.IsValidPubkeyType(thisKeyType) {
			return fmt.Errorf("Validator %v is using pubkey %s, which is unsupported for consensus",
				valUpdate, thisKeyType)
		}
	}
	return nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(
	state State,
	blockID types.BlockID,
	header *types.Header,
	abciResponses *ABCIResponses,
	validatorUpdates []*types.Validator,
	height int64,
) (State, error) {

	// Copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators.
	nValSet := state.NextValidators.Copy()

	// Update the validator set with the latest abciResponses.
	flag := false
	if height%2 == 0 {
		flag = true
	}
	// rand.Seed(time.Now().Unix())
	// randnum := rand.Intn(100) // [0,100)的随机值，返回值为int
	// if randnum < 10 {
	// 	flag = true
	// }
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(validatorUpdates) > 0 {
		//为了固定leader，可以不更新validator set

		err := nValSet.UpdateWithChangeSet(validatorUpdates)
		if err != nil {
			return state, fmt.Errorf("Error changing validator set: %v", err)
		}

		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// Update validator proposer priority and set state variables.

	//不必更新set中的优先级
	if flag {
		nValSet.IncrementProposerPriority(1)
	}
	// Update the params with the latest abciResponses.
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if abciResponses.EndBlock.ConsensusParamUpdates != nil {
		// NOTE: must not mutate s.ConsensusParams
		nextParams = state.ConsensusParams.Update(abciResponses.EndBlock.ConsensusParamUpdates)
		err := nextParams.Validate()
		if err != nil {
			return state, fmt.Errorf("Error updating consensus params: %v", err)
		}
		// Change results from this height but only applies to the next height.
		lastHeightParamsChanged = header.Height + 1
	}

	// TODO: allow app to upgrade version
	nextVersion := state.Version

	// NOTE: the AppHash has not been populated.
	// It will be filled on state.Save.
	return State{
		Version:                          nextVersion,
		ChainID:                          state.ChainID,
		LastBlockHeight:                  header.Height,
		LastBlockTotalTx:                 state.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		NextValidators:                   nValSet,
		Validators:                       state.NextValidators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		LastResultsHash:                  abciResponses.ResultsHash(),
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(logger log.Logger, eventBus types.BlockEventPublisher, block *types.Block, abciResponses *ABCIResponses, validatorUpdates []*types.Validator) {
	eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block:            block,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})
	eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header:           block.Header,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	})

	for i, tx := range block.Data.Txs {
		eventBus.PublishEventTx(types.EventDataTx{TxResult: types.TxResult{
			Height: block.Height,
			Index:  uint32(i),
			Tx:     tx,
			Result: *(abciResponses.DeliverTx[i]),
		}})
	}

	if len(validatorUpdates) > 0 {
		eventBus.PublishEventValidatorSetUpdates(
			types.EventDataValidatorSetUpdates{ValidatorUpdates: validatorUpdates})
	}
}

//----------------------------------------------------------------------------------------------------
// Execute block without state. TODO: eliminate

// ExecCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (result of abci.Commit).
func ExecCommitBlock(
	appConnConsensus proxy.AppConnConsensus,
	block *types.Block,
	logger log.Logger,
	lastValSet *types.ValidatorSet,
	stateDB dbm.DB,
) ([]byte, error) {
	_, err := execBlockOnProxyApp(logger, appConnConsensus, block, lastValSet, stateDB)
	if err != nil {
		logger.Error("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// Commit block, get hash back
	res, err := appConnConsensus.CommitSync()
	if err != nil {
		logger.Error("Client error during proxyAppConn.CommitSync", "err", res)
		return nil, err
	}
	// ResponseCommit has no error or log, just data
	return res.Data, nil
}
