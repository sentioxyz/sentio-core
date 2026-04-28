package ch

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/utils"
)

type SlotBlock interface {
	FromSlot(slot *evm.Slot)
	ToExtendedHeader() evm.ExtendedHeader

	GetBlockIndex() BlockIndex
}

type BlockIndex struct {
	BlockNumber    uint64    `clickhouse:"block_number" number_field:"true"`
	BlockHash      string    `clickhouse:"block_hash" type:"FixedString(66)" index:"bloom_filter"`
	BlockTimestamp time.Time `clickhouse:"block_timestamp"                   index:"minmax"`
}

type Block struct {
	BlockIndex
	ParentHash       string   `clickhouse:"parent_hash"       type:"FixedString(66)"`
	Sha3Hash         string   `clickhouse:"sha3_hash"         type:"FixedString(66)"`
	Miner            string   `clickhouse:"miner"             type:"FixedString(42)"`
	StateRoot        string   `clickhouse:"state_root"        type:"FixedString(66)"`
	TransactionsRoot string   `clickhouse:"transactions_root" type:"FixedString(66)"`
	TransactionCount uint64   `clickhouse:"transaction_count"`
	ReceiptsRoot     string   `clickhouse:"receipts_root"     type:"FixedString(66)"`
	WithdrawalsRoot  *string  `clickhouse:"withdrawals_root"  type:"Nullable(FixedString(66))"`
	LogsBloom        string   `clickhouse:"logs_bloom"`
	Difficulty       int64    `clickhouse:"difficulty"`
	TotalDifficulty  int64    `clickhouse:"total_difficulty"`
	GasLimit         uint64   `clickhouse:"gas_limit"`
	GasUsed          uint64   `clickhouse:"gas_used"`
	ExtraData        string   `clickhouse:"extra_data"`
	MixHash          string   `clickhouse:"mix_hash"          type:"FixedString(66)"`
	Nonce            string   `clickhouse:"nonce"             type:"FixedString(18)"`
	Uncles           []string `clickhouse:"uncles"            type:"Array(FixedString(66))"`
	BaseFeePerGas    *uint64  `clickhouse:"base_fee_per_gas"`
}

func (b *Block) FromSlot(st *evm.Slot) {
	blockIndex := BlockIndex{
		BlockNumber:    uint64(st.GetNumber()),
		BlockHash:      st.Header.Hash.String(),
		BlockTimestamp: st.Header.GetBlockTime(),
	}
	b.BlockIndex = blockIndex
	b.ParentHash = st.Header.ParentHash.String()
	b.Sha3Hash = st.Header.UncleHash.String()
	b.Miner = st.Header.Coinbase.String()
	b.StateRoot = st.Header.Root.String()
	b.TransactionsRoot = st.Header.TxHash.String()
	b.MixHash = st.Header.MixDigest.String()
	b.ReceiptsRoot = st.Header.ReceiptHash.String()
	b.WithdrawalsRoot = utils.NullOrToString(st.Header.WithdrawalsHash)
	b.LogsBloom = hexutil.Encode(st.Header.Bloom.Bytes())
	b.Difficulty = st.Header.Difficulty.Int64()
	b.GasLimit = st.Header.GasLimit
	b.GasUsed = st.Header.GasUsed
	b.ExtraData = hexutil.Encode(st.Header.Extra)
	b.MixHash = st.Header.MixDigest.String()
	b.Nonce = NonceToString(st.Header.Nonce)
	b.Uncles = utils.MapSliceNoError(st.Block.Uncles, common.Hash.String)
	b.TransactionCount = uint64(len(st.Block.Transactions))
	if st.Header.BaseFee != nil {
		baseFee := st.Header.BaseFee.Uint64()
		b.BaseFeePerGas = &baseFee
	}
}

