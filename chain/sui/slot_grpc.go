package sui

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

// remove bcs part in grpc checkpoint
// TODO may be need to list all fields except bcs field in readMask when calling grpc interface to get a checkpoint
func (s *Slot) removeBcs() {
	if s.GrpcCheckpoint == nil {
		return
	}
	if s.GrpcCheckpoint.Summary != nil {
		s.GrpcCheckpoint.Summary.Bcs = nil
	}
	if s.GrpcCheckpoint.Contents != nil {
		s.GrpcCheckpoint.Contents.Bcs = nil
		for _, tx := range s.GrpcCheckpoint.Contents.Transactions {
			for _, sig := range tx.Signatures {
				sig.Bcs = nil
			}
		}
	}
	for _, tx := range s.GrpcCheckpoint.Transactions {
		for _, sig := range tx.Signatures {
			sig.Bcs = nil
		}
		if tx.Transaction != nil {
			tx.Transaction.Bcs = nil
		}
		if tx.Effects != nil {
			tx.Effects.Bcs = nil
		}
		if tx.Events != nil {
			tx.Events.Bcs = nil
			for _, ev := range tx.Events.Events {
				ev.Contents = nil
			}
		}
	}
	if s.GrpcCheckpoint.Objects != nil {
		for _, obj := range s.GrpcCheckpoint.Objects.Objects {
			obj.Bcs = nil
			obj.Contents = nil
		}
	}
}

func (s *Slot) loadCheckpointInfo() {
	s.SlotCheckpointInfo.SequenceNumber = s.GrpcCheckpoint.GetSummary().GetSequenceNumber()
	s.SlotCheckpointInfo.Digest = s.GrpcCheckpoint.GetDigest()
	s.SlotCheckpointInfo.TimestampMs = types.Uint64ToNumber(
		uint64(s.GrpcCheckpoint.GetSummary().GetTimestamp().AsTime().UnixMilli()))
	s.SlotCheckpointInfo.TransactionDigests = utils.MapSliceNoError(
		s.GrpcCheckpoint.GetTransactions(), (*rpcv2.ExecutedTransaction).GetDigest)
}

func (s *Slot) loadTransactions() error {
	objectDict := make(map[string]map[uint64]*rpcv2.Object)
	for _, obj := range s.GrpcCheckpoint.GetObjects().GetObjects() {
		utils.PutIntoK2Map(objectDict, obj.GetObjectId(), obj.GetVersion(), obj)
	}
	for txIndex, tx := range s.GrpcCheckpoint.GetTransactions() {
		r, err := BuildTransactionResponseV1(
			s.GrpcCheckpoint.GetSequenceNumber(),
			s.GrpcCheckpoint.GetSummary().GetTimestamp().AsTime(),
			txIndex,
			tx,
			true,
			objectDict)
		if err != nil {
			return err
		}
		s.Transactions = append(s.Transactions, r)
	}
	return nil
}

func convertOwner(owner *rpcv2.Owner) *types.ObjectOwner {
	var ownerType string
	switch owner.GetKind() {
	case rpcv2.Owner_ADDRESS:
		ownerType = types.OwnerTypeAddress
	case rpcv2.Owner_OBJECT:
		ownerType = types.OwnerTypeObject
	case rpcv2.Owner_IMMUTABLE:
		ownerType = types.OwnerTypeSingle
	case rpcv2.Owner_SHARED:
		ownerType = types.OwnerTypeShared
	case rpcv2.Owner_CONSENSUS_ADDRESS:
		ownerType = types.OwnerTypeConsensusAddress
	default:
		ownerType = types.OwnerTypeSpecial
	}
	return types.BuildObjectOwner(owner.GetAddress(), ownerType, owner.GetVersion())
}

func objectRefLegacyConverter(useInput bool, backupVersion uint64) func(co *rpcv2.ChangedObject) types.ObjectRefLegacy {
	return func(co *rpcv2.ChangedObject) types.ObjectRefLegacy {
		version := utils.Select(useInput, co.GetInputVersion(), co.GetOutputVersion())
		if version == 0 {
			version = backupVersion
		}
		return types.ObjectRefLegacy{
			ObjectID: types.StrToObjectIDMust(co.GetObjectId()),
			Version:  types.Uint64ToNumber(version),
			Digest:   types.StrToDigestOrEmptyMust(utils.Select(useInput, co.GetInputDigest(), co.GetOutputDigest())),
		}
	}
}

