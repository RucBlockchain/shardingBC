package state

import (
	"crypto/sha256"
	abci "github.com/tendermint/tendermint/abci/types"
	tp "github.com/tendermint/tendermint/identypes"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//------------------------------------------------------
// mempool

// Mempool defines the mempool interface as used by the ConsensusState.
// Updates to the mempool need to be synchronized with committing a block
// so apps can reset their transient state on Commit
type Mempool interface {
	Lock()
	Unlock()

	//AddRelaytxDB(tx tp.TX)
	//RemoveRelaytxDB(tx tp.TX)
	//UpdaterDB() []tp.TX
	//GetAllTxs() []tp.TX

	AddCrossMessagesDB(tcm *tp.CrossMessages)
	RemoveCrossMessagesDB(tcm *tp.CrossMessages)
	UpdatecmDB() []*tp.CrossMessages
	GetAllCrossMessages() []*tp.CrossMessages
	SearchRelationTable(Height int64) []tp.Package
	ModifyRelationTable(packages []byte, cfs []byte, height int64)
	SearchPackageExist(pack tp.Package) bool
	LogPrint(phase string, tx_id [sha256.Size]byte, t int64, logtype int)
	SyncRelationTable(pack tp.Package, height int64)
	Size() int
	CheckTx(types.Tx, func(*abci.Response)) error
	CheckTxWithInfo(types.Tx, func(*abci.Response), mempool.TxInfo, bool) error
	ReapMaxBytesMaxGas(maxBytes, maxGas int64, height int64) types.Txs
	ReapDeliverTx() types.Tx
	Update(int64, types.Txs, mempool.PreCheckFunc, mempool.PostCheckFunc) error
	Flush()
	FlushAppConn() error
	GetCrossSendStrike()int
	GetCrossReceiveStrike()int
	SetCrossSendStrike(rate int)
	SetCrossReceiveStrike(rate int)
	CheckSendSeed()bool
	TxsAvailable() <-chan struct{}
	EnableTxsAvailable()

	CalculateBusyScore(int, int64, int64) float64
	GetBusyScore() float64
	GetShardFilters() map[string]struct{}
	UpdateShardFliters([]string, []string)
}

// MockMempool is an empty implementation of a Mempool, useful for testing.
type MockMempool struct{}

var _ Mempool = MockMempool{}

func (MockMempool) Lock()                                                                {}
func (MockMempool) Unlock()                                                              {}
func (MockMempool) Size() int                                                            { return 0 }
func (MockMempool) LogPrint(phase string, tx_id [sha256.Size]byte, t int64, logtype int) {}

func (MockMempool) CheckTx(_ types.Tx, _ func(*abci.Response)) error {
	return nil
}
func (MockMempool) CheckTxWithInfo(_ types.Tx, _ func(*abci.Response),
	_ mempool.TxInfo, _ bool) error {
	return nil
}
func (MockMempool) ReapMaxBytesMaxGas(_, _ int64, _ int64) types.Txs { return types.Txs{} }
func (MockMempool) Update(
	_ int64,
	_ types.Txs,
	_ mempool.PreCheckFunc,
	_ mempool.PostCheckFunc,
) error {
	return nil
}
func (MockMempool) Flush()                                   {}
func (MockMempool) FlushAppConn() error                      { return nil }
func (MockMempool) TxsAvailable() <-chan struct{}            { return make(chan struct{}) }
func (MockMempool) EnableTxsAvailable()                      {}
func (MockMempool) AddCrossMessagesDB(tcm *tp.CrossMessages) {}
func (MockMempool) SearchRelationTable(Height int64) []tp.Package {
	var pack []tp.Package
	return pack
}
func (MockMempool) SyncRelationTable(tp tp.Package, height int64)                 {}
func (MockMempool) ModifyRelationTable(packages []byte, cfs []byte, height int64) {}
func (MockMempool) SearchPackageExist(tp tp.Package) bool {

	return true
}
func (MockMempool) GetCrossSendStrike() int {

	return 0
}
func (MockMempool) GetCrossReceiveStrike() int {

	return 0
}

func (MockMempool) SetCrossSendStrike(rate int){


}
func (MockMempool) CheckSendSeed()bool{
	return true

}
func (MockMempool) SetCrossReceiveStrike(rate int){


}
func (MockMempool) RemoveCrossMessagesDB(tcm *tp.CrossMessages) {}
func (MockMempool) UpdatecmDB() []*tp.CrossMessages {
	var cms []*tp.CrossMessages
	return cms
}
func (MockMempool) GetAllCrossMessages() []*tp.CrossMessages {
	var cms []*tp.CrossMessages
	return cms
}

func (MockMempool) ReapDeliverTx() types.Tx {
	return nil
}

func (MockMempool) CalculateBusyScore(int, int64, int64) float64 {
	return 0
}

func (MockMempool) GetBusyScore() float64 {
	return 0
}

func (MockMempool) GetShardFilters() map[string]struct{} {
	return nil
}

func (MockMempool) UpdateShardFliters([]string, []string) {

}

//------------------------------------------------------
// blockstore

// BlockStoreRPC is the block store interface used by the RPC.
type BlockStoreRPC interface {
	Height() int64

	LoadBlockMeta(height int64) *types.BlockMeta
	LoadBlock(height int64) *types.Block
	LoadBlockPart(height int64, index int) *types.Part

	LoadBlockCommit(height int64) *types.Commit
	LoadSeenCommit(height int64) *types.Commit
}

// BlockStore defines the BlockStore interface used by the ConsensusState.
type BlockStore interface {
	BlockStoreRPC
	SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
}

//-----------------------------------------------------------------------------------------------------
// evidence pool

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
// Get/Set/Commit
type EvidencePool interface {
	PendingEvidence(int64) []types.Evidence
	AddEvidence(types.Evidence) error
	Update(*types.Block, State)
	// IsCommitted indicates if this evidence was already marked committed in another block.
	IsCommitted(types.Evidence) bool
}

// MockMempool is an empty implementation of a Mempool, useful for testing.
type MockEvidencePool struct{}

func (m MockEvidencePool) PendingEvidence(int64) []types.Evidence { return nil }
func (m MockEvidencePool) AddEvidence(types.Evidence) error       { return nil }
func (m MockEvidencePool) Update(*types.Block, State)             {}
func (m MockEvidencePool) IsCommitted(types.Evidence) bool        { return false }
