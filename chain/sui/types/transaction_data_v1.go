package types

import (
	"bytes"
	"errors"
	"io"

	"github.com/fardream/go-bcs/bcs"
	"github.com/goccy/go-json"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

type TransactionDataV1 struct {
	Kind       *TransactionKind       `json:"transaction"`
	Sender     Address                `json:"sender"`
	GasData    *GasData               `json:"gasData"`
	Expiration *TransactionExpiration `json:"-"`
}

type TransactionData struct {
	V1 *TransactionDataV1 `json:"V1,omitempty"`
}

type txDataJSON struct {
	MessageVersion string `json:"messageVersion"`
	*TransactionDataV1
}

func (s *TransactionData) UnmarshalJSON(data []byte) error {
	var txData txDataJSON
	err := json.Unmarshal(data, &txData)
	if err != nil {
		return err
	}
	if txData.MessageVersion != "v1" || txData.TransactionDataV1 == nil {
		return errors.New("invalid message version")
	}
	s.V1 = txData.TransactionDataV1
	return nil
}

func (s TransactionData) MarshalJSON() ([]byte, error) {
	return json.Marshal(txDataJSON{
		MessageVersion:    "v1",
		TransactionDataV1: s.V1,
	})
}

func (s *TransactionData) IsBcsEnum() {}

var EmptyIntentMessage = IntentMessage{
	Scope:   0,
	Version: 0,
	AppID:   0,
}

type IntentMessage struct {
	Scope   int `json:"scope"`
	Version int `json:"version"`
	AppID   int `json:"appId"`
}

func (s IntentMessage) MarshalBCS() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.Write(bcs.ULEB128Encode(s.Scope))
	buf.Write(bcs.ULEB128Encode(s.Version))
	buf.Write(bcs.ULEB128Encode(s.AppID))
	return buf.Bytes(), nil
}

func (s *IntentMessage) UnmarshalBCS(r io.Reader) (int, error) {
	var err error
	s.Scope, _, err = bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	s.Version, _, err = bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	s.AppID, _, err = bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

type Signature []byte

type SenderSignedTransaction struct {
	Intent       *IntentMessage   `json:"-"`
	Data         *TransactionData `json:"data"`
	TxSignatures []Signature      `json:"txSignatures"`
}

type SenderSignedData struct {
	Transactions []SenderSignedTransaction
}

func DecodeSenderSignedData(b []byte) (*SenderSignedData, error) {
	data := &SenderSignedData{}
	return data, serde.NewDecoder(bytes.NewReader(b)).Decode(data)
}

func EncodeSenderSignedData(data *SenderSignedData) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := serde.NewEncoder(buf).Encode(data)
	return buf.Bytes(), err
}

