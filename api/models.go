package api

import (
	errs "github.com/onflow/flow-evm-gateway/api/errors"
	"github.com/onflow/flow-evm-gateway/config"
	"github.com/onflow/flow-evm-gateway/models"

	"github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/common/hexutil"
	"github.com/onflow/go-ethereum/core/types"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`
}

// AccessListResult returns an optional accesslist
// It's the result of the `debug_createAccessList` RPC call.
// It contains an error if the transaction itself failed.
type AccessListResult struct {
	Accesslist *types.AccessList `json:"accessList"`
	Error      string            `json:"error,omitempty"`
	GasUsed    hexutil.Uint64    `json:"gasUsed"`
}

type FeeHistoryResult struct {
	OldestBlock  *hexutil.Big     `json:"oldestBlock"`
	Reward       [][]*hexutil.Big `json:"reward,omitempty"`
	BaseFee      []*hexutil.Big   `json:"baseFeePerGas,omitempty"`
	GasUsedRatio []float64        `json:"gasUsedRatio"`
}

// AccountResult structs for GetProof
type AccountResult struct {
	Address      common.Address  `json:"address"`
	AccountProof []string        `json:"accountProof"`
	Balance      *hexutil.Big    `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        hexutil.Uint64  `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	StorageProof []StorageResult `json:"storageProof"`
}

type StorageResult struct {
	Key   string       `json:"key"`
	Value *hexutil.Big `json:"value"`
	Proof []string     `json:"proof"`
}

// Transaction represents a transaction that will serialize to the RPC representation of a transaction
type Transaction struct {
	BlockHash           *common.Hash             `json:"blockHash"`
	BlockNumber         *hexutil.Big             `json:"blockNumber"`
	From                common.MixedcaseAddress  `json:"from"`
	Gas                 hexutil.Uint64           `json:"gas"`
	GasPrice            *hexutil.Big             `json:"gasPrice"`
	GasFeeCap           *hexutil.Big             `json:"maxFeePerGas,omitempty"`
	GasTipCap           *hexutil.Big             `json:"maxPriorityFeePerGas,omitempty"`
	MaxFeePerBlobGas    *hexutil.Big             `json:"maxFeePerBlobGas,omitempty"`
	Hash                common.Hash              `json:"hash"`
	Input               hexutil.Bytes            `json:"input"`
	Nonce               hexutil.Uint64           `json:"nonce"`
	To                  *common.MixedcaseAddress `json:"to"`
	TransactionIndex    *hexutil.Uint64          `json:"transactionIndex"`
	Value               *hexutil.Big             `json:"value"`
	Type                hexutil.Uint64           `json:"type"`
	Accesses            *types.AccessList        `json:"accessList,omitempty"`
	ChainID             *hexutil.Big             `json:"chainId,omitempty"`
	BlobVersionedHashes []common.Hash            `json:"blobVersionedHashes,omitempty"`
	V                   *hexutil.Big             `json:"v"`
	R                   *hexutil.Big             `json:"r"`
	S                   *hexutil.Big             `json:"s"`
	YParity             *hexutil.Uint64          `json:"yParity,omitempty"`
}

func NewTransaction(
	tx models.Transaction,
	receipt models.StorageReceipt,
	config config.Config,
) (*Transaction, error) {
	txHash, err := tx.Hash()
	if err != nil {
		return nil, err
	}

	f, err := tx.From()
	if err != nil {
		return nil, errs.ErrInternal
	}
	from := common.NewMixedcaseAddress(f)

	var to *common.MixedcaseAddress
	if t := tx.To(); t != nil {
		mixedCaseAddress := common.NewMixedcaseAddress(*t)
		to = &mixedCaseAddress
	}

	v, r, s := tx.RawSignatureValues()
	index := uint64(receipt.TransactionIndex)

	result := &Transaction{
		Type:             hexutil.Uint64(tx.Type()),
		From:             from,
		Gas:              hexutil.Uint64(tx.Gas()),
		GasPrice:         (*hexutil.Big)(tx.GasPrice()),
		Hash:             txHash,
		Input:            tx.Data(),
		Nonce:            hexutil.Uint64(tx.Nonce()),
		To:               to,
		Value:            (*hexutil.Big)(tx.Value()),
		V:                (*hexutil.Big)(v),
		R:                (*hexutil.Big)(r),
		S:                (*hexutil.Big)(s),
		BlockHash:        &receipt.BlockHash,
		BlockNumber:      (*hexutil.Big)(receipt.BlockNumber),
		TransactionIndex: (*hexutil.Uint64)(&index),
	}
	chainID := config.EVMNetworkID

	switch tx.Type() {
	case types.LegacyTxType:
		result.ChainID = (*hexutil.Big)(chainID)
	case types.AccessListTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(chainID)
		result.YParity = &yparity
	case types.DynamicFeeTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(chainID)
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// Since BaseFee is `0`, this is the effective gas price
		// the sender is willing to pay.
		result.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
	case types.BlobTxType:
		al := tx.AccessList()
		yparity := hexutil.Uint64(v.Sign())
		result.Accesses = &al
		result.ChainID = (*hexutil.Big)(chainID)
		result.YParity = &yparity
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// Since BaseFee is `0`, this is the effective gas price
		// the sender is willing to pay.
		result.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
		result.MaxFeePerBlobGas = (*hexutil.Big)(tx.BlobGasFeeCap())
		result.BlobVersionedHashes = tx.BlobHashes()
	}

	return result, nil
}

// SignTransactionResult represents a RLP encoded signed transaction.
type SignTransactionResult struct {
	Raw hexutil.Bytes      `json:"raw"`
	Tx  *types.Transaction `json:"tx"`
}

// OverrideAccount indicates the overriding fields of account during the execution
// of a message call.
// Note, state and stateDiff can't be specified at the same time. If state is
// set, message execution will only use the data in the given state. Otherwise
// if statDiff is set, all diff will be applied first and then execute the call
// message.
type OverrideAccount struct {
	Nonce     *hexutil.Uint64              `json:"nonce"`
	Code      *hexutil.Bytes               `json:"code"`
	Balance   **hexutil.Big                `json:"balance"`
	State     *map[common.Hash]common.Hash `json:"state"`
	StateDiff *map[common.Hash]common.Hash `json:"stateDiff"`
}

// StateOverride is the collection of overridden accounts.
type StateOverride map[common.Address]OverrideAccount

// BlockOverrides is a set of header fields to override.
type BlockOverrides struct {
	Number      *hexutil.Big
	Difficulty  *hexutil.Big
	Time        *hexutil.Uint64
	GasLimit    *hexutil.Uint64
	Coinbase    *common.Address
	Random      *common.Hash
	BaseFee     *hexutil.Big
	BlobBaseFee *hexutil.Big
}

type Block struct {
	Number           hexutil.Uint64   `json:"number"`
	Hash             common.Hash      `json:"hash"`
	ParentHash       common.Hash      `json:"parentHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	Sha3Uncles       common.Hash      `json:"sha3Uncles"`
	LogsBloom        hexutil.Bytes    `json:"logsBloom"`
	TransactionsRoot common.Hash      `json:"transactionsRoot"`
	StateRoot        common.Hash      `json:"stateRoot"`
	ReceiptsRoot     common.Hash      `json:"receiptsRoot"`
	Miner            common.Address   `json:"miner"`
	Difficulty       hexutil.Uint64   `json:"difficulty"`
	TotalDifficulty  hexutil.Uint64   `json:"totalDifficulty"`
	ExtraData        hexutil.Bytes    `json:"extraData"`
	Size             hexutil.Uint64   `json:"size"`
	GasLimit         hexutil.Uint64   `json:"gasLimit"`
	GasUsed          hexutil.Uint64   `json:"gasUsed"`
	Timestamp        hexutil.Uint64   `json:"timestamp"`
	Transactions     interface{}      `json:"transactions"`
	Uncles           []common.Hash    `json:"uncles"`
	MixHash          common.Hash      `json:"mixHash"`
	BaseFeePerGas    hexutil.Big      `json:"baseFeePerGas"`
}

type SyncStatus struct {
	StartingBlock hexutil.Uint64 `json:"startingBlock"`
	CurrentBlock  hexutil.Uint64 `json:"currentBlock"`
	HighestBlock  hexutil.Uint64 `json:"highestBlock"`
}
