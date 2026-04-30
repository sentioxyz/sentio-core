package evm

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/goccy/go-json"
)

// ParityTraceAction see: https://github.com/ledgerwatch/erigon/blob/stable/cmd/rpcdaemon/commands/trace_types.go
type ParityTraceAction struct {
	CallType      string          `json:"callType,omitempty"`
	From          *common.Address `json:"from,omitempty"`
	Gas           *hexutil.Big    `json:"gas,omitempty"`
	Input         hexutil.Bytes   `json:"input,omitempty"`
	To            string          `json:"to,omitempty"`
	Value         string          `json:"value,omitempty"`
	Author        *common.Address `json:"author,omitempty"`
	RewardType    string          `json:"rewardType,omitempty"`
	Init          hexutil.Bytes   `json:"init,omitempty"`
	Address       *common.Address `json:"address,omitempty"`
	RefundAddress *common.Address `json:"refundAddress,omitempty"`
	Balance       *hexutil.Big    `json:"balance,omitempty"`
}

type ParityTraceResult struct {
	Address *common.Address `json:"address,omitempty"`
	GasUsed *hexutil.Big    `json:"gasUsed,omitempty"`
	Output  hexutil.Bytes   `json:"output"`
}

type ParityTrace struct {
	Action              ParityTraceAction  `json:"action"`
	BlockHash           common.Hash        `json:"blockHash,omitempty"`
	BlockNumber         uint64             `json:"blockNumber,omitempty"`
	Error               string             `json:"error,omitempty"`
	Result              *ParityTraceResult `json:"result"`
	Subtraces           int                `json:"subtraces"`
	TraceAddress        []int              `json:"traceAddress"`
	TransactionHash     *common.Hash       `json:"transactionHash,omitempty"`
	TransactionPosition uint64             `json:"transactionPosition"`
	Type                string             `json:"type"`
}

type GethTrace struct {
	Type    string          `json:"type,omitempty"`
	From    *common.Address `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Value   string          `json:"value,omitempty"`
	Gas     *hexutil.Big    `json:"gas,omitempty"`
	GasUsed *hexutil.Big    `json:"gasUsed,omitempty"`
	Input   hexutil.Bytes   `json:"input,omitempty"`
	Output  hexutil.Bytes   `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
	Calls   []GethTrace     `json:"calls,omitempty"`
}

type gethTraceJSON struct {
	Type    string          `json:"type,omitempty"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Value   string          `json:"value,omitempty"`
	Gas     string          `json:"gas,omitempty"`
	GasUsed string          `json:"gasUsed,omitempty"`
	Input   hexutil.Bytes   `json:"input,omitempty"`
	Output  hexutil.Bytes   `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
	Calls   []gethTraceJSON `json:"calls,omitempty"`
}

func populateGethTraceFromJSON(t *GethTrace, jt *gethTraceJSON) error {
	t.Type = jt.Type
	if jt.From != "" {
		from := common.HexToAddress(jt.From)
		t.From = &from
	}
	if jt.To != "" {
		t.To = jt.To
	}
	t.Value = jt.Value
	if jt.Gas != "" {
		gas, err := hexutil.DecodeBig(jt.Gas)
		if err != nil {
			return fmt.Errorf("failed to decode gas: %w", err)
		}
		t.Gas = (*hexutil.Big)(gas)
	}
	if jt.GasUsed != "" {
		gasUsed, err := hexutil.DecodeBig(jt.GasUsed)
		if err != nil {
			return fmt.Errorf("failed to decode gasUsed: %w", err)
		}
		t.GasUsed = (*hexutil.Big)(gasUsed)
	}
	t.Input = jt.Input
	t.Output = jt.Output
	t.Error = jt.Error
	for _, call := range jt.Calls {
		var c GethTrace
		if err := populateGethTraceFromJSON(&c, &call); err != nil {
			return err
		}
		t.Calls = append(t.Calls, c)
	}
	return nil
}

func (t *GethTrace) UnmarshalJSON(data []byte) error {
	var jt gethTraceJSON
	if err := json.Unmarshal(data, &jt); err != nil {
		return err
	}
	if err := populateGethTraceFromJSON(t, &jt); err != nil {
		return err
	}
	return nil
}

var gethPrecompiledContractPrefix = "0x000000000000000000000000000000000000"

func gethFilterPrecompileCalls(calls []GethTrace) []GethTrace {
	var result []GethTrace
	for _, call := range calls {
		//develop test case to see what parity does if is legit call to procompiled contract
		if !strings.HasPrefix(call.To, gethPrecompiledContractPrefix) || call.Value != "" {
			result = append(result, call)
		}
	}
	return result
}

const errParityOutOfGas = "Out of gas"
const errGethOutOfGas = "max code size exceeded"
const errParityReverted = "Reverted"
const errGethReverted = "execution reverted"
const gethSelfDestruct = "SELFDESTRUCT"

