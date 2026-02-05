package abiutil

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"strings"
)

func GetMethodSig(md *abi.Method, withOutput bool) string {
	// although exist `md.Sig`, but it missed the output info
	if !withOutput {
		return md.Sig
	}
	outputs := make([]string, len(md.Outputs))
	for i, output := range md.Outputs {
		outputs[i] = output.Type.String()
	}
	return fmt.Sprintf("%s:(%s)", md.Sig, strings.Join(outputs, ","))
}

func FindMethodBySig(contract *abi.ABI, sig string) *abi.Method {
	withOutput := strings.ContainsRune(sig, ':')
	for _, md := range contract.Methods {
		if GetMethodSig(&md, withOutput) == sig {
			return &md
		}
	}
	return nil
}

func GetEventSig(ev *abi.Event) string {
	// although exist `ev.Sig`, but it missed the indexed info
	inputs := make([]string, len(ev.Inputs))
	for i, input := range ev.Inputs {
		if input.Indexed {
			inputs[i] = "indexed "
		}
		inputs[i] += input.Type.String()
	}
	return fmt.Sprintf("%s(%s)", ev.RawName, strings.Join(inputs, ","))
}

func FindEventBySig(contract *abi.ABI, sig string) *abi.Event {
	for _, ev := range contract.Events {
		if GetEventSig(&ev) == sig {
			return &ev
		}
	}
	return nil
}
