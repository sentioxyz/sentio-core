package types

import "encoding/json"

type CheckpointStub struct {
	// Checkpoint fields are custom field added by us.
	// We have to return it to the client so that it can be used to associate the transaction with a timestamp.
	Checkpoint            Number  `json:"checkpoint"`
	CheckpointTimestampMs *Number `json:"checkpoint_timestamp_ms,omitempty"`
	TransactionPosition   int     `json:"transaction_position,omitempty"`
}

type ObjectChangeType string

const (
	ObjectChangeTypeUnknown              = "unknown"
	ObjectChangeTypeAccumulatorWrite     = "accumulatorWrite"
	ObjectChangeTypePublished            = "published"
	ObjectChangeTypeCreated              = "created"
	ObjectChangeTypeUnwrapped            = "unwrapped"
	ObjectChangeTypeTransferred          = "transferred" // deprecated
	ObjectChangeTypeMutated              = "mutated"
	ObjectChangeTypeDeleted              = "deleted"
	ObjectChangeTypeWrapped              = "wrapped"
	ObjectChangeTypeUnwrappedThenDeleted = "unwrappedThenDeleted"
)

func (t ObjectChangeType) IsDeleted() bool {
	switch t {
	case ObjectChangeTypeDeleted, ObjectChangeTypeWrapped, ObjectChangeTypeUnwrappedThenDeleted:
		return true
	}
	return false
}

func (t ObjectChangeType) IsCreated() bool {
	switch t {
	case ObjectChangeTypeCreated, ObjectChangeTypePublished, ObjectChangeTypeUnwrapped:
		return true
	default:
		return false
	}
}

type ObjectChange struct {
	Type            ObjectChangeType `json:"type"`
	Digest          Digest           `json:"digest"`
	Version         Number           `json:"version"`
	PreviousVersion *Number          `json:"previousVersion,omitempty"`
	Sender          *Address         `json:"sender,omitempty"`
	ObjectID        *ObjectID        `json:"objectId,omitempty"`
	ObjectType      *TypeTag         `json:"objectType,omitempty"`
	Recipient       *ObjectOwner     `json:"recipient,omitempty"`
	Owner           *ObjectOwner     `json:"owner,omitempty"`

	Modules   []string  `json:"modules,omitempty"`
	PackageID *ObjectID `json:"packageId,omitempty"`
}

type ObjectChangeExtend struct {
	Checkpoint       Number `json:"checkpoint"`
	CheckpointDigest Digest `json:"checkpoint_digest"`
	TxIndex          int    `json:"transaction_index"`
	TxDigest         Digest `json:"transaction_digest"`

	ObjectChange
}

func (o ObjectChange) GetObjectID() string {
	if o.PackageID != nil {
		return o.PackageID.String()
	}
	if o.ObjectID != nil {
		return o.ObjectID.String()
	}
	return ""
}

type BalanceChange struct {
	Owner    *ObjectOwner `json:"owner"`
	CoinType *TypeTag     `json:"coinType"`
	Amount   Number       `json:"amount"`
}

type TransactionResponseV1 struct {
	CheckpointStub `json:",inline"`
	Digest         Digest                   `json:"digest"`
	Transaction    *SenderSignedTransaction `json:"transaction,omitempty"`
	RawTransaction Base64Data               `json:"rawTransaction"`
	ObjectChanges  []ObjectChange           `json:"objectChanges,omitempty"`
	Effects        *TransactionEffectsV1    `json:"effects,omitempty"`
	Events         []Event                  `json:"events"`
	Errors         []string                 `json:"errors,omitempty"` // always empty
	BalanceChanges []BalanceChange          `json:"balanceChanges,omitempty"`
	TimestampMs    Number                   `json:"timestampMs"`
}

type GasCostSummary struct {
	ComputationCost         Number `json:"computationCost"`
	StorageCost             Number `json:"storageCost"`
	StorageRebate           Number `json:"storageRebate"`
	NonRefundableStorageFee Number `json:"nonRefundableStorageFee"`
}

const (
	TransactionStatusSuccess = "success"
	TransactionStatusFailure = "failure"
)

type TransactionStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ObjectIDAndSeq struct {
	ObjectID       ObjectID `json:"objectId"`
	SequenceNumber Number   `json:"sequenceNumber"`
}

