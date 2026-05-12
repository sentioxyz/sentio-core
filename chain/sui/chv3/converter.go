package chv3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"

	"github.com/pkg/errors"
)

type SlotConverter struct {
	cli                *sui.ClientPool
	fetchConcurrency   int
	fetchPageSize      int
	convertConcurrency uint
}

func NewSlotConverter(
	cli *sui.ClientPool,
	fetchConcurrency int,
	fetchPageSize int,
	convertConcurrency uint,
) SlotConverter {
	return SlotConverter{
		cli:                cli,
		fetchConcurrency:   fetchConcurrency,
		fetchPageSize:      fetchPageSize,
		convertConcurrency: convertConcurrency,
	}
}

func encodeModifiedAtVersions(m []types.ObjectIDAndSeq) []string {
	r := make([]string, len(m))
	for i, v := range m {
		r[i] = fmt.Sprintf("%s,%d", v.ObjectID.String(), v.SequenceNumber.Uint64())
	}
	return r
}

func digestToStringNilsafe(d *types.Digest) string {
	if d == nil {
		return ""
	}
	return d.String()
}

func (m *SlotConverter) ConvertTxn(
	cp *sui.SlotCheckpointInfo,
	t *types.TransactionResponseV1,
	tp int32,
) (
	txn CHUTransaction,
	events []CHUEvent,
	moveCalls []CHUMoveCall,
	balanceChanges []CHUBalanceChange,
	objectChanges []CHUObjectChange,
	err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = errors.Errorf("%v", panicErr)
		}
	}()

	if t.Checkpoint.Uint64() != cp.SequenceNumber {
		err = errors.Errorf("checkpoint mismatch: %v != %v", t.Checkpoint.Uint64(), cp.SequenceNumber)
		return
	}
	if t.Effects.MessageVersion != "v1" {
		err = errors.Errorf("unsupported effects message version: %v", t.Effects.MessageVersion)
		return
	}
	if t.Transaction == nil && len(t.Errors) == 0 {
		err = errors.Errorf("transaction has no errors nor transaction data")
		return
	}

	// ========================================
	// txn.CHUTransactionBasePart
	txn.CHUTransactionBasePart = CHUTransactionBasePart{
		// SlotCheckpointInfo
		Checkpoint:            cp.SequenceNumber,
		CheckpointDigest:      cp.Digest,
		CheckpointTimestampMs: cp.TimestampMs.Uint64(),
		CheckpointTimestamp:   time.UnixMilli(int64(cp.TimestampMs.Uint64())),
		TransactionPosition:   tp,
		// types.TransactionResponseV1
		TimestampMs:    t.TimestampMs.Uint64(),
		Timestamp:      time.UnixMilli(int64(t.TimestampMs.Uint64())),
		Digest:         t.Digest.String(),
		RawTransaction: string(t.RawTransaction.Data()),
		Errors:         t.Errors,
	}

	// ========================================
	// txn.CHUTransactionEffectPart
	var effectsJSON []byte
	effectsJSON, err = json.Marshal(t.Effects)
	if err != nil {
		err = errors.Wrapf(err, "marshal TransactionResponseV1.effects failed")
		return
	}
	txn.CHUTransactionEffectPart = CHUTransactionEffectPart{
		// types.TransactionResponseV1.Effects
		EffectsJSON:                    string(effectsJSON),
		EffectMessageVersion:           t.Effects.MessageVersion,
		Epoch:                          t.Effects.ExecutedEpoch.Uint64(),
		ModifiedAtVersions:             encodeModifiedAtVersions(t.Effects.ModifiedAtVersions),
		EventsDigest:                   digestToStringNilsafe(t.Effects.EventsDigest),
		Status:                         t.Effects.Status.Status,
		Error:                          t.Effects.Status.Error,
		GasUsedComputationCost:         t.Effects.GasUsed.ComputationCost.Uint64(),
		GasUsedStorageCost:             t.Effects.GasUsed.StorageCost.Uint64(),
		GasUsedStorageRebate:           t.Effects.GasUsed.StorageRebate.Uint64(),
		GasUsedNonRefundableStorageFee: t.Effects.GasUsed.NonRefundableStorageFee.Uint64(),
		CreatedCount:                   uint32(len(t.Effects.Created)),
		MutatedCount:                   uint32(len(t.Effects.Mutated)),
		DeletedCount:                   uint32(len(t.Effects.Deleted)),
		WrappedCount:                   uint32(len(t.Effects.Wrapped)),
		UnwrappedThenDeletedCount:      uint32(len(t.Effects.UnwrappedThenDeleted)),
	}

	// ========================================
	// txn.CHUTransactionEventPart
	eventsTxDigest := make([]string, len(t.Events))
	eventsEventSeq := make([]uint64, len(t.Events))
	eventsPackageID := make([]string, len(t.Events))
	eventsTransactionModule := make([]string, len(t.Events))
	eventsSender := make([]string, len(t.Events))
	eventsType := make([]string, len(t.Events))
	eventsRawType := make([]string, len(t.Events))
	eventsFields := make([]string, len(t.Events))
	for i, e := range t.Events {
		if e.ID.TxDigest.String() != t.Digest.String() {
			err = errors.Errorf("TxDigest of event #%d is %s, not equal to txn digest", i, e.ID.TxDigest.String())
			return
		}
		eventsTxDigest[i] = e.ID.TxDigest.String()
		eventsEventSeq[i] = e.ID.EventSeq.Uint64()
		eventsPackageID[i] = e.PackageID.String()
		eventsTransactionModule[i] = e.TransactionModule
		eventsSender[i] = e.Sender
		eventsType[i] = e.Type.String()
		eventsRawType[i] = e.Type.WithoutArgs().String()
		eventsFields[i] = string(e.Fields)
	}
	txn.CHUTransactionEventPart = CHUTransactionEventPart{
		// types.TransactionResponseV1.Events
		EventsTxDigest:          eventsTxDigest,
		EventsEventSeq:          eventsEventSeq,
		EventsPackageID:         eventsPackageID,
		EventsTransactionModule: eventsTransactionModule,
		EventsSender:            eventsSender,
		EventsType:              eventsType,
		EventsRawType:           eventsRawType,
		EventsFields:            eventsFields,
	}

	// ========================================
	// txn.CHUTransactionBalancePart
	balanceChangesOwner := make([]string, len(t.BalanceChanges))
	balanceChangesCoinType := make([]string, len(t.BalanceChanges))
	balanceChangesAmount := make([]string, len(t.BalanceChanges))
	for i, bc := range t.BalanceChanges {
		balanceChangesOwner[i] = objectOwnerString(*bc.Owner)
		balanceChangesCoinType[i] = bc.CoinType.String()
		balanceChangesAmount[i] = bc.Amount.String()
	}
	txn.CHUTransactionBalancePart = CHUTransactionBalancePart{
		// types.TransactionResponseV1.BalanceChanges
		BalanceChangesOwner:    balanceChangesOwner,
		BalanceChangesCoinType: balanceChangesCoinType,
		BalanceChangesAmount:   balanceChangesAmount,
	}

	txnBase := CHUTxnExtendBase{
		Digest:           t.Digest.String(),
		Checkpoint:       cp.SequenceNumber,
		CheckpointDigest: cp.Digest,
		Epoch:            t.Effects.ExecutedEpoch.Uint64(),
		TimestampMs:      t.TimestampMs.Uint64(),
		Timestamp:        time.UnixMilli(int64(t.TimestampMs.Uint64())),
	}

	// ========================================
	// txn.CHUTransactionInputPart & moveCalls
	if t.Transaction != nil {
		// types.TransactionResponseV1.Transaction.TxSignatures
		for i := range t.Transaction.TxSignatures {
			txn.TxSignature = append(txn.TxSignature, base64.StdEncoding.EncodeToString(t.Transaction.TxSignatures[i]))
			if types.IsMultiSigBytes(t.Transaction.TxSignatures[i]) {
				txn.HasUpgradeMultisig = 1
			}
			if types.IsZkLoginSigBytes(t.Transaction.TxSignatures[i]) {
				txn.HasZkloginSig = 1
			}
		}
		// types.TransactionResponseV1.Transaction.Data.V1
		txn.MessageVersion = "v1"
		// types.TransactionResponseV1.Transaction.Data.V1.Sender
		txn.Sender = t.Transaction.Data.V1.Sender.String()
		// types.TransactionResponseV1.Transaction.Data.V1.Expiration
		if t.Transaction.Data.V1.Expiration != nil {
			txn.ExpirationEpoch = t.Transaction.Data.V1.Expiration.Epoch
		}
		// types.TransactionResponseV1.Transaction.Data.V1.GasData
		txn.GasOwner = t.Transaction.Data.V1.GasData.Owner.String()
		txn.GasPrice = t.Transaction.Data.V1.GasData.Price.Uint64()
		txn.GasBudget = t.Transaction.Data.V1.GasData.Budget.Uint64()
		txn.GasObjectsID = make([]string, len(t.Transaction.Data.V1.GasData.Payment))
		txn.GasObjectsSequence = make([]uint64, len(t.Transaction.Data.V1.GasData.Payment))
		txn.GasObjectsDigest = make([]string, len(t.Transaction.Data.V1.GasData.Payment))
		for i, p := range t.Transaction.Data.V1.GasData.Payment {
			txn.GasObjectsID[i] = p.ObjectID.String()
			txn.GasObjectsSequence[i] = p.Version.Uint64()
			txn.GasObjectsDigest[i] = p.Digest.String()
		}
		// types.TransactionResponseV1.Transaction.Data.V1.Kind
		txKind := t.Transaction.Data.V1.Kind
		switch {
		case txKind.ProgrammableTransaction != nil:
			txn.Kind = "ProgrammableTransaction"
			txn.TransactionCount = uint32(len(txKind.ProgrammableTransaction.Commands))
			txn.InputCount = uint32(len(txKind.ProgrammableTransaction.Inputs))
			for _, arg := range txKind.ProgrammableTransaction.Inputs {
				if arg.Object != nil && arg.Object.SharedObject != nil {
					txn.SharedInputCount++
				}
			}
			for i, cmd := range txKind.ProgrammableTransaction.Commands {
				switch {
				case cmd.MoveCall != nil:
					txn.MoveCallsCount++
					moveCalls = append(moveCalls, CHUMoveCall{
						CHUTxnExtendBase: txnBase,
						Package:          cmd.MoveCall.Package.String(),
						Module:           cmd.MoveCall.Module,
						Function:         cmd.MoveCall.Function,
					})
					txn.MoveCallsPackage = append(txn.MoveCallsPackage, cmd.MoveCall.Package.String())
					txn.MoveCallsModule = append(txn.MoveCallsModule, cmd.MoveCall.Module)
					txn.MoveCallsFunction = append(txn.MoveCallsFunction, cmd.MoveCall.Function)
				case cmd.TransferObjects != nil:
					txn.TransfersCount++
				case cmd.SplitCoins != nil:
					txn.SplitCoinsCount++
					if cmd.SplitCoins.First.GasCoin != nil && *cmd.SplitCoins.First.GasCoin {
						txn.GasCoinsCount++
					}
				case cmd.MergeCoins != nil:
					txn.MergedCoinsCount++
					if cmd.MergeCoins.First.GasCoin != nil && *cmd.MergeCoins.First.GasCoin {
						txn.GasCoinsCount++
					}
				case cmd.Publish != nil:
					txn.PublishCount++
				case cmd.MakeMoveVec != nil:
					txn.MakeMoveVecCount++
				case cmd.Upgrade != nil:
					txn.UpgradeCount++
				default:
					err = errors.Errorf("unknown command kind in #%d command: %#v", i, cmd)
					return
				}
			}
		case txKind.ChangeEpoch != nil:
			txn.Kind = "ChangeEpoch"
			txn.IsSystemTx = 1
		case txKind.Genesis != nil:
			txn.Kind = "Genesis"
			txn.IsSystemTx = 1
		case txKind.ConsensusCommitPrologue != nil:
			txn.Kind = "ConsensusCommitPrologue"
			txn.IsSystemTx = 1
		case txKind.ConsensusCommitPrologueV1 != nil:
			txn.Kind = "ConsensusCommitPrologueV1"
			txn.IsSystemTx = 1
		case txKind.ConsensusCommitPrologueV2 != nil:
			txn.Kind = "ConsensusCommitPrologueV2"
			txn.IsSystemTx = 1
		case txKind.ConsensusCommitPrologueV3 != nil:
			txn.Kind = "ConsensusCommitPrologueV3"
			txn.IsSystemTx = 1
		case txKind.ConsensusCommitPrologueV4 != nil:
			txn.Kind = "ConsensusCommitPrologueV4"
			txn.IsSystemTx = 1
		case txKind.AuthenticatorStateUpdate != nil:
			txn.Kind = "AuthenticatorStateUpdate"
			txn.IsSystemTx = 1
		case txKind.EndOfEpochTransaction != nil:
			txn.Kind = "EndOfEpochTransaction"
			txn.TransactionCount = uint32(len(txKind.EndOfEpochTransaction.Transactions))
			txn.IsSystemTx = 1
		case txKind.RandomnessStateUpdate != nil:
			txn.Kind = "RandomnessStateUpdate"
			txn.IsSystemTx = 1
		default:
			err = errors.Errorf("unsupported transaction kind: %#v", txKind)
			return
		}

		// types.TransactionResponseV1.Transaction
		var transactionJSON []byte
		transactionJSON, err = json.Marshal(t.Transaction)
		if err != nil {
			err = errors.Wrapf(err, "marshal TransactionResponseV1.transaction failed")
			return
		}
		txn.TransactionJSON = string(transactionJSON)
		if txn.Sender != txn.GasOwner {
			txn.IsSponsoredTx = 1
		}
	}

	// ========================================
	// events
	for _, ev := range t.Events {
		events = append(events, CHUEvent{
			CHUTxnExtendBase: txnBase,
			EventSeq:         ev.ID.EventSeq.Uint64(),
			PackageID:        ev.PackageID.String(),
			Module:           ev.TransactionModule,
			Sender:           ev.Sender,
			Type:             ev.Type.String(),
			RawType:          ev.Type.WithoutArgs().String(),
			Fields:           string(ev.Fields),
		})
	}

	// ========================================
	// balanceChanges
	for _, bc := range t.BalanceChanges {
		balanceChanges = append(balanceChanges, CHUBalanceChange{
			CHUTxnExtendBase: txnBase,
			Owner:            objectOwnerString(*bc.Owner),
			OwnerAddress:     objectOwnerAddress(bc.Owner),
			CoinType:         bc.CoinType.String(),
			Amount:           bc.Amount.String(),
			AmountNumber:     bc.Amount.BigInt(),
		})
	}

	// ========================================
	// objectChanges
	for _, oc := range t.ObjectChanges {
		ownerType, ownerID, ownerInitialSharedVersion := oc.Owner.GetTypeAndID()
		objectChanges = append(objectChanges, CHUObjectChange{
			CHUTxnExtendBase:          txnBase,
			Type:                      string(oc.Type),
			ObjectID:                  oc.GetObjectID(),
			ObjectVersion:             oc.Version.Uint64(),
			ObjectPreviousVersion:     oc.PreviousVersion.Uint64Pointer(),
			ObjectDigest:              oc.Digest.String(),
			ObjectType:                utils.NullOrToString(oc.ObjectType),
			ObjectRawType:             utils.NullOrToString(oc.ObjectType.WithoutArgs()),
			Sender:                    utils.NullOrToString(oc.Sender),
			OwnerID:                   ownerID,
			OwnerInitialSharedVersion: ownerInitialSharedVersion,
			OwnerType:                 ownerType,
			Owner:                     utils.NullOrConvert(oc.Owner, objectOwnerString),
			Recipient:                 utils.NullOrConvert(oc.Recipient, objectOwnerString),
			Modules:                   oc.Modules,
			HasPublicTransfer:         false, // missing
			CoinType:                  "",    // missing
			CoinBalance:               0,     // missing
			StorageRebate:             0,     // missing
		})
	}

	return
}

