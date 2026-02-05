package manifest

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ipfs/go-ipfs-api"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"sentioxyz/sentio-core/driver/subgraph/abiutil"
)

func Test_load(t *testing.T) {
	orig := `
dataSources:
  - kind: ethereum
    context:
      foo:
        type: Bool
        data: true
      bar:
        type: String
        data: 'bar'
      int:
        type: Int
        data: 123
      int8:
        type: Int8
        data: 1234
      float:
        type: BigDecimal
        data: "12345678901234567890.1234567890"
      bytes:
        type: Bytes
        data: 0x1234
      big:
        type: BigInt
        data: 123456789012345678901234567890
    mapping:
      abis:
        - file:
            /: /ipfs/Qmf9ihFQ8NAtAwuCr2JWN7gfwdJqncNpkLUNxs4HRQCWEV
          name: MetaCoin
      apiVersion: 0.0.7
      entities:
        - Transfer
      eventHandlers:
        - event: 'Transfer(indexed address,indexed address,uint256)'
          handler: handleTransfer
      callHandlers:
        - function: 'sendCoin(address,uint256)'
          handler: handleSendCoin
      file:
        /: /ipfs/QmSYiKn5eqgvQ6p87YS8YvN28VuPjinNdtpMymt1H3qHC1
      kind: ethereum/events
      language: wasm/assemblyscript
    name: MetaCoin
    network: goerli
    source:
      abi: MetaCoin
      address: '0xa4b7e47d514013129925fe689dc850c16729aa51'
      startBlock: 8772144
templates:
  - kind: ethereum
    context:
      foo:
        type: Bool
        data: true
      bar:
        type: String
        data: 'bar'
      int:
        type: Int
        data: 123
      int8:
        type: Int8
        data: 1234
      float:
        type: BigDecimal
        data: "12345678901234567890.1234567890"
      bytes:
        type: Bytes
        data: 0x1234
      big:
        type: BigInt
        data: 123456789012345678901234567890
    mapping:
      abis:
        - file:
            /: /ipfs/Qmf9ihFQ8NAtAwuCr2JWN7gfwdJqncNpkLUNxs4HRQCWEV
          name: MetaCoin
      apiVersion: 0.0.7
      entities:
        - Transfer
      eventHandlers:
        - event: 'Transfer(indexed address,indexed address,uint256)'
          handler: handleTransfer
      callHandlers:
        - function: 'sendCoin(address,uint256)'
          handler: handleSendCoin
      file:
        /: /ipfs/QmSYiKn5eqgvQ6p87YS8YvN28VuPjinNdtpMymt1H3qHC1
      kind: ethereum/events
      language: wasm/assemblyscript
    name: MetaCoin
    network: goerli
    source:
      abi: MetaCoin
      address: '0xa4b7e47d514013129925fe689dc850c16729aa51'
schema:
  file:
    /: /ipfs/QmctuJqXCKeHYCc3BUTeopxRe6DgHFRuSeCjKU4vCQbzen
specVersion: 0.0.5
description: good
repository: https://github.com/graphprotocol/graph-tooling/
`
	mf, err := load(bytes.NewReader([]byte(orig)))
	assert.NoError(t, err)
	assert.Equal(t, &Manifest{
		SpecVersion: "0.0.5",
		Schema: Schema{File: map[string]string{
			"/": "/ipfs/QmctuJqXCKeHYCc3BUTeopxRe6DgHFRuSeCjKU4vCQbzen",
		}},
		Description: "good",
		Repository:  "https://github.com/graphprotocol/graph-tooling/",
		DataSources: []*DataSource{{
			Kind:    "ethereum",
			Context: `{"bar":{"Kind":0,"Value":"bar","Array":null},"big":{"Kind":7,"Value":"123456789012345678901234567890","Array":null},"bytes":{"Kind":6,"Value":"0x1234","Array":null},"float":{"Kind":2,"Value":"12345678901234567890.1234567890","Array":null},"foo":{"Kind":3,"Value":"true","Array":null},"int":{"Kind":1,"Value":"123","Array":null},"int8":{"Kind":8,"Value":"1234","Array":null}}`,
			Name:    "MetaCoin",
			Network: "goerli",
			Source: EthereumContractSource{
				Abi:        "MetaCoin",
				Address:    "0xa4b7e47d514013129925fe689dc850c16729aa51",
				StartBlock: BuildBigIntFromUint(8772144),
			},
			Mapping: EthereumMapping{
				Kind:       "ethereum/events",
				APIVersion: "0.0.7",
				Language:   "wasm/assemblyscript",
				Entities:   []string{"Transfer"},
				Abis: []*Abi{{
					Name: "MetaCoin",
					File: map[string]string{"/": "/ipfs/Qmf9ihFQ8NAtAwuCr2JWN7gfwdJqncNpkLUNxs4HRQCWEV"},
				}},
				EventHandlers: []*EventHandler{{
					Event:   "Transfer(indexed address,indexed address,uint256)",
					Handler: "handleTransfer",
				}},
				CallHandlers: []*CallHandler{{
					Function: "sendCoin(address,uint256)",
					Handler:  "handleSendCoin",
				}},
				// BlockHandlers: []BlockHandler{},
				File: map[string]string{"/": "/ipfs/QmSYiKn5eqgvQ6p87YS8YvN28VuPjinNdtpMymt1H3qHC1"},
			},
		}},
		Templates: []*DataSourceTemplate{{
			Kind:    "ethereum",
			Name:    "MetaCoin",
			Network: "goerli",
			Context: `{"bar":{"Kind":0,"Value":"bar","Array":null},"big":{"Kind":7,"Value":"123456789012345678901234567890","Array":null},"bytes":{"Kind":6,"Value":"0x1234","Array":null},"float":{"Kind":2,"Value":"12345678901234567890.1234567890","Array":null},"foo":{"Kind":3,"Value":"true","Array":null},"int":{"Kind":1,"Value":"123","Array":null},"int8":{"Kind":8,"Value":"1234","Array":null}}`,
			Source: EthereumContractSource{
				Abi:        "MetaCoin",
				Address:    "0xa4b7e47d514013129925fe689dc850c16729aa51",
				StartBlock: BuildBigIntFromUint(0),
			},
			Mapping: EthereumMapping{
				Kind:       "ethereum/events",
				APIVersion: "0.0.7",
				Language:   "wasm/assemblyscript",
				Entities:   []string{"Transfer"},
				Abis: []*Abi{{
					Name: "MetaCoin",
					File: map[string]string{"/": "/ipfs/Qmf9ihFQ8NAtAwuCr2JWN7gfwdJqncNpkLUNxs4HRQCWEV"},
				}},
				EventHandlers: []*EventHandler{{
					Event:   "Transfer(indexed address,indexed address,uint256)",
					Handler: "handleTransfer",
				}},
				CallHandlers: []*CallHandler{{
					Function: "sendCoin(address,uint256)",
					Handler:  "handleSendCoin",
				}},
				// BlockHandlers: []BlockHandler{},
				File: map[string]string{"/": "/ipfs/QmSYiKn5eqgvQ6p87YS8YvN28VuPjinNdtpMymt1H3qHC1"},
			},
		}},
		Features: nil,
	}, mf)
}

