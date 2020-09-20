package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/libs/log"
	myline "github.com/tendermint/tendermint/line"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	ac "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/types"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)

	trySyncIntervalMS = 10

	// stop syncing when last block's time is
	// within this much of the system time.
	// stopSyncingDurationMinutes = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(sm.State, int)
}

type peerError struct {
	err    error
	peerID p2p.ID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	p2p.BaseReactor
	line *myline.Line
	// immutable
	initialState sm.State
	blockExec    *sm.BlockExecutor
	store        *BlockStore
	pool         *BlockPool
	fastSync     bool

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError
}
type node struct {
	target map[string][]string
}

// NewBlockchainReactor returns new reactor instance.
func (bcR *BlockchainReactor) newline1() *myline.Line {

	endpoints := &node{
		target: make(map[string][]string, 16),
	}

	endpoints.target["A"] = []string{"192.168.5.56:26657", "192.168.5.56:36657", "192.168.5.56:46657", "192.168.5.56:56657"}
	endpoints.target["B"] = []string{"192.168.5.57:26657", "192.168.5.57:36657", "192.168.5.57:46657", "192.168.5.57:56657"}
	endpoints.target["C"] = []string{"192.168.5.58:26657", "192.168.5.58:36657", "192.168.5.58:46657", "192.168.5.58:56657"}
	endpoints.target["D"] = []string{"192.168.5.60:26657", "192.168.5.60:36657", "192.168.5.60:46657", "192.168.5.60:56657"}

	l := myline.NewLine(endpoints.target)
	return l
}
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	requestsCh := make(chan BlockRequest, maxTotalRequesters)

	const capacity = 1000                      // must be bigger than peers count
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewBlockPool(
		store.Height()+1,
		requestsCh,
		errorsCh,
	)
	bcR := &BlockchainReactor{
		initialState: state,
		blockExec:    blockExec,
		store:        store,
		pool:         pool,
		fastSync:     fastSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
	}
	bcR.line = bcR.newline1()
	//go func(){
	//	if err:=bcR.line.Start();err!=nil{
	//		fmt.Println(err)
	//	}
	//	fmt.Println("reactor connect!!!!!!!!!!!!!!!!")
	//}()

	bcR.BaseReactor = *p2p.NewBaseReactor("BlockchainReactor", bcR)
	return bcR
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.Logger) {
	bcR.BaseService.Logger = l
	bcR.pool.Logger = l
}

// OnStart implements cmn.Service.
func (bcR *BlockchainReactor) OnStart() error {
	if bcR.fastSync {
		err := bcR.pool.Start()
		if err != nil {
			return err
		}

		go bcR.poolRoutine()
	}
	//go func(){
	//	if err:=bcR.line.Start();err!=nil{
	//		fmt.Println(err)
	//		return
	//	}
	//	fmt.Println("newconsensusstate connect!!!!!!!!!!!!!!!!")
	//	return
	//}()
	return nil
}