func (b *Block) ToExtendedHeader() evm.ExtendedHeader {
	header := types.Header{
		ParentHash:      common.HexToHash(b.ParentHash),
		UncleHash:       common.HexToHash(b.Sha3Hash),
		Coinbase:        common.HexToAddress(b.Miner),
		Root:            common.HexToHash(b.StateRoot),
		TxHash:          common.HexToHash(b.TransactionsRoot),
		ReceiptHash:     common.HexToHash(b.ReceiptsRoot),
		Bloom:           types.BytesToBloom(hexutil.MustDecode(b.LogsBloom)),
		Difficulty:      big.NewInt(b.Difficulty),
		Number:          big.NewInt(int64(b.BlockNumber)),
		GasLimit:        b.GasLimit,
		GasUsed:         b.GasUsed,
		Time:            uint64(b.BlockTimestamp.Unix()),
		Extra:           hexutil.MustDecode(b.ExtraData),
		MixDigest:       common.Hash{},
		Nonce:           StringToNonce(b.Nonce),
		WithdrawalsHash: utils.NullOrFromString(b.WithdrawalsRoot, common.HexToHash),
		// fields not in clickhouse
		BlobGasUsed:      nil,
		ExcessBlobGas:    nil,
		ParentBeaconRoot: nil,
		RequestsHash:     nil,
		SlotNumber:       nil,
	}
	if b.BaseFeePerGas != nil {
		header.BaseFee = new(big.Int).SetUint64(*b.BaseFeePerGas)
	}
	return evm.ExtendedHeader{
		Header: header,
		Hash:   common.HexToHash(b.BlockHash),
	}
}

func (b *Block) GetBlockIndex() BlockIndex {
	return b.BlockIndex
}

type BlockMoonbeam struct {
	Block
	Author string `clickhouse:"author"`
}

func (b *BlockMoonbeam) FromSlot(st *evm.Slot) {
	b.Block.FromSlot(st)
	b.Author = st.Header.Author
}

func (b *BlockMoonbeam) ToExtendedHeader() evm.ExtendedHeader {
	header := b.Block.ToExtendedHeader()
	header.Author = b.Author
	return header
}

type BlockArbitrum struct {
	Block
	L1BlockNumber uint64   `clickhouse:"l1_block_number"`
	SendCount     *big.Int `clickhouse:"send_count"`
	SendRoot      *string  `clickhouse:"send_root" type:"Nullable(FixedString(66))"`
}

func (b *BlockArbitrum) FromSlot(st *evm.Slot) {
	b.Block.FromSlot(st)
	if st.Header.L1BlockNumber == nil {
		panic(fmt.Errorf("block %d/%s miss l1BlockNumber", st.GetNumber(), st.GetHash()))
	}
	b.L1BlockNumber = st.Header.L1BlockNumber.ToInt().Uint64()
	if st.Header.SendCount == nil {
		b.SendCount = big.NewInt(0)
	} else {
		b.SendCount = st.Header.SendCount.ToInt()
	}
	b.SendRoot = utils.NullOrToString(st.Header.SendRoot)
}

func (b *BlockArbitrum) ToExtendedHeader() evm.ExtendedHeader {
	header := b.Block.ToExtendedHeader()
	l1Block := hexutil.Big(*big.NewInt(int64(b.L1BlockNumber)))
	header.L1BlockNumber = &l1Block
	header.SendRoot = utils.NullOrFromString(b.SendRoot, common.HexToHash)
	header.SendCount = (*hexutil.Big)(b.SendCount)
	return header
}

type BlockCronosZkevm struct {
	Block
	L1BatchNumber    *big.Int `clickhouse:"l1_batch_number"`
	L1BatchTimestamp *big.Int `clickhouse:"l1_batch_timestamp"`
	SealFields       []string `clickhouse:"seal_fields"`
}

func (b *BlockCronosZkevm) FromSlot(st *evm.Slot) {
	b.Block.FromSlot(st)
	if st.Header.L1BatchNumber != nil {
		b.L1BatchNumber = st.Header.L1BatchNumber.ToInt()
	}
	if st.Header.L1BatchTimestamp != nil {
		b.L1BatchTimestamp = st.Header.L1BatchTimestamp.ToInt()
	}
	b.SealFields = st.Header.SealFields
}

func (b *BlockCronosZkevm) ToExtendedHeader() evm.ExtendedHeader {
	header := b.Block.ToExtendedHeader()
	if b.L1BatchNumber != nil {
		header.L1BatchNumber = (*hexutil.Big)(b.L1BatchNumber)
	}
	if b.L1BatchTimestamp != nil {
		header.L1BatchTimestamp = (*hexutil.Big)(b.L1BatchTimestamp)
	}
	header.SealFields = b.SealFields
	return header
}