func DeriveAuxInformationFromBCSV1(data *TransactionDataV1, rawTransaction []byte) error {
	if data == nil || data.Kind == nil {
		return errors.New("invalid transaction, no data populated")
	}
	decoded, err := DecodeSenderSignedData(rawTransaction)
	if err != nil {
		return err
	}
	if len(decoded.Transactions) != 1 {
		return errors.New("invalid transactions")
	}
	if decoded.Transactions[0].Data == nil || decoded.Transactions[0].Data.V1 == nil {
		return errors.New("invalid transaction data")
	}
	decodedV1 := decoded.Transactions[0].Data.V1
	data.Expiration = decodedV1.Expiration

	populateChangeEpoch := func(x *ChangeEpoch, y *ChangeEpoch) {
		// These fields are not included in JSON serialization, but are needed.
		x.ProtocolVersion = y.ProtocolVersion
		x.NonRefundableStorageFee = y.NonRefundableStorageFee
		x.SystemPackages = y.SystemPackages
	}
	switch {
	case data.Kind.ChangeEpoch != nil:
		if decodedV1.Kind.ChangeEpoch == nil {
			// This indicates a mismatch between the given transaction data and the decoded transaction.
			return errors.New("decodedV1.Kind.ChangeEpoch is nil")
		}
		decodedTx := decodedV1.Kind.ChangeEpoch
		targetTx := data.Kind.ChangeEpoch
		populateChangeEpoch(targetTx, decodedTx)
	case data.Kind.ProgrammableTransaction != nil:
		if decodedV1.Kind.ProgrammableTransaction == nil {
			// This indicates a mismatch between the given transaction data and the decoded transaction.
			return errors.New("decodedV1.Kind.ProgrammableTransaction is nil")
		}
		decodedTx := decodedV1.Kind.ProgrammableTransaction
		targetTx := data.Kind.ProgrammableTransaction
		if len(decodedTx.Inputs) != len(targetTx.Inputs) {
			return errors.New("invalid number of inputs")
		}
		if len(decodedTx.Commands) != len(targetTx.Commands) {
			return errors.New("invalid number of outputs")
		}
		for i := range targetTx.Inputs {
			// Should have the same kind.
			decodedPure := decodedTx.Inputs[i].Pure
			targetPure := targetTx.Inputs[i].Pure
			targetIsPure := targetPure != nil
			decodedIsPure := decodedPure != nil
			if targetIsPure != decodedIsPure {
				return errors.New("targetIsPure != decodedIsPure")
			}
			if !targetIsPure {
				continue
			}
			// Derive raw bytes from BCS.
			targetPure.Value = decodedPure.Value
		}

		for i := range targetTx.Commands {
			switch {
			case targetTx.Commands[i].Publish != nil:
				targetPublish := targetTx.Commands[i].Publish
				decodedPublish := decodedTx.Commands[i].Publish
				if decodedPublish == nil {
					return errors.New("decodedPublish is nil")
				}
				// Derive move bytecodes from BCS.
				targetPublish.Package.ByteCodes = decodedPublish.Package.ByteCodes
			case targetTx.Commands[i].Upgrade != nil:
				targetUpgrade := targetTx.Commands[i].Upgrade
				decodedUpgrade := decodedTx.Commands[i].Upgrade
				if decodedUpgrade == nil {
					return errors.New("decodedUpgrade is nil")
				}
				// Derive move bytecodes from BCS.
				targetUpgrade.Package.ByteCodes = decodedUpgrade.Package.ByteCodes
			case targetTx.Commands[i].MoveCall != nil:
				targetMoveCall := targetTx.Commands[i].MoveCall
				decodedMoveCall := decodedTx.Commands[i].MoveCall
				if decodedMoveCall == nil {
					return errors.New("decodedMoveCall is nil")
				}
				targetMoveCall.TypeArgs = decodedMoveCall.TypeArgs
			}
		}
	case data.Kind.AuthenticatorStateUpdate != nil:
		if decodedV1.Kind.AuthenticatorStateUpdate == nil {
			// This indicates a mismatch between the given transaction data and the decoded transaction.
			return errors.New("decodedV1.Kind.AuthenticatorStateUpdate is nil")
		}
		data.Kind.AuthenticatorStateUpdate.AuthenticatorObjInitialSharedVersion = decodedV1.Kind.AuthenticatorStateUpdate.AuthenticatorObjInitialSharedVersion
	case data.Kind.EndOfEpochTransaction != nil:
		if decodedV1.Kind.EndOfEpochTransaction == nil {
			// This indicates a mismatch between the given transaction data and the decoded transaction.
			return errors.New("decodedV1.Kind.EndOfEpochTransaction is nil")
		}
		if len(decodedV1.Kind.EndOfEpochTransaction.Transactions) != len(data.Kind.EndOfEpochTransaction.Transactions) {
			return errors.New("number of transactions mismatch")
		}
		for i := range data.Kind.EndOfEpochTransaction.Transactions {
			x := &data.Kind.EndOfEpochTransaction.Transactions[i]
			y := &decodedV1.Kind.EndOfEpochTransaction.Transactions[i]
			if x.AuthenticatorStateExpire != nil {
				if y.AuthenticatorStateExpire == nil {
					return errors.New("y.AuthenticatorStateExpire == nil")
				}
				x.AuthenticatorStateExpire.AuthenticatorObjInitialSharedVersion = y.AuthenticatorStateExpire.AuthenticatorObjInitialSharedVersion
			} else if x.ChangeEpoch != nil {
				if y.ChangeEpoch == nil {
					return errors.New("y.ChangeEpoch == nil")
				}
				populateChangeEpoch(x.ChangeEpoch, y.ChangeEpoch)
			}
		}
	}
	return nil
}
