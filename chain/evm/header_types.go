package evm

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/common/utils"
)

// ExtendedHeader is a copy of types.Header that is compatible with any L1 or L2 evm-based
// chains, with additional fields.  Using ExtendedHeader is preferred over types.Header.
type ExtendedHeader struct {
	// The original header, same as Ethereum.
	types.Header `msgpack:",inline"`

	// L2 chains do not necessarily observe the same hash method as Ethereum.
	// We may or may not derive a correct hash value from Header, therefore we attempt not to compute
	// it but to store it.
	Hash common.Hash `msgpack:"hash"`

	// Size and TotalDifficulty are not included by types.Header but somehow returned by eth_getBlockByNumber.
	Size            *hexutil.Uint64 `msgpack:"size,omitempty"`
	TotalDifficulty *hexutil.Big    `msgpack:"totalDifficulty,omitempty"`

	// For moonbeam.
	Author string `msgpack:"author,omitempty"`

	// For arbitrum.
	L1BlockNumber *hexutil.Big `msgpack:"l1BlockNumber,omitempty"`
	SendCount     *hexutil.Big `msgpack:"sendCount,omitempty"`
	SendRoot      *common.Hash `msgpack:"sendRoot,omitempty"`

	// For cronos zkevm.
	L1BatchNumber    *hexutil.Big `msgpack:"l1BatchNumber,omitempty"`
	L1BatchTimestamp *hexutil.Big `msgpack:"l1BatchTimestamp,omitempty"`
	SealFields       []string     `msgpack:"sealFields,omitempty"`

	// For base
	RequestsHash *common.Hash `msgpack:"requestsHash"`
}

func NewExtendedHeader(h types.Header, hash common.Hash) *ExtendedHeader {
	return &ExtendedHeader{
		Header: h,
		Hash:   hash,
	}
}

type ExtendedHeaderJSON struct {
	ParentHash            common.Hash      `json:"parentHash"`
	UncleHash             common.Hash      `json:"sha3Uncles"`
	Coinbase              common.Address   `json:"miner"`
	Root                  common.Hash      `json:"stateRoot"`
	TxHash                common.Hash      `json:"transactionsRoot"`
	ReceiptHash           common.Hash      `json:"receiptsRoot"`
	Bloom                 types.Bloom      `json:"logsBloom"`
	Difficulty            *hexutil.Big     `json:"difficulty"`
	Number                *hexutil.Big     `json:"number"`
	GasLimit              hexutil.Uint64   `json:"gasLimit"`
	GasUsed               hexutil.Uint64   `json:"gasUsed"`
	Time                  hexutil.Uint64   `json:"timestamp"`
	Extra                 hexutil.Bytes    `json:"extraData"`
	MixDigest             common.Hash      `json:"mixHash"`
	Nonce                 types.BlockNonce `json:"nonce"`
	BaseFee               *hexutil.Big     `json:"baseFeePerGas"`
	WithdrawalsRoot       *common.Hash     `json:"withdrawalsRoot"`
	BlobGasUsed           *hexutil.Uint64  `json:"blobGasUsed"`
	ExcessBlobGas         *hexutil.Uint64  `json:"excessBlobGas"`
	ParentBeaconBlockRoot *common.Hash     `json:"parentBeaconBlockRoot"`
	RequestsRoot          *common.Hash     `json:"requestsRoot"`

	Hash            common.Hash     `json:"hash"`
	Author          string          `json:"author,omitempty"`
	Size            *hexutil.Uint64 `json:"size,omitempty"`
	TotalDifficulty *hexutil.Big    `json:"totalDifficulty,omitempty"`

	L1BlockNumber *hexutil.Big `json:"l1BlockNumber,omitempty"`
	SendCount     *hexutil.Big `json:"sendCount,omitempty"`
	SendRoot      *common.Hash `json:"sendRoot,omitempty"`

	L1BatchNumber    *hexutil.Big `json:"l1BatchNumber,omitempty"`
	L1BatchTimestamp *hexutil.Big `json:"l1BatchTimestamp,omitempty"`
	SealFields       []string     `json:"sealFields,omitempty"`

	RequestsHash *common.Hash `json:"requestsHash,omitempty"`
}

