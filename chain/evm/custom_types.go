package evm

import "github.com/ethereum/go-ethereum/common"

type StorageAtArgs struct {
	Address common.Address `json:"address"`
	Key     common.Hash    `json:"key"`
}

type MultipleStorageAtArgs []*StorageAtArgs

type StorageAtResult struct {
	Address common.Address `json:"address"`
	Key     common.Hash    `json:"key"`
	Data    common.Hash    `json:"data"`
}

type MultipleStorageAtResult []*StorageAtResult
