package ethereum

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
)

type EthCallClient interface {
	CallContract(ctx context.Context, callMsg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

// see:
// - https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/runtime/runtime_adapter.rs#L37
// - https://github.com/graphprotocol/graph-node/blob/v0.30.0/chain/ethereum/src/ethereum_adapter.rs#L461
// In graph-node v0.30, gas for each eth_call request always use 50000000. Since v0.31 it can be removed by env.
const ethCallGas = 50000000

// see: https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/ethereum_adapter.rs#L480
var ethExecutionErrors = []string{
	// The "revert" substring covers a few known error messages, including:
	// Hardhat: "error: transaction reverted",
	// Ganache and Moonbeam: "vm exception while processing transaction: revert",
	// Geth: "execution reverted"
	// And others.
	"revert",
	"invalid jump destination",
	"invalid opcode",
	// Ethereum says 1024 is the stack sizes limit, so this is deterministic.
	"stack limit reached 1024",
	// See f0af4ab0-6b7c-4b68-9141-5b79346a5f61 for why the gas limit is considered deterministic.
	"out of gas",
}

func isRevertErr(err error) bool {
	// see: https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/ethereum_adapter.rs#L518
	// TODO check for Parity revert.
	//      https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/ethereum_adapter.rs#L526
	return utils.ContainsAnyIgnoreCase(err.Error(), ethExecutionErrors)
}

const (
	SendEthCallRequestErrMsg = "send eth_call request failed"

	normalErrMsg = "eth_call failed"
	revertErrMsg = "eth_call failed, SmartContract.tryCall in graph-ts will handle this case, " +
		"will tell user the request is reverted"
)

var ErrEthCallDataFormatErr = fmt.Errorf("eth call data format error")

func EthCall(
	ctx context.Context,
	logger *log.SentioLogger,
	client EthCallClient,
	contractAddr []byte,
	methodABI *abi.Method,
	methodParams *wasm.ObjectArray[*Value],
	blockNumber uint64,
) (*wasm.ObjectArray[*Value], error) {
	// get `to` of eth_call
	contractAddress := common.BytesToAddress(contractAddr)

	// pack `data` of eth_call from call.FunctionSignature + call.FunctionParams
	params := make([]any, len(methodABI.Inputs))
	for i, input := range methodABI.Inputs {
		params[i] = methodParams.Data[i].ToGoType(input.Type)
	}
	inputs, err := methodABI.Inputs.Pack(params...)
	if err != nil {
		err = errors.Wrapf(ErrEthCallDataFormatErr, "pack input data of eth_call failed: %v", err)
		logger.With("inputsABI", methodABI.Inputs, "params", params).Errore(err, normalErrMsg)
		return nil, err
	}
	inputs = append(methodABI.ID, inputs...)
	logger = logger.With("packedInput", fmt.Sprintf("0x%x", inputs))

	// send the eth_call request and get the response.
	// refer to the parameters of graph-node sending eth_call:
	// https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/ethereum_adapter.rs#L442
	callMsg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: inputs,
		Gas:  ethCallGas,
	}
	respData, callErr := client.CallContract(ctx, callMsg, new(big.Int).SetUint64(blockNumber))
	if callErr != nil {
		err = fmt.Errorf("%s: %w", SendEthCallRequestErrMsg, callErr)
		if isRevertErr(callErr) {
			logger.Warne(err, revertErrMsg)
			return nil, nil
		}
		logger.Errorfe(err, normalErrMsg)
		return nil, err
	}
	logger = logger.With("respData", fmt.Sprintf("0x%x", respData))

	// unpack outputs from response data
	unpackedRespData, unpackErr := methodABI.Outputs.UnpackValues(respData)
	if unpackErr != nil {
		// wrap the error with ErrRevert
		// https://github.com/graphprotocol/graph-node/blob/v0.31.0/chain/ethereum/src/ethereum_adapter.rs#L1239
		err = errors.Wrapf(unpackErr, "unpack output data of eth_call failed")
		logger.With("outputsABI", methodABI.Outputs).Warne(err, revertErrMsg)
		return nil, nil
	}

	// build return value and return them
	ret := &wasm.ObjectArray[*Value]{
		Data: make([]*Value, len(methodABI.Outputs)),
	}
	for i, output := range methodABI.Outputs {
		ret.Data[i] = &Value{}
		ret.Data[i].FromGoType(unpackedRespData[i], output.Type)
	}
	logger.Debug("eth_call succeed")
	return ret, nil
}