func (h *ExtendedHeader) MakeJSON() *ExtendedHeaderJSON {
	var enc ExtendedHeaderJSON
	enc.ParentHash = h.ParentHash
	enc.UncleHash = h.UncleHash
	enc.Coinbase = h.Coinbase
	enc.Root = h.Root
	enc.TxHash = h.TxHash
	enc.ReceiptHash = h.ReceiptHash
	enc.Bloom = h.Bloom
	enc.Difficulty = (*hexutil.Big)(h.Difficulty)
	enc.Number = (*hexutil.Big)(h.Number)
	enc.GasLimit = hexutil.Uint64(h.GasLimit)
	enc.GasUsed = hexutil.Uint64(h.GasUsed)
	enc.Time = hexutil.Uint64(h.Time)
	enc.Extra = h.Extra
	enc.MixDigest = h.MixDigest
	enc.Nonce = h.Nonce
	enc.BaseFee = (*hexutil.Big)(h.BaseFee)
	enc.WithdrawalsRoot = h.WithdrawalsHash
	enc.BlobGasUsed = (*hexutil.Uint64)(h.BlobGasUsed)
	enc.ExcessBlobGas = (*hexutil.Uint64)(h.ExcessBlobGas)
	enc.ParentBeaconBlockRoot = h.Header.ParentBeaconRoot
	enc.RequestsRoot = h.Header.RequestsHash
	enc.Hash = h.Hash
	enc.Author = h.Author
	enc.Size = h.Size
	enc.TotalDifficulty = h.TotalDifficulty
	enc.L1BlockNumber = h.L1BlockNumber
	enc.SendCount = h.SendCount
	enc.SendRoot = h.SendRoot
	enc.L1BatchNumber = h.L1BatchNumber
	enc.L1BatchTimestamp = h.L1BatchTimestamp
	enc.SealFields = h.SealFields
	enc.RequestsHash = h.RequestsHash
	return &enc
}

func (h *ExtendedHeader) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.MakeJSON())
}

func (h *ExtendedHeader) UnmarshalJSON(input []byte) error {
	var dec ExtendedHeaderJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	h.ParentHash = dec.ParentHash
	h.UncleHash = dec.UncleHash
	h.Coinbase = dec.Coinbase
	h.Root = dec.Root
	h.TxHash = dec.TxHash
	h.ReceiptHash = dec.ReceiptHash
	h.Bloom = dec.Bloom
	h.Difficulty = (*big.Int)(dec.Difficulty)
	h.Number = (*big.Int)(dec.Number)
	h.GasLimit = uint64(dec.GasLimit)
	h.GasUsed = uint64(dec.GasUsed)
	h.Time = uint64(dec.Time)
	h.Extra = dec.Extra
	h.MixDigest = dec.MixDigest
	h.Nonce = dec.Nonce
	h.BaseFee = (*big.Int)(dec.BaseFee)
	h.WithdrawalsHash = dec.WithdrawalsRoot
	h.BlobGasUsed = (*uint64)(dec.BlobGasUsed)
	h.ExcessBlobGas = (*uint64)(dec.ExcessBlobGas)
	h.ParentBeaconRoot = dec.ParentBeaconBlockRoot
	h.Header.RequestsHash = dec.RequestsRoot
	h.Author = dec.Author
	h.Size = dec.Size
	h.TotalDifficulty = dec.TotalDifficulty
	h.Hash = dec.Hash
	h.L1BlockNumber = dec.L1BlockNumber
	h.SendCount = dec.SendCount
	h.SendRoot = dec.SendRoot
	h.L1BatchNumber = dec.L1BatchNumber
	h.L1BatchTimestamp = dec.L1BatchTimestamp
	h.SealFields = dec.SealFields
	h.RequestsHash = dec.RequestsHash
	return nil
}

