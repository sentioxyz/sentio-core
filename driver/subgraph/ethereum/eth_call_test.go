package ethereum

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/abiutil"
	"sentioxyz/sentio-core/driver/subgraph/common"
)

//go:embed testdata/metacoin-abi.json
var metaCoinABI []byte

func Test_ethCall(t *testing.T) {
	t.Skip("will call alchemy online, will only be executed manually locally")

	const goerliEndpoint = "https://eth-goerli.g.alchemy.com/v2/8QJL478hs0cI9DUzmZm4rOrZzt-h4TFP"

	client, err := ethclient.Dial(goerliEndpoint)
	assert.NoError(t, err)

	ctx, logger := context.Background(), log.With()

	addr, err := wasm.BuildByteArrayFromHex("0xfdc006debd37838e9144f662968ee7b145e055b8")
	assert.NoError(t, err)

	contractABI, err := abi.JSON(bytes.NewReader(metaCoinABI))
	assert.NoError(t, err)
	methodABI := contractABI.Methods["getBalance"]

	params := wasm.ObjectArray[*Value]{
		Data: []*Value{{
			Kind:  ValueKindAddress,
			Value: common.MustBuildAddressFromString("0x31dc8e776275249e9a14a8e48c57ab2da1f688a4"),
		}},
	}

	ret, err := EthCall(ctx, logger, client, addr.Data, &methodABI, &params, uint64(9085388))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret.Data))
	assert.Equal(t, common.MustBuildBigInt(10000), ret.Data[0].Value)

	// In transaction 0x6bbc76aab4b8cb7e271ddaeb3fcb176879aeda2cc2d174ffc88e86163258e9e5 of block 9085389
	// 0x31dc8e776275249e9a14a8e48c57ab2da1f688a4 transferred 4433 to 0x8f828e1264504cd3b0b55ccdbd6df3de5d13a7f6

	ret, err = EthCall(ctx, logger, client, addr.Data, &methodABI, &params, uint64(9085389))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret.Data))
	assert.Equal(t, common.MustBuildBigInt(5567), ret.Data[0].Value)
}

//go:embed testdata/erc20-abi.json
var erc20ABI []byte

//go:embed testdata/erc721-abi.json
var erc721ABI []byte

//go:embed testdata/dstoken-abi.json
var dsTokenABI []byte

func Test_ethCallUnpack(t *testing.T) {
	t.Skip("will call alchemy online, will only be executed manually locally")

	ctx, logger := context.Background(), log.With()

	// EventLog:
	//   blockNumber: 14934684
	//   transactionHash: 0xff669b8fda4f794bdf4948ee8fb006c965ecb8af6ea452d7483300abfcaff7f0
	//   logIndex: 148
	//   topic0: 0xb734785e2fac0d09f3e4d4c0240bc9009e97a639cc8b5bbec49dfb40b18de384
	//   topic1: 0x40dFB80A253414C07e8189B863424Fb19521749b
	//   topic2: 0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2
	// eventHandler handlePoolTokenCreated in bancor-v3 use topic2 as ERC20 contract address,
	// try to call name function, caused unpack output error. In fact, the contract is DSToken, not ERC20.
	// https://etherscan.io/address/0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2#code

	const mainnetEndpoint = "https://eth-mainnet.alchemyapi.io/v2/BJQpOPPKYT7mr3X6iRL3IawOJPnyiG-P"

	client, err := ethclient.Dial(mainnetEndpoint)
	assert.NoError(t, err)

	addr, err := wasm.BuildByteArrayFromHex("0x9f8f72aa9304c8b593d555f12ef6589cc3a579a2")
	assert.NoError(t, err)

	contractABI, err := abi.JSON(bytes.NewReader(erc20ABI))
	//contractABI, err := abi.JSON(bytes.NewReader(dsTokenABI))
	assert.NoError(t, err)
	methodABI := contractABI.Methods["name"]

	for i, output := range methodABI.Outputs {
		fmt.Printf("output[%d][%s][%v]: %#v\n", i, output.Name, output.Indexed, output.Type)
	}

	params := wasm.ObjectArray[*Value]{}

	ret, err := EthCall(ctx, logger, client, addr.Data, &methodABI, &params, uint64(14934684))
	assert.NoError(t, err)
	assert.Nil(t, ret)
}