// OnStop implements cmn.Service.
func (bcR *BlockchainReactor) OnStop() {
	bcR.pool.Stop()
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
	if !peer.Send(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcR.pool.RemovePeer(peer.ID())
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a block that's non-existent.
func (bcR *BlockchainReactor) respondToPeer(msg *bcBlockRequestMessage,
	src p2p.Peer) (queued bool) {

	block := bcR.store.LoadBlock(msg.Height)
	if block != nil {
		msgBytes := cdc.MustMarshalBinaryBare(&bcBlockResponseMessage{Block: block})
		return src.TrySend(BlockchainChannel, msgBytes)
	}

	bcR.Logger.Info("Peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes := cdc.MustMarshalBinaryBare(&bcNoBlockResponseMessage{Height: msg.Height})
	return src.TrySend(BlockchainChannel, msgBytes)
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		bcR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		bcR.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	bcR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcBlockRequestMessage:
		if queued := bcR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcBlockResponseMessage:
		bcR.pool.AddBlock(src.ID(), msg.Block, len(msgBytes))
	case *bcStatusRequestMessage:
		// Send peer our state.
		msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
		queued := src.TrySend(BlockchainChannel, msgBytes)
		if !queued {
			// sorry
		}
	case *bcStatusResponseMessage:
		// Got a peer status. Unverified.
		bcR.pool.SetPeerHeight(src.ID(), msg.Height)

	/*
	 * @Author: zyj
	 * @Desc: 增加快照请求和响应的逻辑处理
	 * @Date: 19.12.07
	 */
	case *bcSnapshotRequestMessage:
		bcR.Logger.Error("收到快照请求! 现在发送快照")
		// 获取所有状态集合
		snapshot := ac.GetSnapshot()
		snapShopMap, _ := json.Marshal(snapshot.Content)
		msgBytes := cdc.MustMarshalBinaryBare(&bcSnapshotResponseMessage{snapshot.Version, snapShopMap})
		src.TrySend(BlockchainChannel, msgBytes)

	case *bcSnapshotResponseMessage:
		bcR.Logger.Error(fmt.Sprintf("收到快照版本为 %v, 内容为%v", msg.Version, string(msg.Content)))
		// 处理快照
		myMap := make(map[string]string)
		json.Unmarshal(msg.Content, &myMap)
		count := 0
		for k, v := range myMap {
			// 快照写入
			ac.SetState([]byte(k), []byte(v))
			count++
		}
		bcR.Logger.Error(fmt.Sprintf("快照写入完成, 长度为%v", count))
		// 更新当前快照
		ac.SetSnapshot(ac.Snapshot{msg.Version, myMap})

		// 更新当前区块高度为快照版本
		if msg.Version > 0 && msg.Version < 0x7fffffff {
			bcR.store.height = msg.Version     // just for test
			// 更新pool中的高度为当前高度+1
			bcR.pool.height = bcR.store.height + 1
			bcR.initialState.LastBlockHeight = bcR.store.height
			// 修改consensus模块中的区块高度
			//cs := consensus.GetConsensusState()
			//cs.Height = bcR.store.height
		}

		// 标记为新加入节点
		ac.IsNewPeer = true
		/* ---------- zyj change --------- */
	default:
		bcR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcR *BlockchainReactor) poolRoutine() {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

	blocksSynced := 0

	chainID := bcR.initialState.ChainID
	state := bcR.initialState

	lastHundred := time.Now()
	lastRate := 0.0

	didProcessCh := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case <-bcR.Quit():
				return
			case <-bcR.pool.Quit():
				return
			case request := <-bcR.requestsCh:
				peer := bcR.Switch.Peers().Get(request.PeerID)
				if peer == nil {
					continue
				}
				/*
				 * @Author: zyj
				 * @Desc: 增加快照请求发送逻辑，当requestsCh通道中高度为-1时触发
				 * @Date: 19.11.24
				 */
				if request.Height == -1 {
					bcR.Logger.Error(fmt.Sprintf("现在发送同步快照请求: %v", request.PeerID))
					msgBytes := cdc.MustMarshalBinaryBare(&bcSnapshotRequestMessage{})
					peer.TrySend(BlockchainChannel, msgBytes)
					continue
				}
				/* ---------- zyj change --------- */
				msgBytes := cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{request.Height})
				queued := peer.TrySend(BlockchainChannel, msgBytes)
				if !queued {
					bcR.Logger.Debug("Send queue is full, drop block request", "peer", peer.ID(), "height", request.Height)
				}
			case err := <-bcR.errorsCh:
				peer := bcR.Switch.Peers().Get(err.peerID)
				if peer != nil {
					bcR.Switch.StopPeerForError(peer, err)
				}

			case <-statusUpdateTicker.C:
				// ask for status updates
				go bcR.BroadcastStatusRequest() // nolint: errcheck

			}
		}
	}()