type coinBalance struct {
	types.Number
}

func (b *coinBalance) UnmarshalJSON(raw []byte) error {
	// ignore the error if unmarshal failed
	_ = b.Number.UnmarshalJSON(raw)
	return nil
}

type objectDetail struct {
	ObjectID      string             `json:"objectId"`
	Version       types.Number       `json:"version"`
	Owner         *types.ObjectOwner `json:"owner"`
	StorageRebate types.Number       `json:"storageRebate"`
	Content       struct {
		Type              string `json:"type"`
		HasPublicTransfer bool   `json:"hasPublicTransfer"`
		Fields            struct {
			Balance coinBalance `json:"balance"`
		} `json:"fields"`
	} `json:"content"`
}

type getPastObjectRequest struct {
	ObjectID string `json:"objectId"`
	Version  string `json:"version"`
}

func getCoinType(objType string) string {
	if strings.HasPrefix(objType, "0x2::coin::Coin<") && strings.HasSuffix(objType, ">") {
		return objType[len("0x2::coin::Coin<") : len(objType)-1]
	}
	return ""
}

func (m *SlotConverter) _fillFields(
	ctx context.Context,
	data []*CHUObjectChange,
	getOpt types.SuiObjectDataOptions,
	fillFn func(*CHUObjectChange, *objectDetail),
) error {
	_, pageLogger := log.FromContext(ctx)
	pageStart := time.Now()
	var pageRequest = make([]getPastObjectRequest, len(data))
	for i, change := range data {
		pageRequest[i] = getPastObjectRequest{
			ObjectID: change.ObjectID,
			Version:  strconv.FormatUint(change.ObjectVersion, 10),
		}
	}
	var pageResponse []*types.SuiPastObjectResponse
	r := m.cli.UseClient(
		ctx,
		"converter.sui_tryMultiGetPastObjects",
		func(ctx context.Context, cli *sui.Client) clientpool.Result {
			return cli.CallContext(ctx, &pageResponse, "", "converter", "sui_tryMultiGetPastObjects", pageRequest, &getOpt)
		},
		clientpool.WithoutTags[sui.ClientConfig](clientpool.MethodNotSupportedTag("sui_tryMultiGetPastObjects")),
	)
	if r.Err != nil {
		pageLogger.Errorfe(r.Err, "multi get past objects failed (%s)", r.ConfigName)
		return errors.Wrapf(r.Err, "multi get past objects failed")
	}
	if len(pageResponse) != len(pageRequest) {
		pageLogger.Errorf("number of results %d is less than expected %d", len(pageResponse), len(pageRequest))
		return errors.Errorf("number of results %d is less than expected %d", len(pageResponse), len(pageRequest))
	}
	for j, resp := range pageResponse {
		objectSummary := fmt.Sprintf("%s/%d/%s", data[j].ObjectID, data[j].ObjectVersion, data[j].Type)
		if resp.Status != types.SuiPastObjectStatusVersionFound {
			pageLogger.Errorf("unexpected status %q for object %s", resp.Status, objectSummary)
			return errors.Errorf("unexpected status %q for object %s", resp.Status, objectSummary)
		}
		var d objectDetail
		if err := json.Unmarshal(resp.Details, &d); err != nil {
			pageLogger.Errorf("unmarshal object %s detail failed (%v): %s", objectSummary, err, string(resp.Details))
			return errors.Wrapf(err, "unmarshal object %s detail failed", objectSummary)
		}
		if d.ObjectID != data[j].ObjectID || d.Version.Uint64() != data[j].ObjectVersion {
			pageLogger.Errorf("the %d response is %s/%d not %s", j, d.ObjectID, d.Version.Uint64(), objectSummary)
			return errors.Errorf("the %d response is %s/%d not %s", j, d.ObjectID, d.Version.Uint64(), objectSummary)
		}
		fillFn(data[j], &d)
	}
	pageLogger.With("used", time.Since(pageStart).String()).Debugf("fetch detail of objects in page succeed")
	return nil
}

