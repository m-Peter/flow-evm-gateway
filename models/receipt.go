package models

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/onflow/flow-go/fvm/evm/types"

	"github.com/onflow/cadence"
	"github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/common/hexutil"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/rlp"
)

// StorageReceipt is a receipt representation for storage.
//
// This struct copies the geth.Receipt type found here: https://github.com/ethereum/go-ethereum/blob/9bbb9df18549d6f81c3d1f4fc6c65f71bc92490d/core/types/receipt.go#L52
// the reason is if we use geth.Receipt some values will be skipped when RLP encoding which is because
// geth node has the data locally, but we don't in evm gateway, so we can not reproduce those values
// and we need to store them
type StorageReceipt struct {
	Type              uint8
	PostState         []byte
	Status            uint64
	CumulativeGasUsed uint64
	// todo we could skip bloom to optimize storage and dynamically recalculate it
	Bloom             gethTypes.Bloom
	Logs              []*gethTypes.Log
	TxHash            common.Hash
	ContractAddress   common.Address
	GasUsed           uint64
	EffectiveGasPrice *big.Int
	BlobGasUsed       uint64
	BlobGasPrice      *big.Int
	BlockHash         common.Hash
	BlockNumber       *big.Int
	TransactionIndex  uint
}

// decodeReceipt takes a cadence event for transaction executed and decodes it into the receipt.
func decodeReceipt(event cadence.Event) (*gethTypes.Receipt, error) {
	tx, err := types.DecodeTransactionEventPayload(event)
	if err != nil {
		return nil, fmt.Errorf("failed to cadence decode receipt: %w", err)
	}

	encLogs, err := hex.DecodeString(tx.Logs)
	if err != nil {
		return nil, fmt.Errorf("failed to hex decode receipt: %w", err)
	}

	var logs []*gethTypes.Log
	if len(encLogs) > 0 {
		err = rlp.Decode(bytes.NewReader(encLogs), &logs)
		if err != nil {
			return nil, fmt.Errorf("failed to rlp decode receipt: %w", err)
		}
	}

	t, err := decodeTransaction(event)
	if err != nil {
		return nil, err
	}

	receipt := &gethTypes.Receipt{
		BlockNumber:       big.NewInt(int64(tx.BlockHeight)),
		Type:              tx.TransactionType,
		Logs:              logs,
		TxHash:            common.HexToHash(tx.Hash),
		ContractAddress:   common.HexToAddress(tx.ContractAddress),
		GasUsed:           tx.GasConsumed,
		CumulativeGasUsed: tx.GasConsumed, // todo use cumulative after added to the tx result
		EffectiveGasPrice: t.GasPrice(),   // since there's no base fee we can always use gas price
		TransactionIndex:  uint(tx.Index),
		BlockHash:         common.HexToHash(tx.BlockHash),
	}

	if tx.ErrorCode == uint16(types.ErrCodeNoError) {
		receipt.Status = gethTypes.ReceiptStatusSuccessful
	} else {
		receipt.Status = gethTypes.ReceiptStatusFailed
	}

	receipt.Bloom = gethTypes.CreateBloom([]*gethTypes.Receipt{receipt})

	return receipt, nil
}

// MarshalReceipt takes a receipt and its associated transaction,
// and marshals the receipt to the proper structure needed by
// eth_getTransactionReceipt.
func MarshalReceipt(
	receipt *gethTypes.Receipt,
	tx Transaction,
) (map[string]interface{}, error) {
	from, err := tx.From()
	if err != nil {
		return map[string]interface{}{}, err
	}

	txHash, err := tx.Hash()
	if err != nil {
		return map[string]interface{}{}, err
	}

	fields := map[string]interface{}{
		"blockHash":         receipt.BlockHash,
		"blockNumber":       hexutil.Uint64(receipt.BlockNumber.Uint64()),
		"transactionHash":   txHash,
		"transactionIndex":  hexutil.Uint64(receipt.TransactionIndex),
		"from":              from.Hex(),
		"to":                nil,
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
		"type":              hexutil.Uint(tx.Type()),
		"effectiveGasPrice": (*hexutil.Big)(receipt.EffectiveGasPrice),
	}

	if tx.To() != nil {
		fields["to"] = tx.To().Hex()
	}

	fields["status"] = hexutil.Uint(receipt.Status)

	if receipt.Logs == nil {
		fields["logs"] = []*gethTypes.Log{}
	}

	if tx.Type() == gethTypes.BlobTxType {
		fields["blobGasUsed"] = hexutil.Uint64(receipt.BlobGasUsed)
		fields["blobGasPrice"] = (*hexutil.Big)(receipt.BlobGasPrice)
	}

	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress.Hex()
	}

	return fields, nil
}