func (h ExtendedHeader) MarshalStructpb() *structpb.Value {
	fields := map[string]*structpb.Value{
		"parentHash":       structpb.NewStringValue(h.ParentHash.Hex()),
		"sha3Uncles":       structpb.NewStringValue(h.UncleHash.Hex()),
		"miner":            structpb.NewStringValue(h.Coinbase.Hex()),
		"stateRoot":        structpb.NewStringValue(h.Root.Hex()),
		"transactionsRoot": structpb.NewStringValue(h.TxHash.Hex()),
		"receiptsRoot":     structpb.NewStringValue(h.ReceiptHash.Hex()),
		"logsBloom":        structpb.NewStringValue(hexutil.Encode(h.Bloom[:])),
		"number":           structpb.NewStringValue((*hexutil.Big)(h.Number).String()),
		"gasLimit":         structpb.NewStringValue(hexutil.EncodeUint64(h.GasLimit)),
		"gasUsed":          structpb.NewStringValue(hexutil.EncodeUint64(h.GasUsed)),
		"timestamp":        structpb.NewStringValue(hexutil.EncodeUint64(h.Time)),
		"extraData":        structpb.NewStringValue(hexutil.Encode(h.Extra)),
		"mixHash":          structpb.NewStringValue(h.MixDigest.Hex()),
		"nonce":            structpb.NewStringValue(hexutil.Encode(h.Nonce[:])),
		"hash":             structpb.NewStringValue(h.Hash.Hex()),
	}
	if h.Difficulty != nil {
		fields["difficulty"] = structpb.NewStringValue((*hexutil.Big)(h.Difficulty).String())
	}
	if h.BaseFee != nil {
		fields["baseFeePerGas"] = structpb.NewStringValue((*hexutil.Big)(h.BaseFee).String())
	}
	if h.WithdrawalsHash != nil {
		fields["withdrawalsRoot"] = structpb.NewStringValue(h.WithdrawalsHash.Hex())
	}
	if h.BlobGasUsed != nil {
		fields["blobGasUsed"] = structpb.NewStringValue((*hexutil.Uint64)(h.BlobGasUsed).String())
	}
	if h.ExcessBlobGas != nil {
		fields["excessBlobGas"] = structpb.NewStringValue((*hexutil.Uint64)(h.ExcessBlobGas).String())
	}
	if h.ParentBeaconRoot != nil {
		fields["parentBeaconBlockRoot"] = structpb.NewStringValue(h.ParentBeaconRoot.String())
	}
	if h.Header.RequestsHash != nil {
		fields["requestsRoot"] = structpb.NewStringValue(h.Header.RequestsHash.String())
	}
	if h.Author != "" {
		fields["author"] = structpb.NewStringValue(h.Author)
	}
	if h.Size != nil {
		fields["size"] = structpb.NewStringValue(h.Size.String())
	}
	if h.TotalDifficulty != nil {
		fields["totalDifficulty"] = structpb.NewStringValue(h.TotalDifficulty.String())
	}
	if h.L1BlockNumber != nil {
		fields["l1BlockNumber"] = structpb.NewStringValue(h.L1BlockNumber.String())
	}
	if h.SendCount != nil {
		fields["sendCount"] = structpb.NewStringValue(h.SendCount.String())
	}
	if h.SendRoot != nil {
		fields["sendRoot"] = structpb.NewStringValue(h.SendRoot.Hex())
	}
	if h.L1BatchNumber != nil {
		fields["l1BatchNumber"] = structpb.NewStringValue(h.L1BatchNumber.String())
	}
	if h.L1BatchTimestamp != nil {
		fields["l1BatchTimestamp"] = structpb.NewStringValue(h.L1BatchTimestamp.String())
	}
	if h.SealFields != nil {
		sealFields, _ := structpb.NewList(utils.ToAnyArray(h.SealFields))
		fields["sealFields"] = structpb.NewListValue(sealFields)
	}
	if h.RequestsHash != nil {
		fields["requestsHash"] = structpb.NewStringValue(h.RequestsHash.String())
	}
	return structpb.NewStructValue(&structpb.Struct{Fields: fields})
}

func (h ExtendedHeader) GetBlockTime() time.Time {
	if ts := int64(h.Header.Time); ts < 10_000_000_000 {
		return time.Unix(ts, 0)
	} else if ts < 10_000_000_000_000 {
		return time.UnixMilli(ts)
	} else if ts < 10_000_000_000_000_000 {
		return time.UnixMicro(ts)
	} else {
		return time.Unix(0, ts)
	}
}