type TransactionEffectsV1 struct {
	MessageVersion     string            `json:"messageVersion"`
	Status             TransactionStatus `json:"status"`
	ExecutedEpoch      Number            `json:"executedEpoch"`
	GasUsed            *GasCostSummary   `json:"gasUsed"`
	ModifiedAtVersions []ObjectIDAndSeq  `json:"modifiedAtVersions"`
	SharedObjects      []ObjectRefLegacy `json:"sharedObjects,omitempty"`
	TransactionDigest  Digest            `json:"transactionDigest"`

	Created              []OwnedObjectRef  `json:"created,omitempty"`
	Mutated              []OwnedObjectRef  `json:"mutated,omitempty"`
	Unwrapped            []OwnedObjectRef  `json:"unwrapped,omitempty"`
	Deleted              []ObjectRefLegacy `json:"deleted,omitempty"`
	UnwrappedThenDeleted []ObjectRefLegacy `json:"unwrappedThenDeleted,omitempty"`
	Wrapped              []ObjectRefLegacy `json:"wrapped,omitempty"`

	GasObject    *OwnedObjectRef `json:"gasObject"`
	EventsDigest *Digest         `json:"eventsDigest,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
}

type CheckpointResponse struct {
	Epoch                      Number `json:"epoch"`
	SequenceNumber             Number `json:"sequenceNumber"`
	Digest                     Digest `json:"digest"`
	NetworkTotalTransactions   Number `json:"networkTotalTransactions"`
	PreviousDigest             string `json:"previousDigest"`
	EpochRollingGasCostSummary struct {
		ComputationCost Number `json:"computation_cost"`
		StorageCost     Number `json:"storage_cost"`
		StorageRebate   Number `json:"storage_rebate"`
	} `json:"epochRollingGasCostSummary"`
	TimestampMs  Number   `json:"timestampMs"`
	Transactions []string `json:"transactions"`
}

type DynamicFieldName struct {
	Type  TypeTag         `json:"type"`
	Value json.RawMessage `json:"value"`
}

type DynamicFieldInfo struct {
	Name       DynamicFieldName `json:"name"`
	BCSName    Base58Data       `json:"bcsName"`
	Type       string           `json:"type"`
	ObjectType string           `json:"objectType"`
	ObjectID   ObjectID         `json:"objectId"`
	Version    Number           `json:"version"`
	Digest     Digest           `json:"digest"`
}

type DynamicFieldPage struct {
	Data        []DynamicFieldInfo `json:"data"`
	NextCursor  *ObjectID          `json:"nextCursor"`
	HasNextPage bool               `json:"hasNextPage"`
}

type SuiGetPastObjectRequest struct {
	ObjectID ObjectID `json:"objectId"`
	Version  Number   `json:"version"`
}

const (
	SuiPastObjectStatusVersionFound    = "VersionFound"
	SuiPastObjectStatusObjectNotExists = "ObjectNotExists"
	SuiPastObjectStatusObjectDeleted   = "ObjectDeleted"
	SuiPastObjectStatusVersionNotFound = "VersionNotFound"
	SuiPastObjectStatusVersionTooHigh  = "VersionTooHigh"
)

type SuiPastObjectResponse struct {
	Status  string
	Details json.RawMessage
}

type SuiObjectDataOptions struct {
	ShowType                bool `json:"showType"`
	ShowOwner               bool `json:"showOwner"`
	ShowPreviousTransaction bool `json:"showPreviousTransaction"`
	ShowDisplay             bool `json:"showDisplay"`
	ShowContent             bool `json:"showContent"`
	ShowBCS                 bool `json:"showBcs"`
	ShowStorageRebate       bool `json:"showStorageRebate"`
}

type SuiObjectResponse struct {
	Data struct {
		ObjectID            ObjectID     `json:"objectId"`
		Version             Number       `json:"version"`
		Digest              Digest       `json:"digest"`
		Type                TypeTag      `json:"type"`
		Owner               *ObjectOwner `json:"owner"`
		PreviousTransaction *Digest      `json:"previousTransaction"`
		StorageRebate       Number       `json:"storageRebate"`
		Content             struct {
			DataType          string          `json:"dataType"`
			Type              TypeTag         `json:"type"`
			HasPublicTransfer bool            `json:"hasPublicTransfer"`
			Fields            json.RawMessage `json:"fields"`
		} `json:"content"`
	} `json:"data"`
}
