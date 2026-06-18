package standard

import (
	"bytes"

	"sentioxyz/sentio-core/common/utils"
)

func AdjustAddress(address string) string {
	return utils.Select(address == "" || address == "*", "", address)
}

func AdjustEndBlock(endBlock uint64) *uint64 {
	return utils.Select(endBlock == 0, nil, &endBlock)
}

func BuildDataSource(chainType, chainID, srcType, address string) string {
	var b bytes.Buffer
	b.WriteString(chainType)
	b.WriteString(":")
	b.WriteString(chainID)
	b.WriteString(":")
	b.WriteString(srcType)
	if address != "" {
		b.WriteString(":")
		b.WriteString(address)
	}
	return b.String()
}