func convertOwnedObjectRef(co *rpcv2.ChangedObject) types.OwnedObjectRef {
	return types.OwnedObjectRef{
		Owner: convertOwner(co.GetOutputOwner()),
		Reference: &types.ObjectRefLegacy{
			ObjectID: types.StrToObjectIDMust(co.GetObjectId()),
			Version:  types.Uint64ToNumber(co.GetOutputVersion()),
			Digest:   types.StrToDigestMust(co.GetOutputDigest()),
		},
	}
}

func convertOwnedObjectRefPointer(co *rpcv2.ChangedObject) *types.OwnedObjectRef {
	if co == nil {
		return nil
	}
	ref := convertOwnedObjectRef(co)
	return &ref
}

func changedObjectFilter(types ...types.ObjectChangeType) func(co *rpcv2.ChangedObject) bool {
	return func(co *rpcv2.ChangedObject) bool {
		return utils.IndexOf(types, GetChangeType(co)) >= 0
	}
}

func convertArgument(title string, arg *rpcv2.Argument) types.Argument {
	switch arg.GetKind() {
	case rpcv2.Argument_GAS:
		t := true
		return types.Argument{GasCoin: &t}
	case rpcv2.Argument_INPUT:
		in := uint16(arg.GetInput())
		return types.Argument{Input: &in}
	case rpcv2.Argument_RESULT:
		if arg.Subresult == nil {
			result := uint16(arg.GetResult())
			return types.Argument{Result: &result}
		}
		return types.Argument{NestedResult: []uint16{uint16(arg.GetResult()), uint16(arg.GetSubresult())}}
	default:
		panic(errors.Errorf("%s has unknown argument kind %q", title, arg.GetKind().String()))
	}
}

func convertArguments(title string, args []*rpcv2.Argument) []types.Argument {
	return utils.MapSliceNoErrWithIndex(args,
		func(i int, arg *rpcv2.Argument) (types.Argument, bool) {
			return convertArgument(fmt.Sprintf("#%d %s", i, title), arg), true
		})
}