func Test_ethCallERC721(t *testing.T) {
	t.Skip("will call alchemy online, will only be executed manually locally")

	const mainnetEndpoint = "https://eth-mainnet.alchemyapi.io/v2/BJQpOPPKYT7mr3X6iRL3IawOJPnyiG-P"

	client, err := ethclient.Dial(mainnetEndpoint)
	assert.NoError(t, err)

	ctx, logger := context.Background(), log.With()

	addr, err := wasm.BuildByteArrayFromHex("0x97f29a145ffe5a51e27f74ea925c97954a39759f")
	assert.NoError(t, err)

	contractABI, err := abi.JSON(bytes.NewReader(erc721ABI))
	assert.NoError(t, err)

	callFunc := func(funcName string) {
		methodABI := contractABI.Methods[funcName]
		p := abiutil.NewStdOutPrinter()
		p.PrintMethod(&methodABI, funcName)
		params := wasm.ObjectArray[*Value]{}
		ret, err := EthCall(ctx, logger, client, addr.Data, &methodABI, &params, uint64(17930000))
		assert.NoError(t, err)
		for i, val := range ret.Data {
			fmt.Printf("ret[%d]: %s\n", i, val.String())
		}
	}
	callFunc("name")
	callFunc("symbol")
}

//go:embed testdata/seniorpool-abi.json
var seniorPoolABI []byte

func Test_ethCallPackUseBytes32(t *testing.T) {
	t.Skip("will call alchemy online, will only be executed manually locally")

	const mainnetEndpoint = "https://eth-mainnet.alchemyapi.io/v2/BJQpOPPKYT7mr3X6iRL3IawOJPnyiG-P"

	client, err := ethclient.Dial(mainnetEndpoint)
	assert.NoError(t, err)

	ctx, logger := context.Background(), log.With()

	contractABI, err := abi.JSON(bytes.NewReader(seniorPoolABI))
	assert.NoError(t, err)

	const methodSig = "hasRole(bytes32,address):(bool)"
	method := abiutil.FindMethodBySig(&contractABI, methodSig)

	base64Role := "1iSwS2qG3ohiXMB4Ala4UVfFphXbVtE1fgqXow/eJ2c="
	rawRole, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(base64Role))))
	assert.NoError(t, err)
	role := &Value{
		Kind:  ValueKindBytes,
		Value: &wasm.ByteArray{Data: rawRole},
	}
	address := &Value{
		Kind:  ValueKindAddress,
		Value: common.MustBuildAddressFromString("0xf04accd11e5bf12d1d48e8bc48713a600187b0cf"),
	}

	result, err := EthCall(
		ctx,
		logger,
		client,
		wasm.MustBuildByteArrayFromHex("0x8481a6ebaf5c7dabc3f7e09e44a89531fd31f822").Data,
		method,
		&wasm.ObjectArray[*Value]{Data: []*Value{role, address}},
		14636818,
	)
	assert.NoError(t, err)
	assert.Equal(t, &wasm.ObjectArray[*Value]{Data: []*Value{{
		Kind:  ValueKindBool,
		Value: wasm.Bool(false),
	}}}, result)
}

//go:embed testdata/comet-abi.json
var cometABI []byte

func Test_ethCallPackUseUint8(t *testing.T) {
	t.Skip("will call alchemy online, will only be executed manually locally")

	const mainnetEndpoint = "https://eth-mainnet.alchemyapi.io/v2/BJQpOPPKYT7mr3X6iRL3IawOJPnyiG-P"

	client, err := ethclient.Dial(mainnetEndpoint)
	assert.NoError(t, err)

	ctx, logger := context.Background(), log.With()

	contractABI, err := abi.JSON(bytes.NewReader(cometABI))
	assert.NoError(t, err)

	const methodSig = "getAssetInfo(uint8):((uint8,address,address,uint64,uint64,uint64,uint64,uint128))"
	method := abiutil.FindMethodBySig(&contractABI, methodSig)

	result, err := EthCall(
		ctx,
		logger,
		client,
		wasm.MustBuildByteArrayFromHex("0xc3d688b66703497daa19211eedff47f25384cdc3").Data,
		method,
		&wasm.ObjectArray[*Value]{Data: []*Value{{Kind: ValueKindInt, Value: common.MustBuildBigInt(0)}}},
		15331596,
	)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Data))
	assert.Equal(t, &Value{Kind: ValueKindTuple, Value: NewTuple(
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(0)},
		&Value{Kind: ValueKindAddress,
			Value: common.MustBuildAddressFromString("0xc00e94cb662c3520282e6f5717214004a7f26888")},
		&Value{Kind: ValueKindAddress,
			Value: common.MustBuildAddressFromString("0xdbd020caef83efd542f4de03e3cf0c28a4428bd5")},
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(1000000000000000000)},
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(650000000000000000)},
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(700000000000000000)},
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(930000000000000000)},
		&Value{Kind: ValueKindUint, Value: common.MustBuildBigInt(0)},
	)}, result.Data[0])
}