// GethToParityTrace see: https://github.com/openrelayxyz/plugeth-plugins/blob/master/packages/plugeth-parity/trace.go
func GethToParityTrace(gr *GethTrace, address []int) []*ParityTrace {
	var result []*ParityTrace
	calls := gethFilterPrecompileCalls(gr.Calls)
	addr := make([]int, len(address))
	copy(addr[:], address)
	value := gr.Value
	if value == "" {
		value = "0x0"
	}
	unique := 0
	if gr.Error == errGethReverted {
		unique = 1
	}
	if gr.Type == "CREATE" || gr.Type == "CREATE2" {
		unique = 2
	}
	// if gr.Gas <= gr.GasUsed
	if gr.Error == errGethOutOfGas {
		unique = 3
	}
	if gr.Type == gethSelfDestruct {
		unique = 4
	}
	switch unique {
	case 0:
		result = append(result, &ParityTrace{
			Action: ParityTraceAction{
				CallType: strings.ToLower(gr.Type),
				From:     gr.From,
				Gas:      gr.Gas,
				Input:    gr.Input,
				To:       gr.To,
				Value:    value,
			},
			Result: &ParityTraceResult{
				GasUsed: gr.GasUsed,
				Output:  gr.Output,
			},
			Subtraces:    len(calls),
			TraceAddress: addr,
			Type:         "call",
		})

	case 1:
		result = append(result, &ParityTrace{
			Action: ParityTraceAction{
				CallType: strings.ToLower(gr.Type),
				From:     gr.From,
				Gas:      gr.Gas,
				Input:    gr.Input,
				To:       gr.To,
				Value:    value,
			},
			Error:        errParityReverted,
			Subtraces:    len(calls),
			TraceAddress: addr,
			Type:         "call",
		})

	case 2:
		toAddress := common.HexToAddress(gr.To)
		result = append(result, &ParityTrace{
			Action: ParityTraceAction{
				From:  gr.From,
				Gas:   gr.Gas,
				Init:  gr.Input,
				Value: value,
			},
			Result: &ParityTraceResult{
				Address: &toAddress,
				GasUsed: gr.GasUsed,
			},
			Subtraces:    len(calls),
			TraceAddress: addr,
			Type:         "create",
		})

	case 3:
		result = append(result, &ParityTrace{
			Action: ParityTraceAction{
				From:  gr.From,
				Gas:   gr.Gas,
				Init:  gr.Input,
				Value: value,
			},
			Error:        errParityOutOfGas,
			Subtraces:    len(calls),
			TraceAddress: addr,
			Type:         "call",
		})

	case 4:
		toAddress := common.HexToAddress(gr.To)
		balance := hexutil.MustDecodeBig(value)
		result = append(result, &ParityTrace{
			Action: ParityTraceAction{
				Address:       gr.From,
				Balance:       (*hexutil.Big)(balance),
				RefundAddress: &toAddress,
			},
			Result:       nil,
			Subtraces:    len(calls),
			TraceAddress: addr,
			Type:         "suicide",
		})
	}

	for i, call := range calls {
		if call.Type == "DELEGATECALL" {
			call.Value = gr.Value
		}
		result = append(result, GethToParityTrace(&call, append(address, i))...)
	}
	return result
}

func convertOneParityToGethTrace(pr *ParityTrace) (*GethTrace, error) {
	switch pr.Type {
	case "call":
		var gethErr string
		switch pr.Error {
		case errParityOutOfGas:
			gethErr = errGethOutOfGas
		case errParityReverted:
			gethErr = errParityReverted
		}
		value := pr.Action.Value
		if value == "0x0" && (pr.Action.CallType == "staticcall" || pr.Action.CallType == "delegatecall") {
			value = ""
		}
		gr := &GethTrace{
			Type:  strings.ToUpper(pr.Action.CallType),
			From:  pr.Action.From,
			To:    pr.Action.To,
			Value: value,
			Gas:   pr.Action.Gas,
			Input: pr.Action.Input,
			Error: gethErr,
		}
		if gethErr == "" {
			gr.GasUsed = pr.Result.GasUsed
			gr.Output = pr.Result.Output
		}
		return gr, nil
	case "suicide":
		if pr.Action.RefundAddress == nil {
			return nil, fmt.Errorf("refundAddress must set for suicide trace")
		}
		if pr.Action.Balance == nil {
			return nil, fmt.Errorf("balance must set for suicide trace")
		}
		return &GethTrace{
			Type:    gethSelfDestruct,
			From:    pr.Action.Address,
			To:      strings.ToLower(pr.Action.RefundAddress.String()),
			Value:   pr.Action.Balance.String(),
			Gas:     &hexutil.Big{},
			GasUsed: &hexutil.Big{},
		}, nil
	case "create":
		if pr.Result.Address == nil {
			return nil, fmt.Errorf("address must set for create trace")
		}
		return &GethTrace{
			Type:    "CREATE",
			From:    pr.Action.From,
			Gas:     pr.Action.Gas,
			Input:   pr.Action.Init,
			Value:   pr.Action.Value,
			To:      strings.ToLower(pr.Result.Address.String()),
			GasUsed: pr.Result.GasUsed,
		}, nil
	}
	return nil, fmt.Errorf("unknown trace type %s", pr.Type)
}

// ParityToGethTrace combines one stack of ParityTrace into a nested GethTrace.
// Note that these ParityTrace must belong to one single transaction.
// The conversion also relies on the relative order of ParityTrace passed in.
func ParityToGethTrace(pr []*ParityTrace) (*GethTrace, error) {
	var root GethTrace
	for _, p := range pr {
		gr, err := convertOneParityToGethTrace(p)
		if err != nil {
			return nil, err
		}
		gr.Calls = make([]GethTrace, p.Subtraces)

		curr := &root
		for _, i := range p.TraceAddress {
			if len(curr.Calls) <= i {
				return nil, fmt.Errorf("invalid trace address %v", p.TraceAddress)
			}
			curr = &curr.Calls[i]
		}

		*curr = *gr
	}
	return &root, nil
}

type SimpleTrace struct {
	BlockNumber      uint64
	TransactionIndex uint64
	TraceIndex       uint64
	MethodSig        string
}
