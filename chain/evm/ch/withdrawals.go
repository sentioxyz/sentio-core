package ch

type Withdrawal struct {
	BlockIndex

	Index          uint64 // `clickhouse:"withdrawal_index"`
	ValidatorIndex uint64 // `clickhouse:"validator_index"`
	Address        string // `clickhouse:"address" type:"FixedString(42)"`
	Amount         uint64 // `clickhouse:"amount"`
}