FOR_LOOP:
	for {
		select {
		case <-switchToConsensusTicker.C:
			height, numPending, lenRequesters := bcR.pool.GetStatus()
			outbound, inbound, _ := bcR.Switch.NumPeers()
			bcR.Logger.Debug("Consensus ticker", "numPending", numPending, "total", lenRequesters,
				"outbound", outbound, "inbound", inbound)
			if bcR.pool.IsCaughtUp() {
				bcR.Logger.Info("Time to switch to consensus reactor!", "height", height)
				/*
                 * @Author: zyj
                 * @Desc: 更新高度，同步至consensus模块，否则会从高度1开始共识
                 * @Date: 19.11.24
                 */
				state.LastBlockHeight = height - 1
				// ------------------------------------
				bcR.pool.Stop()
				conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
				if ok {
					conR.SwitchToConsensus(state, blocksSynced)
				} else {
					// should only happen during testing
				}

				break FOR_LOOP
			}

		case <-trySyncTicker.C: // chan time
			select {
			case didProcessCh <- struct{}{}:
			default:
			}

		case <-didProcessCh:
			// NOTE: It is a subtle mistake to process more than a single block
			// at a time (e.g. 10) here, because we only TrySend 1 request per
			// loop.  The ratio mismatch can result in starving of blocks, a
			// sudden burst of requests and responses, and repeat.
			// Consequently, it is better to split these routines rather than
			// coupling them as it's written here.  TODO uncouple from request
			// routine.

			// See if there are any blocks to sync.
			first, second := bcR.pool.PeekTwoBlocks()
			//bcR.Logger.Info("TrySync peeked", "first", first, "second", second)
			if first == nil || second == nil {
				// We need both to sync the first block.
				continue FOR_LOOP
			} else {
				// Try again quickly next loop.
				didProcessCh <- struct{}{}
			}

			firstParts := first.MakePartSet(types.BlockPartSizeBytes)
			firstPartsHeader := firstParts.Header()
			firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}
			// Finally, verify the first block using the second's commit
			// NOTE: we can probably make this more efficient, but note that calling
			// first.Hash() doesn't verify the tx contents, so MakePartSet() is
			// currently necessary.
			err := state.Validators.VerifyCommit(
				chainID, firstID, first.Height, second.LastCommit)
			if err != nil {
				bcR.Logger.Error("Error in validation", "err", err)
				peerID := bcR.pool.RedoRequest(first.Height)
				peer := bcR.Switch.Peers().Get(peerID)
				if peer != nil {
					// NOTE: we've already removed the peer's request, but we
					// still need to clean up the rest.
					bcR.Switch.StopPeerForError(peer, fmt.Errorf("BlockchainReactor validation error: %v", err))
				}
				peerID2 := bcR.pool.RedoRequest(second.Height)
				peer2 := bcR.Switch.Peers().Get(peerID2)
				if peer2 != nil && peer2 != peer {
					// NOTE: we've already removed the peer's request, but we
					// still need to clean up the rest.
					bcR.Switch.StopPeerForError(peer2, fmt.Errorf("BlockchainReactor validation error: %v", err))
				}
				continue FOR_LOOP
			} else {
				bcR.pool.PopRequest()

				// TODO: batch saves so we dont persist to disk every block
				bcR.store.SaveBlock(first, firstParts, second.LastCommit)

				// TODO: same thing for app - but we would need a way to
				// get the hash without persisting the state
				var err error
				state, err = bcR.blockExec.ApplyBlock(state, firstID, first, false)
				if err != nil {
					// TODO This is bad, are we zombie?
					panic(fmt.Sprintf("Failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
				}
				blocksSynced++

				if blocksSynced%100 == 0 {

					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					bcR.Logger.Info("Fast Sync Rate", "height", bcR.pool.height,
						"max_peer_height", bcR.pool.MaxPeerHeight(), "blocks/s", lastRate)
					lastHundred = time.Now()
				}
			}
			continue FOR_LOOP

		case <-bcR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) BroadcastStatusRequest() error {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
	return nil
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface {
	ValidateBasic() error
}

/*
 * @Author: zyj
 * @Desc: 增加快照消息类型注册
 * @Date: 19.11.24
 */
func RegisterBlockchainMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainMessage)(nil), nil)
	cdc.RegisterConcrete(&bcBlockRequestMessage{}, "tendermint/blockchain/BlockRequest", nil)
	cdc.RegisterConcrete(&bcBlockResponseMessage{}, "tendermint/blockchain/BlockResponse", nil)
	cdc.RegisterConcrete(&bcNoBlockResponseMessage{}, "tendermint/blockchain/NoBlockResponse", nil)
	cdc.RegisterConcrete(&bcStatusResponseMessage{}, "tendermint/blockchain/StatusResponse", nil)
	cdc.RegisterConcrete(&bcStatusRequestMessage{}, "tendermint/blockchain/StatusRequest", nil)
	cdc.RegisterConcrete(&bcSnapshotRequestMessage{}, "tendermint/blockchain/SnapshotRequest", nil)
	cdc.RegisterConcrete(&bcSnapshotResponseMessage{}, "tendermint/blockchain/SnapshotResponse", nil)
}

func decodeMsg(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

type bcBlockRequestMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcBlockRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (m *bcBlockRequestMessage) String() string {
	return fmt.Sprintf("[bcBlockRequestMessage %v]", m.Height)
}

type bcNoBlockResponseMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcNoBlockResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (brm *bcNoBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcNoBlockResponseMessage %d]", brm.Height)
}

//-------------------------------------

type bcBlockResponseMessage struct {
	Block *types.Block
}

// ValidateBasic performs basic validation.
func (m *bcBlockResponseMessage) ValidateBasic() error {
	return m.Block.ValidateBasic()
}

func (m *bcBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (m *bcStatusRequestMessage) String() string {
	return fmt.Sprintf("[bcStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (m *bcStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v]", m.Height)
}

//-------------------------------------
/*
 * @Author: zyj
 * @Desc: 增加快照消息类型定义
 * @Date: 19.11.24
 */
type bcSnapshotRequestMessage struct {
}

func (m *bcSnapshotRequestMessage) ValidateBasic() error {
	return nil
}


func (m *bcSnapshotRequestMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v]")
}


type bcSnapshotResponseMessage struct {
	Version int64
	Content []byte
}

func (m *bcSnapshotResponseMessage) ValidateBasic() error {
	return nil
}

func (m *bcSnapshotResponseMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v]", m.Version)
}