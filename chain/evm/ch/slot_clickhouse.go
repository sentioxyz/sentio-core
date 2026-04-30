package ch

import (
	"context"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"
)

type ClickhouseSchemaMgr struct {
	tablesMeta         clickhouse.TablesMeta
	ethVarCtrl         EthVariationCtrl
	convertConcurrency uint
}

func NewClickhouseSchemaMgr(
	chainID string,
	ctrl chx.Controller,
	tablePrefix string,
	blockPartitionSize uint64,
	convertConcurrency uint,
) *ClickhouseSchemaMgr {
	ethVarCtrl := NewEthVarCtrl(chainID, ctrl, tablePrefix)
	return &ClickhouseSchemaMgr{
		ethVarCtrl:         ethVarCtrl,
		tablesMeta:         ethVarCtrl.BuildTablesMeta(blockPartitionSize),
		convertConcurrency: convertConcurrency,
	}
}

func (m *ClickhouseSchemaMgr) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, slot *evm.Slot) (clickhouse.Chunk, error) {
	return m.ethVarCtrl.Convert(slot)
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
