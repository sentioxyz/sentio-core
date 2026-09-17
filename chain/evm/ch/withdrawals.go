package ch

type Withdrawal struct {
	BlockIndex

	// Index is the EIP-4895 withdrawal index: a global counter over every withdrawal of the chain,
	// incremented by one per withdrawal, so (block_number, withdrawal_index) identifies a row.
	Index          uint64 `clickhouse:"withdrawal_index"`
	ValidatorIndex uint64 `clickhouse:"validator_index"`
	Address        string `clickhouse:"address" type:"FixedString(42)" index:"bloom_filter"`
	Amount         uint64 `clickhouse:"amount"`
}