func convertTransactionKind(tx *rpcv2.TransactionKind) *types.TransactionKind {
	var r types.TransactionKind
	switch tx.GetKind() {
	case rpcv2.TransactionKind_PROGRAMMABLE_TRANSACTION, rpcv2.TransactionKind_PROGRAMMABLE_SYSTEM_TRANSACTION:
		r.ProgrammableTransaction = &types.ProgrammableTransaction{
			Inputs: utils.MapSliceNoErrWithIndex(tx.GetProgrammableTransaction().GetInputs(),
				func(i int, input *rpcv2.Input) (types.CallArg, bool) {
					defer func() {
						if panicErr := recover(); panicErr != nil {
							if err, is := panicErr.(error); is {
								panic(errors.Wrapf(err, "#%d input is invalid", i))
							}
							panic(errors.Errorf("#%d input is invalid: %v", i, panicErr))
						}
					}()
					switch input.GetKind() {
					case rpcv2.Input_PURE:
						return types.CallArg{
							Pure: &types.PureValue{
								Value: input.GetPure(),
								// No value type information in grpc input data structure
							},
						}, true
					case rpcv2.Input_IMMUTABLE_OR_OWNED:
						return types.CallArg{
							Object: &types.ObjectArg{
								ImmOrOwnedObject: &types.ObjectRef{
									ObjectID: types.StrToObjectIDMust(input.GetObjectId()),
									Version:  types.Uint64ToNumber(input.GetVersion()),
									Digest:   types.StrToDigestMust(input.GetDigest()),
								},
							},
						}, true
					case rpcv2.Input_SHARED:
						return types.CallArg{
							Object: &types.ObjectArg{
								SharedObject: &types.SharedObject{
									ObjectID:             types.StrToObjectIDMust(input.GetObjectId()),
									InitialSharedVersion: types.Uint64ToNumber(input.GetVersion()),
									Mutable:              input.GetMutable(),
								},
							},
						}, true
					case rpcv2.Input_RECEIVING:
						return types.CallArg{
							Object: &types.ObjectArg{
								Receiving: &types.ObjectRef{
									ObjectID: types.StrToObjectIDMust(input.GetObjectId()),
									Version:  types.Uint64ToNumber(input.GetVersion()),
									Digest:   types.StrToDigestMust(input.GetDigest()),
								},
							},
						}, true
					case rpcv2.Input_FUNDS_WITHDRAWAL:
						// types.CallArg cannot cover this kind of input, just ignore it
						return types.CallArg{}, false
					default:
						panic(errors.Errorf("kind %s is unknown", input.GetKind()))
					}
				}),
			Commands: utils.MapSliceNoErrWithIndex(tx.GetProgrammableTransaction().GetCommands(),
				func(i int, cmd *rpcv2.Command) (types.Command, bool) {
					defer func() {
						if panicErr := recover(); panicErr != nil {
							if err, is := panicErr.(error); is {
								panic(errors.Wrapf(err, "#%d command is invalid", i))
							}
							panic(errors.Errorf("#%d command is invalid: %v", i, panicErr))
						}
					}()
					switch {
					case cmd.GetMoveCall() != nil:
						mc := cmd.GetMoveCall()
						return types.Command{
							MoveCall: &types.MoveCall{
								Package:  types.StrToObjectIDMust(mc.GetPackage()),
								Module:   mc.GetModule(),
								Function: mc.GetFunction(),
								TypeArgs: utils.MapSliceNoError(mc.GetTypeArguments(), types.TypeTagFromStringMust),
								Args:     convertArguments("move call argument", mc.GetArguments()),
							},
						}, true
					case cmd.GetTransferObjects() != nil:
						return types.Command{
							TransferObjects: &types.ArgumentsM2O{
								Oprands: convertArguments("transfer objects", cmd.GetTransferObjects().GetObjects()),
								Last:    convertArgument("transfer target address", cmd.GetTransferObjects().GetAddress()),
							},
						}, true
					case cmd.GetSplitCoins() != nil:
						return types.Command{
							SplitCoins: &types.ArgumentsO2M{
								First:   convertArgument("target split coin", cmd.GetSplitCoins().GetCoin()),
								Oprands: convertArguments("split coin amounts", cmd.GetSplitCoins().GetAmounts()),
							},
						}, true
					case cmd.GetMergeCoins() != nil:
						return types.Command{
							MergeCoins: &types.ArgumentsO2M{
								First:   convertArgument("merge coin result", cmd.GetMergeCoins().GetCoin()),
								Oprands: convertArguments("coins to merge", cmd.GetMergeCoins().GetCoinsToMerge()),
							},
						}, true
					case cmd.GetPublish() != nil:
						return types.Command{
							Publish: &types.Publish{
								ObjectIDs: utils.MapSliceNoError(cmd.GetPublish().GetDependencies(), types.StrToObjectIDMust),
								// types.Publish.Package is big and useless
							},
						}, true
					case cmd.GetMakeMoveVector() != nil:
						var typeTag *types.TypeTag
						if cmd.GetMakeMoveVector().ElementType != nil {
							typeTag = utils.WrapPointer(types.TypeTagFromStringMust(cmd.GetMakeMoveVector().GetElementType()))
						}
						return types.Command{
							MakeMoveVec: &types.MakeMoveVec{
								TypeTag: typeTag,
								Args:    convertArguments("make move vector element", cmd.GetMakeMoveVector().GetElements()),
							},
						}, true
					case cmd.GetUpgrade() != nil:
						return types.Command{
							Upgrade: &types.Upgrade{
								TransitiveDeps: utils.MapSliceNoError(
									cmd.GetUpgrade().GetDependencies(),
									types.StrToObjectIDMust,
								),
								CurrentPackageObjectID: types.StrToObjectIDMust(cmd.GetUpgrade().GetPackage()),
								Argument:               convertArgument("upgrade ticket", cmd.GetUpgrade().GetTicket()),
								// types.Upgrade.Package is big and useless
							},
						}, true
					default:
						panic(errors.Errorf("kind is unknown"))
					}
				}),
		}
	case rpcv2.TransactionKind_CHANGE_EPOCH:
		ce := tx.GetChangeEpoch()
		r.ChangeEpoch = &types.ChangeEpoch{
			Epoch:                   types.Uint64ToNumber(ce.GetEpoch()),
			ProtocolVersion:         types.Uint64ToNumber(ce.GetProtocolVersion()),
			StorageCharge:           types.Uint64ToNumber(ce.GetStorageCharge()),
			ComputationCharge:       types.Uint64ToNumber(ce.GetComputationCharge()),
			StorageRebate:           types.Uint64ToNumber(ce.GetStorageRebate()),
			NonRefundableStorageFee: types.Uint64ToNumber(ce.GetNonRefundableStorageFee()),
			EpochStartTimestampMs:   types.Int64ToNumber(ce.GetEpochStartTimestamp().AsTime().UnixMilli()),
			// types.ChangeEpoch.SystemPackages is big and useless
		}
	case rpcv2.TransactionKind_GENESIS:
		r.Genesis = &types.Genesis{}
	case rpcv2.TransactionKind_CONSENSUS_COMMIT_PROLOGUE_V1:
		ccp := tx.GetConsensusCommitPrologue()
		r.ConsensusCommitPrologue = &types.ConsensusCommitPrologue{
			Epoch:             types.Uint64ToNumber(ccp.GetEpoch()),
			Round:             types.Uint64ToNumber(ccp.GetRound()),
			CommitTimestampMs: types.Int64ToNumber(ccp.GetCommitTimestamp().AsTime().UnixMilli()),
		}
	case rpcv2.TransactionKind_AUTHENTICATOR_STATE_UPDATE:
		asu := tx.GetAuthenticatorStateUpdate()
		r.AuthenticatorStateUpdate = &types.AuthenticatorStateUpdate{
			Epoch: types.Uint64ToNumber(asu.GetEpoch()),
			Round: types.Uint64ToNumber(asu.GetRound()),
			NewActiveJwks: utils.MapSliceNoError(asu.GetNewActiveJwks(),
				func(naj *rpcv2.ActiveJwk) (res types.ActiveJwk) {
					res.JwkID.Iss = naj.GetId().GetIss()
					res.JwkID.Kid = naj.GetId().GetKid()
					res.Jwk.Kty = naj.GetJwk().GetKty()
					res.Jwk.E = naj.GetJwk().GetE()
					res.Jwk.N = naj.GetJwk().GetN()
					res.Jwk.Alg = naj.GetJwk().GetAlg()
					res.Epoch = types.Uint64ToNumber(naj.GetEpoch())
					return res
				}),
			AuthenticatorObjInitialSharedVersion: asu.GetAuthenticatorObjectInitialSharedVersion(),
		}
	case rpcv2.TransactionKind_END_OF_EPOCH:
		r.EndOfEpochTransaction = &types.EndOfEpochTransaction{
			Transactions: utils.MapSliceNoErrWithIndex(tx.GetEndOfEpoch().GetTransactions(),
				func(i int, etx *rpcv2.EndOfEpochTransactionKind) (types.EndOfEpochTransactionSingle, bool) {
					switch etx.GetKind() {
					case rpcv2.EndOfEpochTransactionKind_CHANGE_EPOCH:
						ce := etx.GetChangeEpoch()
						return types.EndOfEpochTransactionSingle{
							ChangeEpoch: &types.ChangeEpoch{
								Epoch:                   types.Uint64ToNumber(ce.GetEpoch()),
								ProtocolVersion:         types.Uint64ToNumber(ce.GetProtocolVersion()),
								StorageCharge:           types.Uint64ToNumber(ce.GetStorageCharge()),
								ComputationCharge:       types.Uint64ToNumber(ce.GetComputationCharge()),
								StorageRebate:           types.Uint64ToNumber(ce.GetStorageRebate()),
								NonRefundableStorageFee: types.Uint64ToNumber(ce.GetNonRefundableStorageFee()),
								EpochStartTimestampMs:   types.Int64ToNumber(ce.GetEpochStartTimestamp().AsTime().UnixMilli()),
								// types.ChangeEpoch.SystemPackages is big and useless
							},
						}, true
					case rpcv2.EndOfEpochTransactionKind_AUTHENTICATOR_STATE_CREATE:
						return types.EndOfEpochTransactionSingle{
							AuthenticatorStateCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_AUTHENTICATOR_STATE_EXPIRE:
						ase := etx.GetAuthenticatorStateExpire()
						return types.EndOfEpochTransactionSingle{
							AuthenticatorStateExpire: &types.AuthenticatorStateExpire{
								MinEpoch:                             types.Uint64ToNumber(ase.GetMinEpoch()),
								AuthenticatorObjInitialSharedVersion: ase.GetAuthenticatorObjectInitialSharedVersion(),
							},
						}, true
					case rpcv2.EndOfEpochTransactionKind_RANDOMNESS_STATE_CREATE:
						return types.EndOfEpochTransactionSingle{
							RandomnessStateCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_DENY_LIST_STATE_CREATE:
						return types.EndOfEpochTransactionSingle{
							CoinDenyListStateCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_BRIDGE_STATE_CREATE:
						bridgeChainID := etx.GetBridgeChainId()
						return types.EndOfEpochTransactionSingle{
							BridgeStateCreate: &bridgeChainID,
						}, true
					case rpcv2.EndOfEpochTransactionKind_BRIDGE_COMMITTEE_INIT:
						bridgeObjectVer := int64(etx.GetBridgeObjectVersion())
						return types.EndOfEpochTransactionSingle{
							BridgeCommitteeUpdate: &bridgeObjectVer,
						}, true
					case rpcv2.EndOfEpochTransactionKind_STORE_EXECUTION_TIME_OBSERVATIONS:
						return types.EndOfEpochTransactionSingle{
							StoreExecutionTimeObservations: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_ACCUMULATOR_ROOT_CREATE:
						return types.EndOfEpochTransactionSingle{
							AccumulatorRootCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_COIN_REGISTRY_CREATE:
						return types.EndOfEpochTransactionSingle{
							CoinRegistryCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_DISPLAY_REGISTRY_CREATE:
						return types.EndOfEpochTransactionSingle{
							DisplayRegistryCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_ADDRESS_ALIAS_STATE_CREATE:
						return types.EndOfEpochTransactionSingle{
							AddressAliasStateCreate: &struct{}{},
						}, true
					case rpcv2.EndOfEpochTransactionKind_WRITE_ACCUMULATOR_STORAGE_COST:
						return types.EndOfEpochTransactionSingle{
							WriteAccumulatorStorageCost: &struct{}{},
						}, true
					default:
						panic(errors.Errorf("#%d EndOfEpochTransaction has unknown kind %s", i, etx.GetKind()))
					}
				}),
		}
	case rpcv2.TransactionKind_RANDOMNESS_STATE_UPDATE:
		r.RandomnessStateUpdate = &types.RandomnessStateUpdate{}
	case rpcv2.TransactionKind_CONSENSUS_COMMIT_PROLOGUE_V2:
		ccp := tx.GetConsensusCommitPrologue()
		r.ConsensusCommitPrologueV2 = &types.ConsensusCommitPrologueV2{
			Epoch:                 types.Uint64ToNumber(ccp.GetEpoch()),
			Round:                 types.Uint64ToNumber(ccp.GetRound()),
			CommitTimestampMs:     types.Int64ToNumber(ccp.GetCommitTimestamp().AsTime().UnixMilli()),
			ConsensusCommitDigest: types.StrToDigestMust(ccp.GetConsensusCommitDigest()),
		}
	case rpcv2.TransactionKind_CONSENSUS_COMMIT_PROLOGUE_V3:
		ccp := tx.GetConsensusCommitPrologue()
		r.ConsensusCommitPrologueV3 = &types.ConsensusCommitPrologueV3{
			Epoch:                 types.Uint64ToNumber(ccp.GetEpoch()),
			Round:                 types.Uint64ToNumber(ccp.GetRound()),
			SubDagIndex:           types.PUint64ToPNumber(ccp.SubDagIndex),
			CommitTimestampMs:     types.Int64ToNumber(ccp.GetCommitTimestamp().AsTime().UnixMilli()),
			ConsensusCommitDigest: types.StrToDigestMust(ccp.GetConsensusCommitDigest()),
		}
	case rpcv2.TransactionKind_CONSENSUS_COMMIT_PROLOGUE_V4:
		ccp := tx.GetConsensusCommitPrologue()
		r.ConsensusCommitPrologueV4 = &types.ConsensusCommitPrologueV4{
			Epoch:                 types.Uint64ToNumber(ccp.GetEpoch()),
			Round:                 types.Uint64ToNumber(ccp.GetRound()),
			SubDagIndex:           types.PUint64ToPNumber(ccp.SubDagIndex),
			CommitTimestampMs:     types.Int64ToNumber(ccp.GetCommitTimestamp().AsTime().UnixMilli()),
			ConsensusCommitDigest: types.StrToDigestMust(ccp.GetConsensusCommitDigest()),
			AdditionalStateDigest: types.StrToDigestMust(ccp.GetAdditionalStateDigest()),
		}
	default:
		panic(fmt.Sprintf("unknown transaction kind %s", tx.GetKind()))
	}
	return &r
}

func convertTransactionExpiration(te *rpcv2.TransactionExpiration) *types.TransactionExpiration {
	if te.GetKind() == rpcv2.TransactionExpiration_NONE {
		return &types.TransactionExpiration{None: &struct{}{}}
	}
	epoch := te.GetEpoch()
	return &types.TransactionExpiration{Epoch: &epoch}
}

func BuildTransactionResponseV1(
	checkpoint uint64,
	timestamp time.Time,
	txIndex int,
	tx *rpcv2.ExecutedTransaction,
	withObjectChanges bool,
	objectDict map[string]map[uint64]*rpcv2.Object, // required if withObjectChanges
) (r types.TransactionResponseV1, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			var is bool
			if err, is = panicErr.(error); !is {
				err = errors.Errorf("%v", panicErr)
			}
		}
		if err != nil {
			err = errors.Wrapf(err, "convert transaction #%d/%s in checkpoint %d failed", txIndex, tx.GetDigest(), checkpoint)
		}
	}()
	effects := tx.GetEffects()
	// TimestampMs
	r.TimestampMs = types.Int64ToNumber(timestamp.UnixMilli())
	// CheckpointStub
	r.CheckpointStub.Checkpoint = types.Uint64ToNumber(checkpoint)
	r.CheckpointStub.CheckpointTimestampMs = &r.TimestampMs
	r.CheckpointStub.TransactionPosition = txIndex
	// Digest
	r.Digest = types.StrToDigestMust(tx.GetDigest())
	// Transaction
	r.Transaction = &types.SenderSignedTransaction{
		Data: &types.TransactionData{
			V1: &types.TransactionDataV1{
				Kind:   convertTransactionKind(tx.GetTransaction().GetKind()),
				Sender: types.StrToAddressMust(tx.GetTransaction().GetSender()),
				GasData: &types.GasData{
					Payment: utils.MapSliceNoError(tx.GetTransaction().GetGasPayment().GetObjects(),
						func(obj *rpcv2.ObjectReference) types.ObjectRefLegacy {
							return types.ObjectRefLegacy{
								ObjectID: types.StrToObjectIDMust(obj.GetObjectId()),
								Version:  types.Uint64ToNumber(obj.GetVersion()),
								Digest:   types.StrToDigestMust(obj.GetDigest()),
							}
						}),
					Owner:  types.StrToAddressMust(tx.GetTransaction().GetGasPayment().GetOwner()),
					Price:  types.Uint64ToNumber(tx.GetTransaction().GetGasPayment().GetPrice()),
					Budget: types.Uint64ToNumber(tx.GetTransaction().GetGasPayment().GetBudget()),
				},
				Expiration: convertTransactionExpiration(tx.GetTransaction().GetExpiration()),
			},
		},
		TxSignatures: utils.MapSliceNoError(tx.GetSignatures(), func(sig *rpcv2.UserSignature) types.Signature {
			return sig.GetBcs().GetValue()
		}),
	}

	// ObjectChanges
	if withObjectChanges {
		for _, co := range effects.GetChangedObjects() {
			changeType := GetChangeType(co)
			if changeType == types.ObjectChangeTypeUnknown || changeType == types.ObjectChangeTypeAccumulatorWrite {
				// just ignore it
				continue
			}
			if changeType == types.ObjectChangeTypeUnwrappedThenDeleted {
				// The unwrapTheDeleted record lacks various details, including digest, version, owner, type,
				// so ignore it here.
				continue
			}
			objectID := types.StrToObjectIDMust(co.GetObjectId())
			version := co.GetOutputVersion()
			if version == 0 {
				version = effects.GetLamportVersion()
			}
			var preVersion *types.Number
			if !changeType.IsCreated() {
				preVersion = utils.WrapPointer(types.Uint64ToNumber(co.GetInputVersion()))
			}
			var sender *types.Address
			if tx.GetTransaction().GetSender() != "" {
				sender = utils.WrapPointer(types.StrToAddressMust(tx.GetTransaction().GetSender()))
			}
			fullObjVersion := utils.Select(changeType.IsDeleted(), co.GetInputVersion(), version)
			fullObj, has := utils.GetFromK2Map(objectDict, co.GetObjectId(), fullObjVersion)
			if !has {
				return r, errors.Errorf("object %s/%d in checkpoint %d not found",
					co.GetObjectId(), fullObjVersion, checkpoint)
			}
			r.ObjectChanges = append(r.ObjectChanges, types.ObjectChange{
				Type:            changeType,
				Digest:          types.StrToDigestOrEmptyMust(co.GetOutputDigest()), // co.GetOutputDigest() will be empty if changeType.IsDeleted()
				Version:         types.Uint64ToNumber(version),
				PreviousVersion: preVersion,
				Sender:          sender,
				ObjectID:        &objectID,
				ObjectType:      types.TypeTagFromStringOrNil(fullObj.GetObjectType()),
				Recipient:       nil,
				Owner:           convertOwner(fullObj.GetOwner()),
				Modules:         utils.MapSliceNoError(fullObj.GetPackage().GetModules(), (*rpcv2.Module).GetName),
				PackageID:       utils.Select(changeType == types.ObjectChangeTypePublished, &objectID, nil),
			})
		}
	}
	// Effects
	r.Effects = &types.TransactionEffectsV1{
		MessageVersion: "v1",
		Status: types.TransactionStatus{
			Status: utils.Select(effects.GetStatus().GetSuccess(), "success", "failure"),
			Error:  effects.GetStatus().GetError().GetDescription(),
		},
		ExecutedEpoch: types.Uint64ToNumber(effects.GetEpoch()),
		GasUsed: &types.GasCostSummary{
			ComputationCost:         types.Uint64ToNumber(effects.GetGasUsed().GetComputationCost()),
			StorageCost:             types.Uint64ToNumber(effects.GetGasUsed().GetStorageCost()),
			StorageRebate:           types.Uint64ToNumber(effects.GetGasUsed().GetStorageRebate()),
			NonRefundableStorageFee: types.Uint64ToNumber(effects.GetGasUsed().GetNonRefundableStorageFee()),
		},
		ModifiedAtVersions: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), func(co *rpcv2.ChangedObject) bool {
				return co.GetInputState() == rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS
			}),
			func(co *rpcv2.ChangedObject) types.ObjectIDAndSeq {
				return types.ObjectIDAndSeq{
					ObjectID:       types.StrToObjectIDMust(co.GetObjectId()),
					SequenceNumber: types.Uint64ToNumber(co.GetInputVersion()),
				}
			},
		),
		SharedObjects: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), func(co *rpcv2.ChangedObject) bool {
				return co.GetInputState() == rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS &&
					co.GetInputOwner().GetKind() == rpcv2.Owner_SHARED
			}),
			objectRefLegacyConverter(true, effects.GetLamportVersion()),
		),
		TransactionDigest: types.StrToDigestMust(effects.GetTransactionDigest()),
		Created: utils.MapSliceNoError(
			utils.FilterArr(
				effects.GetChangedObjects(),
				changedObjectFilter(types.ObjectChangeTypeCreated, types.ObjectChangeTypePublished),
			),
			convertOwnedObjectRef,
		),
		Mutated: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), changedObjectFilter(types.ObjectChangeTypeMutated)),
			convertOwnedObjectRef,
		),
		Unwrapped: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), changedObjectFilter(types.ObjectChangeTypeUnwrapped)),
			convertOwnedObjectRef,
		),
		Deleted: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), changedObjectFilter(types.ObjectChangeTypeDeleted)),
			objectRefLegacyConverter(false, effects.GetLamportVersion()),
		),
		UnwrappedThenDeleted: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), changedObjectFilter(types.ObjectChangeTypeUnwrappedThenDeleted)),
			objectRefLegacyConverter(false, effects.GetLamportVersion()),
		),
		Wrapped: utils.MapSliceNoError(
			utils.FilterArr(effects.GetChangedObjects(), changedObjectFilter(types.ObjectChangeTypeWrapped)),
			objectRefLegacyConverter(false, effects.GetLamportVersion()),
		),
		GasObject:    convertOwnedObjectRefPointer(effects.GetGasObject()),
		EventsDigest: types.StrToDigestPointerMust(effects.GetEventsDigest()),
		Dependencies: effects.GetDependencies(),
	}
	// Events
	for ei, ev := range tx.GetEvents().GetEvents() {
		r.Events = append(r.Events, types.Event{
			ID: types.EventID{
				TxDigest: types.StrToDigestMust(tx.GetDigest()),
				EventSeq: types.Uint64ToNumber(uint64(ei)),
			},
			PackageID:         types.StrToObjectIDMust(ev.GetPackageId()),
			TransactionModule: ev.GetModule(),
			Sender:            ev.GetSender(),
			Type:              types.TypeTagFromStringMust(ev.GetEventType()),
			Fields:            json.RawMessage(utils.MustJSONMarshal(ev.GetJson())),
		})
	}
	// BalanceChanges
	for _, bc := range tx.GetBalanceChanges() {
		r.BalanceChanges = append(r.BalanceChanges, types.BalanceChange{
			Owner:    types.BuildObjectOwner(bc.GetAddress(), types.OwnerTypeAddress, 0),
			CoinType: utils.WrapPointer(types.TypeTagFromStringMust(bc.GetCoinType())),
			Amount:   types.StringToNumber(bc.GetAmount()),
		})
	}
	return r, nil
}

