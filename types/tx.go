package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/identypes"
	"strconv"
	"strings"

	amino "github.com/tendermint/go-amino"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// Tx is an arbitrary byte array.
// NOTE: Tx has no types at this level, so when wire encoded it's just length-prefixed.
// Might we want types here ?
type Tx []byte

// Hash computes the TMHASH hash of the wire encoded transaction.
func (tx Tx) Hash() []byte {
	return tmhash.Sum(tx)
}

// String returns the hex-encoded transaction as a string.
func (tx Tx) String() string {
	return fmt.Sprintf("Tx{%X}", []byte(tx))
}

// Txs is a slice of Tx.
type Txs []Tx

// TODO 从configtx中解析出待更新待validators
func ExecuteConfigTx(origin Tx) ([]*Validator, error) {
	validators := make([]*Validator, 0, 0)

	tx, err := identypes.NewTX(origin)
	if err != nil {
		return nil, errors.New("[ExecuteConfigTx] parse tx failed. err" + err.Error())
	} else if tx.Txtype != identypes.ConfigTx {
		return nil, errors.New("[ExecuteConfigTx] tx type is wrong, expect 'configtx' but got " + tx.Txtype)
	}

	// 暂用tendermint deliver tx的格式
	ValidatorSetChangePrefix := "val:"
	content := tx.Content[len(ValidatorSetChangePrefix):]

	//get the pubkey and power
	pubKeyAndPower := strings.Split(content, "/")
	if len(pubKeyAndPower) != 2 {
		fmt.Println("[ExecuteConfigTx] 解析交易内容错误")
		return nil, errors.New("[ExecuteConfigTx] parse config tx error, wrong format")
	}
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]
	pubkey, err := hex.DecodeString(pubkeyS)
	power, err := strconv.ParseInt(powerS, 10, 64)
	fmt.Println("[ExecuteConfigTx] parsed tx: ", pubkey, power)

	tmppub := abci.BlsValidatorUpdate(pubkey, int64(power))
	// conver abci.Pubkey 2 crypto.Pubkey
	pub, err := PB2TM.PubKey(tmppub.PubKey)
	if err != nil {
		fmt.Println("[ExecuteConfigTx] conver abci.Pubkey 2 crypto.Pubkey failed. err: ", err)
		return validators, errors.New("[ExecuteConfigTx] conver abci.Pubkey 2 crypto.Pubkey failed. err: " + err.Error())
	}

	validator := NewValidator(pub, power)
	validators = append(validators, validator)

	return validators, nil
}

// Hash returns the Merkle root hash of the transaction hashes.
// i.e. the leaves of the tree are the hashes of the txs.
func (txs Txs) Hash() []byte {
	// These allocations will be removed once Txs is switched to [][]byte,
	// ref #2603. This is because golang does not allow type casting slices without unsafe
	txBzs := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i].Hash()
	}
	return merkle.SimpleHashFromByteSlices(txBzs)
}

// Index returns the index of this transaction in the list, or -1 if not found
func (txs Txs) Index(tx Tx) int {
	for i := range txs {
		if bytes.Equal(txs[i], tx) {
			return i
		}
	}
	return -1
}

// IndexByHash returns the index of this transaction hash in the list, or -1 if not found
func (txs Txs) IndexByHash(hash []byte) int {
	for i := range txs {
		if bytes.Equal(txs[i].Hash(), hash) {
			return i
		}
	}
	return -1
}

// Proof returns a simple merkle proof for this node.
// Panics if i < 0 or i >= len(txs)
// TODO: optimize this!
func (txs Txs) Proof(i int) TxProof {
	l := len(txs)
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {
		bzs[i] = txs[i].Hash()
	}
	root, proofs := merkle.SimpleProofsFromByteSlices(bzs)

	return TxProof{
		RootHash: root,
		Data:     txs[i],
		Proof:    *proofs[i],
	}
}

func (txs Txs) Bytes() [][]byte {
	l := len(txs)
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {
		bzs[i] = make([]byte, len(txs[i]))
		copy(bzs[i], txs[i])
	}

	return bzs
}

// TxProof represents a Merkle proof of the presence of a transaction in the Merkle tree.
type TxProof struct {
	RootHash cmn.HexBytes
	Data     Tx
	Proof    merkle.SimpleProof
}

// Leaf returns the hash(tx), which is the leaf in the merkle tree which this proof refers to.
func (tp TxProof) Leaf() []byte {
	return tp.Data.Hash()
}

// Validate verifies the proof. It returns nil if the RootHash matches the dataHash argument,
// and if the proof is internally consistent. Otherwise, it returns a sensible error.
func (tp TxProof) Validate(dataHash []byte) error {
	if !bytes.Equal(dataHash, tp.RootHash) {
		return errors.New("Proof matches different data hash")
	}
	if tp.Proof.Index < 0 {
		return errors.New("Proof index cannot be negative")
	}
	if tp.Proof.Total <= 0 {
		return errors.New("Proof total must be positive")
	}
	valid := tp.Proof.Verify(tp.RootHash, tp.Leaf())
	if valid != nil {
		return errors.New("Proof is not internally consistent")
	}
	return nil
}

// TxResult contains results of executing the transaction.
//
// One usage is indexing transaction results.
type TxResult struct {
	Height int64                  `json:"height"`
	Index  uint32                 `json:"index"`
	Tx     Tx                     `json:"tx"`
	Result abci.ResponseDeliverTx `json:"result"`
}

// ComputeAminoOverhead calculates the overhead for amino encoding a transaction.
// The overhead consists of varint encoding the field number and the wire type
// (= length-delimited = 2), and another varint encoding the length of the
// transaction.
// The field number can be the field number of the particular transaction, or
// the field number of the parenting struct that contains the transactions []Tx
// as a field (this field number is repeated for each contained Tx).
// If some []Tx are encoded directly (without a parenting struct), the default
// fieldNum is also 1 (see BinFieldNum in amino.MarshalBinaryBare).
func ComputeAminoOverhead(tx Tx, fieldNum int) int64 {
	fnum := uint64(fieldNum)
	typ3AndFieldNum := (uint64(fnum) << 3) | uint64(amino.Typ3_ByteLength)
	return int64(amino.UvarintSize(typ3AndFieldNum)) + int64(amino.UvarintSize(uint64(len(tx))))
}
