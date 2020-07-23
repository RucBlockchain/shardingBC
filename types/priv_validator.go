package types

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/identypes"
	"strconv"
	"syscall"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

// PrivValidator defines the functionality of a local Tendermint validator
// that signs votes and proposals, and never double signs.
type PrivValidator interface {
	GetPubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignCrossTXVote(txs Txs, vote *Vote) error // 为每一个跨片交易产生一个签名
	SigCrossMerkleRoot(MerkleRoot []byte, vote *Vote) error
}

//----------------------------------------
// Misc.

type PrivValidatorsByAddress []PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].GetPubKey().Address(), pvs[j].GetPubKey().Address()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}

//----------------------------------------
// MockPV

// MockPV implements PrivValidator without any safety or persistence.
// Only use it for testing.
type MockPV struct {
	privKey              crypto.PrivKey
	breakProposalSigning bool
	breakVoteSigning     bool
}

func NewMockPV() *MockPV {
	return &MockPV{ed25519.GenPrivKey(), false, false}
}

// NewMockPVWithParams allows one to create a MockPV instance, but with finer
// grained control over the operation of the mock validator. This is useful for
// mocking test failures.
func NewMockPVWithParams(privKey crypto.PrivKey, breakProposalSigning, breakVoteSigning bool) *MockPV {
	return &MockPV{privKey, breakProposalSigning, breakVoteSigning}
}

// Implements PrivValidator.
func (pv *MockPV) GetPubKey() crypto.PubKey {
	return pv.privKey.PubKey()
}

// Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, vote *Vote) error {
	useChainID := chainID
	if pv.breakVoteSigning {
		useChainID = "incorrect-chain-id"
	}
	signBytes := vote.SignBytes(useChainID)
	sig, err := pv.privKey.Sign(signBytes)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return nil
}

// Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, proposal *Proposal) error {
	useChainID := chainID
	if pv.breakProposalSigning {
		useChainID = "incorrect-chain-id"
	}
	signBytes := proposal.SignBytes(useChainID)
	sig, err := pv.privKey.Sign(signBytes)
	if err != nil {
		return err
	}
	proposal.Signature = sig
	return nil
}

// Implements PrivValidator.
func (pv *MockPV) SignCrossTXVote(txs Txs, vote *Vote) error {
	var successNo, errorNo int
	CTxSigs := make([]identypes.VoteCrossTxSig, 0, len(txs))
	for _, txdata := range txs {
		tx, err := identypes.NewTX(txdata)
		if err != nil {
			return err
		}
		if tx.Txtype != "relaytx" {
			// 暂时只处理跨片交易的前半程，后半程的addtx没想好
			continue
		}

		if sig, err := pv.privKey.Sign(tx.Digest()); err == nil {
			csig := identypes.VoteCrossTxSig{TxId: tx.ID, CrossTxSig: sig}
			CTxSigs = append(CTxSigs, csig)
			successNo += 1
		} else {
			errorNo += 1
		}
	}

	fmt.Printf("[mockPV] Sign cross traction,  success: %v, error: %v", successNo, errorNo)
	// log for debug
	fmt.Println("=============== crossTx Sig ===============")

	for _, csig := range CTxSigs {
		fmt.Println("txid: ", csig.TxId, "sig: ", csig.CrossTxSig)
	}
	fmt.Println("=============== Sig End ===============")

	//copy(vote.CrossTxSigs, CTxSigs)
	return nil
}

//分割字符串得到相应的内容,默认容器名为：1_0 分片名_分片的index
func ParseId() int64 {
	v, _ := syscall.Getenv("TASKID")
	g, _ := syscall.Getenv("TASKINDEX")
	Shard, _ := strconv.Atoi(v)
	Index, _ := strconv.Atoi(g)
	var id int64
	id = int64(Shard*500 + Index)
	return id
}
func (pv *MockPV) SigCrossMerkleRoot(MerkleRoot []byte, vote *Vote) error {
	vote.PartSig.Id = ParseId()
	if sign, err := pv.privKey.Sign(MerkleRoot); err == nil {
		vote.PartSig.PeerCrossSig = sign
	} else {
		return err
	}
	return nil
}

// String returns a string representation of the MockPV.
func (pv *MockPV) String() string {
	addr := pv.GetPubKey().Address()
	return fmt.Sprintf("MockPV{%v}", addr)
}

// XXX: Implement.
func (pv *MockPV) DisableChecks() {
	// Currently this does nothing,
	// as MockPV has no safety checks at all.
}

type erroringMockPV struct {
	*MockPV
}

var ErroringMockPVErr = errors.New("erroringMockPV always returns an error")

// Implements PrivValidator.
func (pv *erroringMockPV) SignVote(chainID string, vote *Vote) error {
	return ErroringMockPVErr
}

// Implements PrivValidator.
func (pv *erroringMockPV) SignProposal(chainID string, proposal *Proposal) error {
	return ErroringMockPVErr
}

// NewErroringMockPV returns a MockPV that fails on each signing request. Again, for testing only.
func NewErroringMockPV() *erroringMockPV {
	return &erroringMockPV{&MockPV{ed25519.GenPrivKey(), false, false}}
}