func Test_fullLoad(t *testing.T) {
	t.Skip("use local ipfs node, will only be executed manually locally")

	const localIpfsNodeURL = "localhost:5001"

	hash := "QmW11fPcUfuBKXjB6cBSnP1hZsbV9ppqzQfpXzWCmGLBnz"
	ipfsShell := shell.NewShell(localIpfsNodeURL)
	ipfsShell.SetTimeout(5 * time.Second)

	mf, err := LoadFromIpfs(ipfsShell, hash, true)
	assert.NoError(t, err)

	contractABI := mf.DataSources[0].GetABIByName("MetaCoin")

	p := abiutil.NewStdOutPrinter()

	for key, md := range contractABI.contractABI.Methods {
		p.PrintMethod(&md, key)
	}
	for key, ev := range contractABI.contractABI.Events {
		p.PrintEvent(&ev, key)
	}

	ev := contractABI.FindEventBySig("Transfer(indexed address,indexed address,uint256)")
	p.PrintEvent(ev, "TargetEvent")

	md := contractABI.FindMethodBySig("getBalance(address):(uint256)")
	addr := common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314")
	data, err := md.Inputs.Pack(addr)
	assert.NoError(t, err)
	fmt.Printf("pack data: %d %v\n", len(data), data)

	md = contractABI.FindMethodBySig("sendCoin(address,uint256)")
	assert.Equal(t, "sendCoin", md.Name)
}

func Test_loadDataSourceContext(t *testing.T) {
	raw := `
foo:
  type: Bool
  data: true
bar:
  type: String
  data: 'bar'
int:
  type: Int
  data: 123
int8:
  type: Int8
  data: 1234
float:
  type: BigDecimal
  data: "12345678901234567890.1234567890"
bytes:
  type: Bytes
  data: 0x1234
big:
  type: BigInt
  data: 123456789012345678901234567890
`
	var ctx DataSourceContext
	assert.NoError(t, yaml.NewDecoder(bytes.NewReader([]byte(raw))).Decode(&ctx))
	d, err := ctx.ToString()
	assert.NoError(t, err)
	assert.Equal(t, `{"bar":{"Kind":0,"Value":"bar","Array":null},"big":{"Kind":7,"Value":"123456789012345678901234567890","Array":null},"bytes":{"Kind":6,"Value":"0x1234","Array":null},"float":{"Kind":2,"Value":"12345678901234567890.1234567890","Array":null},"foo":{"Kind":3,"Value":"true","Array":null},"int":{"Kind":1,"Value":"123","Array":null},"int8":{"Kind":8,"Value":"1234","Array":null}}`, d)
}

func Test_loadDataSourceContext2(t *testing.T) {
	raw := `
foo:
  type: Bool
  data: true
bar:
  type: String
  data: 'bar'
int:
  type: Int
  data: 123
int8:
  type: Int8
  data: 1234
float:
  type: BigDecimal
  data: "12345678901234567890.1234567890"
bytes:
  type: Bytes
  data: 0xgh
big:
  type: BigInt
  data: 123456789012345678901234567890
`
	var ctx DataSourceContext
	assert.NoError(t, yaml.NewDecoder(bytes.NewReader([]byte(raw))).Decode(&ctx))
	_, err := ctx.ToString()
	assert.Equal(t, `build json payload for property "bytes" failed: invalid data "0xgh" with type "Bytes": invalid word "gh"`, err.Error())
}
