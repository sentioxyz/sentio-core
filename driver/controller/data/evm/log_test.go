package evm

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/controller"
)

func Test_logFilter(t *testing.T) {
	var ev types.Log
	evRaw := `{
    "address": "0xbe9895146f7af43049ca1c1ae358b0541ea49704",
    "topics": [
      "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "0x000000000000000000000000fae23c30d383df59d3e031c325a73d454e8721a6"
    ],
    "data": "0x000000000000000000000000000000000000000000000000000000003b9aca00",
    "blockNumber": "0xd8110d",
    "transactionHash": "0xa4cc1d25099cc2f8fc18ae8a54e079076a89db38dad54696f7453afa3adcfbe8",
    "transactionIndex": "0x8c",
    "blockHash": "0x5b22e991900f1e961ff296dd37973279a91dc97ecdb2ae290680e512b3dec7fe",
    "logIndex": "0xaf",
    "removed": false
  }`
	assert.NoError(t, json.Unmarshal([]byte(evRaw), &ev))
	f := LogFilter{
		Topics: [][]string{
			{
				"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
			},
		},
		Address: []string{
			"0xbe9895146f7af43049ca1c1ae358b0541ea49704",
		},
	}
	ok, err := f.BuildChecker(nil, nil)(ev)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func Test_MergeLogRequirements(t *testing.T) {
	var reqs []LogRequirement
	var addrs []string
	for i := 0; i < 20000; i++ {
		addr := fmt.Sprintf("0x%040x", i)
		addrs = append(addrs, addr)
		reqs = append(reqs, LogRequirement{
			BlockRange: controller.BlockRange{StartBlock: uint64(i * 1000)},
			LogFilter: LogFilter{
				Topics: [][]string{{
					"0x503ec09c1b4597d114eca849f7013c8c457988802d1dc5da49a0c461b5f88658",
					"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
					"0x53a0f15ecd2c4989821244d3c7363d7de154e125eac5b2a7c9bb008152778de1",
					"0xd51a9c61267aa6196961883ecf5ff2da6619c37dac0fa92122513fb32c032d2d",
					"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0",
					"0x043a568d47b1a65cdc989ff14c921411b62abf40132dc4b5e78675ba8d0bc9df",
				}},
				Address: []string{addr},
			},
		})
	}
	r := MergeLogRequirements(199991000, reqs)
	assert.Equal(t, 1, len(r))
	assert.Equal(t, set.New(addrs...), set.New(r[0].Address...))
	assert.False(t, r[0].AddressShouldBeERC20)
	assert.Equal(t, 1, len(r[0].Topics))
	assert.Equal(t,
		set.New(
			"0x503ec09c1b4597d114eca849f7013c8c457988802d1dc5da49a0c461b5f88658",
			"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
			"0x53a0f15ecd2c4989821244d3c7363d7de154e125eac5b2a7c9bb008152778de1",
			"0xd51a9c61267aa6196961883ecf5ff2da6619c37dac0fa92122513fb32c032d2d",
			"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0",
			"0x043a568d47b1a65cdc989ff14c921411b62abf40132dc4b5e78675ba8d0bc9df"),
		set.New(r[0].Topics[0]...),
	)
	assert.Equal(t, controller.BlockRange{StartBlock: 199991000}, r[0].BlockRange)
	//for i, x := range r {
	//	fmt.Printf("[%d]: %s\n", i, x.String())
	//}
	//fmt.Printf("used1: %s\n", used1.String())
	//fmt.Printf("used2: %s\n", used2.String())
}