func (m *SlotConverter) _fillBaseFields(ctx context.Context, data []*CHUObjectChange) error {
	return m._fillFields(ctx, data, types.SuiObjectDataOptions{
		ShowOwner:         true,
		ShowStorageRebate: true,
	}, func(change *CHUObjectChange, d *objectDetail) {
		change.OwnerType, change.OwnerID, change.OwnerInitialSharedVersion = d.Owner.GetTypeAndID()
		change.StorageRebate = d.StorageRebate.Uint64()
	})
}

func (m *SlotConverter) _fillCoinFields(ctx context.Context, data []*CHUObjectChange) error {
	return m._fillFields(ctx, data, types.SuiObjectDataOptions{
		ShowContent: true,
	}, func(change *CHUObjectChange, d *objectDetail) {
		change.HasPublicTransfer = d.Content.HasPublicTransfer
		change.CoinType = getCoinType(d.Content.Type)
		change.CoinBalance = d.Content.Fields.Balance.Uint64()
	})
}

func (m *SlotConverter) fillMissingFieldsForObjectChanges(
	ctx context.Context,
	objectChanges []*CHUObjectChange,
) error {
	_, logger := log.FromContext(ctx, "total", len(objectChanges))
	start := time.Now()
	var totalCoinData int
	_, err := concurrency.TraverseByPage(
		ctx, m.fetchConcurrency, m.fetchPageSize, objectChanges,
		func(ctx context.Context, page concurrency.Page, data []*CHUObjectChange) ([]*CHUObjectChange, error) {
			if err := m._fillBaseFields(ctx, data); err != nil {
				return nil, err
			}
			var coinData []*CHUObjectChange
			for _, c := range data {
				if getCoinType(utils.EmptyStringIfNil(c.ObjectType)) != "" {
					coinData = append(coinData, c)
				}
			}
			totalCoinData += len(coinData)
			if len(coinData) > 0 {
				if err := m._fillCoinFields(ctx, coinData); err != nil {
					return nil, err
				}
			}
			return data, nil
		})
	logger = logger.With("totalCoinData", totalCoinData, "used", time.Since(start).String())
	if err != nil {
		logger.Errorfe(err, "fetch detail of objects failed")
		return err
	}
	logger.Debugf("fetch detail of objects succeed")
	return nil
}