func GetChangeType(co *rpcv2.ChangedObject) types.ObjectChangeType {
	if co.GetOutputState() == rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_UNKNOWN ||
		co.GetInputState() == rpcv2.ChangedObject_INPUT_OBJECT_STATE_UNKNOWN {
		return types.ObjectChangeTypeUnknown
	}
	if co.GetOutputState() == rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_ACCUMULATOR_WRITE {
		return types.ObjectChangeTypeAccumulatorWrite
	}
	if co.GetInputState() == rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST {
		if co.GetOutputState() == rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_DOES_NOT_EXIST {
			return types.ObjectChangeTypeUnwrappedThenDeleted
		} else if co.GetIdOperation() == rpcv2.ChangedObject_NONE {
			return types.ObjectChangeTypeUnwrapped
		} else if co.GetOutputState() == rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE {
			return types.ObjectChangeTypeCreated
		} else {
			return types.ObjectChangeTypePublished
		}
	} else if co.GetOutputState() == rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_DOES_NOT_EXIST {
		if co.GetIdOperation() == rpcv2.ChangedObject_NONE {
			return types.ObjectChangeTypeWrapped
		} else {
			return types.ObjectChangeTypeDeleted
		}
	} else {
		return types.ObjectChangeTypeMutated
	}
}
