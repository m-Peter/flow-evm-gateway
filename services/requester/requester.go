package requester

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/rs/zerolog"

	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk/access"

	gethCore "github.com/ethereum/go-ethereum/core"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	evmTypes "github.com/onflow/flow-go/fvm/evm/types"
)

var (
	//go:embed cadence/call.cdc
	callScript []byte

	//go:embed cadence/run.cdc
	runTxScript []byte

	//go:embed cadence/get_balance.cdc
	getBalanceScript []byte

	//go:embed cadence/create_coa.cdc
	createCOAScript []byte

	//go:embed cadence/estimate_gas.cdc
	estimateGasScript []byte

	byteArrayType = cadence.NewVariableSizedArrayType(cadence.UInt8Type)
	addressType   = cadence.NewConstantSizedArrayType(
		common.AddressLength,
		cadence.UInt8Type,
	)
)

const minFlowBalance = 2
const coaFundingBalance = minFlowBalance - 1

type Requester interface {
	// SendRawTransaction will submit signed transaction data to the network.
	// The submitted EVM transaction hash is returned.
	SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error)

	// GetBalance returns the amount of wei for the given address in the state of the
	// given block height.
	// todo in future this should be deprecated for local data
	GetBalance(ctx context.Context, address common.Address, height uint64) (*big.Int, error)

	// Call executes the given signed transaction data on the state for the given block number.
	// Note, this function doesn't make and changes in the state/blockchain and is
	// useful to execute and retrieve values.
	Call(ctx context.Context, address common.Address, data []byte) ([]byte, error)

	// EstimateGas executes the given signed transaction data on the state.
	// Note, this function doesn't make any changes in the state/blockchain and is
	// useful to executed and retrieve the gas consumption and possible failures.
	EstimateGas(ctx context.Context, data []byte) (uint64, error)
}

var _ Requester = &EVM{}

type EVM struct {
	logger  zerolog.Logger
	client  access.Client
	address flow.Address
	signer  crypto.Signer
	network string // todo change the type to FVM type once the "previewnet" is added
}

func NewEVM(
	client access.Client,
	address flow.Address,
	signer crypto.Signer,
	network string,
	createCOA bool,
	logger zerolog.Logger,
) (*EVM, error) {
	logger = logger.With().Str("component", "requester").Logger()
	// check that the address stores already created COA resource in the "evm" storage path.
	// if it doesn't check if the auto-creation boolean is true and if so create it
	// otherwise fail. COA resource is required by the EVM requester to be able to submit transactions.
	acc, err := client.GetAccount(context.Background(), address)
	if err != nil {
		return nil, fmt.Errorf(
			"could not fetch the configured COA account: %s make sure it exists: %w",
			address.String(),
			err,
		)
	}

	if acc.Balance < minFlowBalance {
		return nil, fmt.Errorf(
			"COA account must be funded with at least %d Flow, but has balance of: %d",
			minFlowBalance,
			acc.Balance,
		)
	}

	evm := &EVM{
		client:  client,
		address: address,
		signer:  signer,
		logger:  logger,
		network: network,
	}

	// create COA on the account
	// todo improve this to first check if coa exists and only if it doesn't create it, if it doesn't and the flag is false return an error
	if createCOA {
		// we ignore errors for now since creation of already existing COA resource will fail, which is fine for now
		id, err := evm.signAndSend(
			context.Background(),
			evm.replaceAddresses(createCOAScript),
			cadence.UFix64(coaFundingBalance),
		)
		logger.Info().Err(err).Str("id", id.String()).Msg("COA resource auto-created")
	}

	return evm, nil
}

func (e *EVM) SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error) {
	e.logger.Debug().
		Str("data", fmt.Sprintf("%x", data)).
		Msg("send raw transaction")

	tx := &types.Transaction{}
	err := tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(data),
			uint64(len(data)),
		),
	)
	if err != nil {
		return common.Hash{}, err
	}

	// todo make sure the gas price is not bellow the configured gas price
	script := e.replaceAddresses(runTxScript)
	flowID, err := e.signAndSend(
		ctx,
		script,
		cadenceArrayFromBytes(data),
	)
	if err != nil {
		return common.Hash{}, err
	}

	var to string
	if tx.To() != nil {
		to = tx.To().String()
	}
	e.logger.Info().
		Str("evm ID", tx.Hash().Hex()).
		Str("flow ID", flowID.Hex()).
		Str("to", to).
		Str("value", tx.Value().String()).
		Str("data", fmt.Sprintf("%x", tx.Data())).
		Msg("raw transaction sent")

	return tx.Hash(), nil
}

