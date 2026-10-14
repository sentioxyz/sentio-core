package ch

type Withdrawal struct {
	BlockIndex

	// Index is the EIP-4895 withdrawal index: a global counter over the validator withdrawals of the chain,
	// incremented by one per withdrawal. Berachain additionally opens every block with one withdrawal whose
	// index and validator index are both 0xffffffffffffffff (seen at block 11459328), at most one per block,
	// so (block_number, withdrawal_index) still identifies a row.
	Index          uint64 `clickhouse:"withdrawal_index"`
	ValidatorIndex uint64 `clickhouse:"validator_index"`
	Address        string `clickhouse:"address" type:"FixedString(42)" index:"bloom_filter"`
	Amount         uint64 `clickhouse:"amount"`
}