func (m *SlotConverter) ConvertSlot(ctx context.Context, slot *sui.Slot) (CHUCheckpoint, error) {
	var checkpoint CHUCheckpoint
	for i := range slot.Transactions {
		txn, events, moveCalls, balanceChanges, objectChanges, err := m.ConvertTxn(
			&slot.SlotCheckpointInfo, &slot.Transactions[i], int32(i))
		if err != nil {
			return checkpoint, errors.Wrapf(err, "convert #%d txn %s in checkpoint %d failed",
				i, slot.Transactions[i].Digest.String(), slot.SlotCheckpointInfo.SequenceNumber)
		}
		// objectChanges always contains ObjectID、ObjectVersion、Checkpoint
		objectPositions := utils.MapSliceNoError(objectChanges, func(t CHUObjectChange) CHUObjectPosition {
			return CHUObjectPosition{
				ObjectID:      t.ObjectID,
				ObjectVersion: t.ObjectVersion,
				Checkpoint:    t.Checkpoint,
			}
		})
		checkpoint.Transactions = append(checkpoint.Transactions, txn)
		checkpoint.Events = append(checkpoint.Events, events...)
		checkpoint.MoveCalls = append(checkpoint.MoveCalls, moveCalls...)
		checkpoint.BalanceChanges = append(checkpoint.BalanceChanges, balanceChanges...)
		checkpoint.ObjectChanges = append(checkpoint.ObjectChanges, objectChanges...)
		checkpoint.ObjectPositions = append(checkpoint.ObjectPositions, objectPositions...)
	}
	if err := m.fillMissingFieldsForObjectChanges(ctx, utils.WrapPointerForArray(checkpoint.ObjectChanges)); err != nil {
		return checkpoint, errors.Wrapf(err, "convert checkpoint %d failed", slot.SequenceNumber)
	}
	return checkpoint, nil
}

func (m *SlotConverter) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *SlotConverter) Done(r rg.Range) error {
	return nil
}