// signAndSend creates a flow transaction from the provided script with the arguments and signs it with the
// configured COA account and then submits it to the network.
func (e *EVM) signAndSend(ctx context.Context, script []byte, args ...cadence.Value) (flow.Identifier, error) {
	latestBlock, err := e.client.GetLatestBlock(ctx, true)
	if err != nil {
		return flow.EmptyID, fmt.Errorf("failed to get latest flow block: %w", err)
	}

	index, seqNum, err := e.getSignerNetworkInfo(ctx)
	if err != nil {
		return flow.EmptyID, fmt.Errorf("failed to get signer info: %w", err)
	}

	flowTx := flow.NewTransaction().
		SetScript(script).
		SetProposalKey(e.address, index, seqNum).
		SetReferenceBlockID(latestBlock.ID).
		SetPayer(e.address).
		AddAuthorizer(e.address)

	for _, arg := range args {
		if err = flowTx.AddArgument(arg); err != nil {
			return flow.EmptyID, fmt.Errorf("failed to add argument: %w", err)
		}
	}

	if err = flowTx.SignEnvelope(e.address, index, e.signer); err != nil {
		return flow.EmptyID, fmt.Errorf("failed to sign envelope: %w", err)
	}

	err = e.client.SendTransaction(ctx, *flowTx)
	if err != nil {
		return flow.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	// todo should we wait for the transaction result?
	// we should handle a case where flow transaction is failed but we will get a result back, it would only be failed,
	// but there is no evm transaction. So if we submit an evm tx and get back an ID and then we wait for receipt
	// we would never get it, but this failure of sending flow transaction could somehow help with this case

	// this is only used for debugging purposes
	go func(tx *flow.Transaction) {
		res, _ := e.client.GetTransactionResult(context.Background(), flowTx.ID())
		if res.Error != nil {
			e.logger.Error().
				Str("flow-id", flowTx.ID().String()).
				Err(res.Error).
				Msg("flow transaction failed to execute")
			return
		}

		e.logger.Debug().
			Str("flow-id", flowTx.ID().String()).
			Uint64("cadence-height", res.BlockHeight).
			Str("events", fmt.Sprintf("%v", res.Events)).
			Str("script", string(flowTx.Script)).
			Msg("flow transaction executed successfully")
	}(flowTx)

	return flowTx.ID(), nil
}

func (e *EVM) GetBalance(ctx context.Context, address common.Address, height uint64) (*big.Int, error) {
	// todo make sure provided height is used
	addr := cadenceArrayFromBytes(address.Bytes()).WithType(addressType)

	val, err := e.client.ExecuteScriptAtLatestBlock(
		ctx,
		e.replaceAddresses(getBalanceScript),
		[]cadence.Value{addr},
	)
	if err != nil {
		return nil, err
	}

	e.logger.Info().Str("address", address.String()).Msg("get balance")

	// sanity check, should never occur
	if _, ok := val.(cadence.UInt); !ok {
		e.logger.Panic().Msg(fmt.Sprintf("failed to convert balance %v to UInt", val))
	}

	return val.(cadence.UInt).ToGoValue().(*big.Int), nil
}

func (e *EVM) Call(ctx context.Context, address common.Address, data []byte) ([]byte, error) {
	// todo make "to" address optional, this can be used for contract deployment simulations
	txData := cadenceArrayFromBytes(data).WithType(byteArrayType)
	toAddress := cadenceArrayFromBytes(address.Bytes()).WithType(addressType)

	e.logger.Debug().
		Str("address", address.Hex()).
		Str("data", fmt.Sprintf("%x", data)).
		Msg("call")

	value, err := e.client.ExecuteScriptAtLatestBlock(
		ctx,
		e.replaceAddresses(callScript),
		[]cadence.Value{txData, toAddress},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	e.logger.Info().
		Str("address", address.Hex()).
		Str("data", fmt.Sprintf("%x", data)).
		Str("result", value.String()).
		Msg("call executed")

	return bytesFromCadenceArray(value)
}

func (e *EVM) EstimateGas(ctx context.Context, data []byte) (uint64, error) {
	e.logger.Debug().
		Str("data", fmt.Sprintf("%x", data)).
		Msg("estimate gas")

	txData := cadenceArrayFromBytes(data).WithType(byteArrayType)

	value, err := e.client.ExecuteScriptAtLatestBlock(
		ctx,
		e.replaceAddresses(estimateGasScript),
		[]cadence.Value{txData},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to execute script: %w", err)
	}

	// sanity check, should never occur
	// TODO(m-Peter): Consider adding a decoder for EVM.Result struct
	// to a Go value/type.
	if _, ok := value.(cadence.Array); !ok {
		e.logger.Panic().Msg(fmt.Sprintf("failed to convert value to array: %v", value))
	}

	result := value.(cadence.Array)
	errorCode := result.Values[0].ToGoValue().(uint64)

	if errorCode != 0 {
		return 0, getErrorForCode(errorCode)
	}

	gasUsed := result.Values[1].ToGoValue().(uint64)
	return gasUsed, nil
}

// getSignerNetworkInfo loads the signer account from network and returns key index and sequence number
func (e *EVM) getSignerNetworkInfo(ctx context.Context) (int, uint64, error) {
	account, err := e.client.GetAccount(ctx, e.address)
	if err != nil {
		return 0, 0, err
	}

	signerPub := e.signer.PublicKey()
	for _, k := range account.Keys {
		if k.PublicKey.Equals(signerPub) {
			return k.Index, k.SequenceNumber, nil
		}
	}

	return 0, 0, fmt.Errorf("provided account address and signer keys do not match")
}

// replaceAddresses replace the addresses based on the network
func (e *EVM) replaceAddresses(script []byte) []byte {
	// todo use the FVM configured addresses once the previewnet is added, this should all be replaced once flow-go is updated
	addresses := map[string]map[string]string{
		"previewnet": {
			"EVM":           "0xb6763b4399a888c8",
			"FungibleToken": "0xa0225e7000ac82a9",
			"FlowToken":     "0x4445e7ad11568276",
		},
		"emulator": {
			"EVM":           "0xf8d6e0586b0a20c7",
			"FungibleToken": "0xee82856bf20e2aa6",
			"FlowToken":     "0x0ae53cb6e3f42a79",
		},
	}

	s := string(script)
	// iterate over all the import name and address pairs and replace them in script
	for imp, addr := range addresses[e.network] {
		s = strings.ReplaceAll(s,
			fmt.Sprintf("import %s", imp),
			fmt.Sprintf("import %s from %s", imp, addr),
		)
	}

	// also replace COA address if used (in scripts)
	s = strings.ReplaceAll(s, "0xCOA", e.address.HexWithPrefix())

	return []byte(s)
}

func cadenceArrayFromBytes(input []byte) cadence.Array {
	values := make([]cadence.Value, 0)
	for _, element := range input {
		values = append(values, cadence.UInt8(element))
	}

	return cadence.NewArray(values)
}

func bytesFromCadenceArray(value cadence.Value) ([]byte, error) {
	arr, ok := value.(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("cadence value is not of array type, can not conver to byte array")
	}

	res := make([]byte, len(arr.Values))
	for i, x := range arr.Values {
		res[i] = x.ToGoValue().(byte)
	}

	return res, nil
}

// TODO(m-Peter): Consider moving this to flow-go repository
func getErrorForCode(errorCode uint64) error {
	switch evmTypes.ErrorCode(errorCode) {
	case evmTypes.ValidationErrCodeInvalidBalance:
		return evmTypes.ErrInvalidBalance
	case evmTypes.ValidationErrCodeInsufficientComputation:
		return evmTypes.ErrInsufficientComputation
	case evmTypes.ValidationErrCodeUnAuthroizedMethodCall:
		return evmTypes.ErrUnAuthroizedMethodCall
	case evmTypes.ValidationErrCodeWithdrawBalanceRounding:
		return evmTypes.ErrWithdrawBalanceRounding
	case evmTypes.ValidationErrCodeGasUintOverflow:
		return gethVM.ErrGasUintOverflow
	case evmTypes.ValidationErrCodeNonceTooLow:
		return gethCore.ErrNonceTooLow
	case evmTypes.ValidationErrCodeNonceTooHigh:
		return gethCore.ErrNonceTooHigh
	case evmTypes.ValidationErrCodeNonceMax:
		return gethCore.ErrNonceMax
	case evmTypes.ValidationErrCodeGasLimitReached:
		return gethCore.ErrGasLimitReached
	case evmTypes.ValidationErrCodeInsufficientFundsForTransfer:
		return gethCore.ErrInsufficientFundsForTransfer
	case evmTypes.ValidationErrCodeMaxInitCodeSizeExceeded:
		return gethCore.ErrMaxInitCodeSizeExceeded
	case evmTypes.ValidationErrCodeInsufficientFunds:
		return gethCore.ErrInsufficientFunds
	case evmTypes.ValidationErrCodeIntrinsicGas:
		return gethCore.ErrIntrinsicGas
	case evmTypes.ValidationErrCodeTxTypeNotSupported:
		return gethCore.ErrTxTypeNotSupported
	case evmTypes.ValidationErrCodeTipAboveFeeCap:
		return gethCore.ErrTipAboveFeeCap
	case evmTypes.ValidationErrCodeTipVeryHigh:
		return gethCore.ErrTipVeryHigh
	case evmTypes.ValidationErrCodeFeeCapVeryHigh:
		return gethCore.ErrFeeCapVeryHigh
	case evmTypes.ValidationErrCodeFeeCapTooLow:
		return gethCore.ErrFeeCapTooLow
	case evmTypes.ValidationErrCodeSenderNoEOA:
		return gethCore.ErrSenderNoEOA
	case evmTypes.ValidationErrCodeBlobFeeCapTooLow:
		return gethCore.ErrBlobFeeCapTooLow
	case evmTypes.ExecutionErrCodeOutOfGas:
		return gethVM.ErrOutOfGas
	case evmTypes.ExecutionErrCodeCodeStoreOutOfGas:
		return gethVM.ErrCodeStoreOutOfGas
	case evmTypes.ExecutionErrCodeDepth:
		return gethVM.ErrDepth
	case evmTypes.ExecutionErrCodeInsufficientBalance:
		return gethVM.ErrInsufficientBalance
	case evmTypes.ExecutionErrCodeContractAddressCollision:
		return gethVM.ErrContractAddressCollision
	case evmTypes.ExecutionErrCodeExecutionReverted:
		return gethVM.ErrExecutionReverted
	case evmTypes.ExecutionErrCodeMaxInitCodeSizeExceeded:
		return gethVM.ErrMaxInitCodeSizeExceeded
	case evmTypes.ExecutionErrCodeMaxCodeSizeExceeded:
		return gethVM.ErrMaxCodeSizeExceeded
	case evmTypes.ExecutionErrCodeInvalidJump:
		return gethVM.ErrInvalidJump
	case evmTypes.ExecutionErrCodeWriteProtection:
		return gethVM.ErrWriteProtection
	case evmTypes.ExecutionErrCodeReturnDataOutOfBounds:
		return gethVM.ErrReturnDataOutOfBounds
	case evmTypes.ExecutionErrCodeGasUintOverflow:
		return gethVM.ErrGasUintOverflow
	case evmTypes.ExecutionErrCodeInvalidCode:
		return gethVM.ErrInvalidCode
	case evmTypes.ExecutionErrCodeNonceUintOverflow:
		return gethVM.ErrNonceUintOverflow
	case evmTypes.ValidationErrCodeMisc:
		return fmt.Errorf("validation error: %d", errorCode)
	case evmTypes.ExecutionErrCodeMisc:
		return fmt.Errorf("execution error: %d", errorCode)
	}

	return fmt.Errorf("unknown error code: %d", errorCode)
}