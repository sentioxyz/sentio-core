package types

import "encoding/json"

type EventID struct {
	TxDigest Digest `json:"txDigest"`
	EventSeq Number `json:"eventSeq"`
}

type Event struct {
	ID                EventID         `json:"id"`
	PackageID         ObjectID        `json:"packageId"`
	TransactionModule string          `json:"transactionModule"`
	Sender            string          `json:"sender"`
	Type              TypeTag         `json:"type"`
	Fields            json.RawMessage `json:"parsedJson"`

	// TODO
	// testnet is using Base64Data, but mainnet is using Base58Data.
	// Considering that this field is not actually used at present, do not decode this field for now.
	BCS string `json:"bcs"`
	//BCS Base58Data `json:"bcs"`
}
