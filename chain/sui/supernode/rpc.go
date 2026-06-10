package supernode

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/kvstore"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
)

func NewSuperNode(
	superSvr *SuperService,
	client *sui.ClientPool,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case "sui_getLatestCheckpointSequenceNumber": // DriverVersion[0]
					return superSvr.GetLatestCheckpointSequenceNumber(ctx)
				case "sui_getCheckpointTime": // DriverVersion[0]
					return jsonrpc.CallMethod(superSvr.GetCheckpointTime, ctx, params)
				case "sui_getTransactions": // DriverVersion[0]
					return jsonrpc.CallMethod(superSvr.GetTransactions, ctx, params)
				case "sui_filterObjectChanges": // DriverVersion[0]
					return jsonrpc.CallMethod(superSvr.FilterObjectChanges, ctx, params)
				case "sui_getLatestSimpleCheckpoint": // DriverVersion[1,2]
					return jsonrpc.CallMethod(superSvr.GetLatestSimpleCheckpoint, ctx, params)
				case "sui_getSimpleCheckpoint": // DriverVersion[1,2]
					return jsonrpc.CallMethod(superSvr.GetSimpleCheckpoint, ctx, params)
				case "sui_getTransactionsV2": // DriverVersion[1]
					return jsonrpc.CallMethod(superSvr.GetTransactionsV2, ctx, params)
				case "sui_filterObjectChangesV2": // DriverVersion[1]
					return jsonrpc.CallMethod(superSvr.FilterObjectChangesV2, ctx, params)
				case "sui_getObjectCreation": // DriverVersion[0,1,2]
					return jsonrpc.CallMethod(superSvr.GetObjectCreation, ctx, params)
				case "sui_getObjectStat": // DriverVersion[0,1,2]
					return jsonrpc.CallMethod(superSvr.GetObjectStat, ctx, params)
				case "sui_getObjectsStat": // DriverVersion[0,1,2]
					return jsonrpc.CallMethod(superSvr.GetObjectsStat, ctx, params)
				case "sui_getGrpcTransactions": // DriverVersion[2]
					return jsonrpc.CallMethod(superSvr.GetGrpcTransactions, ctx, params)
				case "sui_filterGrpcChangedObjects": // DriverVersion[2]
					return jsonrpc.CallMethod(superSvr.FilterGrpcChangedObjects, ctx, params)
				case "sui_getGrpcObjects": // DriverVersion[2]
					return jsonrpc.CallMethod(superSvr.GetGrpcObjects, ctx, params)
				default:
					return next(ctx, method, params)
				}
			}
		},
		jsonrpc.NewJSONRPCProxyMiddleware(client.ClientPool),
	}
}

type SuperService struct {
	client                 *sui.ClientPool
	slotCache              chain.LatestSlotCache[*sui.Slot]
	cachedSimpleCheckpoint kvstore.Store[sui.SimpleCheckpoint]
	cachedCheckpointTime   kvstore.Store[sui.CheckpointTime]
	cachedObjectCreation   kvstore.Store[sui.ObjectCreation]
	storageJSONRPC         StorageJSONRPC
	storageGRPC            StorageGRPC
	// storageShared serves the format-agnostic queries; it is whichever of
	// storageGRPC / storageJSONRPC is configured (grpc preferred when both exist).
	storageShared StorageShared
}

func NewSuperService(
	client *sui.ClientPool,
	slotCache chain.LatestSlotCache[*sui.Slot],
	cachedSimpleCheckpoint kvstore.Store[sui.SimpleCheckpoint],
	cachedCheckpointTime kvstore.Store[sui.CheckpointTime],
	cachedObjectCreation kvstore.Store[sui.ObjectCreation],
	storageJSONRPC StorageJSONRPC,
	storageGRPC StorageGRPC,
) *SuperService {
	// A chain may have only grpc data, only json-rpc data (e.g. iota), or both,
	// but it must have at least one.
	var storageShared StorageShared
	switch {
	case storageGRPC != nil:
		storageShared = storageGRPC
	case storageJSONRPC != nil:
		storageShared = storageJSONRPC
	default:
		panic("supernode: at least one of storageJSONRPC / storageGRPC must be provided")
	}
	return &SuperService{
		client:                 client,
		slotCache:              slotCache,
		cachedSimpleCheckpoint: cachedSimpleCheckpoint,
		cachedCheckpointTime:   cachedCheckpointTime,
		cachedObjectCreation:   cachedObjectCreation,
		storageJSONRPC:         storageJSONRPC,
		storageGRPC:            storageGRPC,
		storageShared:          storageShared,
	}
}

// requireJSONRPC guards json-rpc-format methods on chains that have no json-rpc storage.
func (s *SuperService) requireJSONRPC() error {
	if s.storageJSONRPC == nil {
		return errors.Errorf("json-rpc storage is not configured for this chain")
	}
	return nil
}

// requireGRPC guards grpc-format methods on chains that have no grpc storage (e.g. iota).
func (s *SuperService) requireGRPC() error {
	if s.storageGRPC == nil {
		return errors.Errorf("grpc storage is not configured for this chain")
	}
	return nil
}

func (s *SuperService) GetLatestCheckpointSequenceNumber(ctx context.Context) (types.Number, error) {
	cur, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return types.Uint64ToNumber(0), err
	}
	return types.Uint64ToNumber(*cur.End), nil
}

func (s *SuperService) GetLatestSimpleCheckpoint(
	ctx context.Context,
	checkpointGt uint64,
) (sui.GetLatestSimpleCheckpointResponse, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp := sui.GetLatestSimpleCheckpointResponse{APIVersion: sui.APIVersion}
	latest, err := s.slotCache.Wait(ctx, checkpointGt)
	if err != nil {
		return resp, err
	}
	latestSlot, err := s.slotCache.GetByNumber(ctx, latest)
	if err != nil {
		return resp, err
	}
	resp.Checkpoint = sui.NewSimpleCheckpoint(latestSlot)
	return resp, nil
}

func (s *SuperService) GetSimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error) {
	ctx, logger := log.FromContext(ctx, "checkpoint", checkpoint)
	key := strconv.FormatUint(checkpoint, 10)
	if cached, err := s.cachedSimpleCheckpoint.Get(ctx, key); err != nil {
		logger.Errorfe(err, "get simple checkpoint from cache failed")
		return sui.SimpleCheckpoint{}, err
	} else if sc, has := cached[key]; has {
		return sc, nil
	}
	scs, err := chain.QueryRangeWithCache(
		ctx,
		rg.NewSingleRange(checkpoint),
		s.slotCache,
		func(slot *sui.Slot) ([]sui.SimpleCheckpoint, error) {
			return []sui.SimpleCheckpoint{sui.NewSimpleCheckpoint(slot)}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]sui.SimpleCheckpoint, error) {
			sc, queryErr := s.storageShared.QuerySimpleCheckpoint(ctx, queryRange.Start)
			if queryErr != nil {
				return nil, queryErr
			}
			return []sui.SimpleCheckpoint{sc}, nil
		},
	)
	if err != nil {
		return sui.SimpleCheckpoint{}, err
	}
	if len(scs) == 0 {
		return sui.SimpleCheckpoint{}, errors.Errorf("checkpoint %d not found", checkpoint)
	}
	err = s.cachedSimpleCheckpoint.Set(ctx, map[string]sui.SimpleCheckpoint{key: scs[0]})
	if err != nil {
		logger.Warne(err, "update cached simple checkpoint failed")
	}
	return scs[0], nil
}

// GetCheckpointTime return [checkpointTimestampMs, minTxnTimestampMs, maxTxnTimestampMs]
func (s *SuperService) GetCheckpointTime(
	ctx context.Context,
	network string,
	checkpointSequenceNumber types.Number,
) ([]uint64, error) {
	if err := s.requireJSONRPC(); err != nil {
		return nil, err
	}
	ctx, logger := log.FromContext(ctx, "checkpoint", checkpointSequenceNumber)
	key := checkpointSequenceNumber.String()
	cached, getCacheErr := s.cachedCheckpointTime.Get(ctx, key)
	if getCacheErr != nil {
		logger.Errorfe(getCacheErr, "get checkpoint time from cache failed")
		return nil, getCacheErr
	}
	if ct, has := cached[key]; has {
		return []uint64{ct.CheckpointTime, ct.MinTxnTime, ct.MaxTxnTime}, nil
	}
	results, err := chain.QueryRangeWithCache(
		ctx,
		rg.NewSingleRange(checkpointSequenceNumber.Uint64()),
		s.slotCache,
		func(slot *sui.Slot) ([]sui.CheckpointTime, error) {
			ts := slot.TimestampMs.Uint64()
			minMs, maxMs := ts, ts
			for _, tx := range slot.Transactions {
				maxMs = max(maxMs, tx.TimestampMs.Uint64())
				minMs = min(minMs, tx.TimestampMs.Uint64())
			}
			return []sui.CheckpointTime{{CheckpointTime: ts, MinTxnTime: minMs, MaxTxnTime: maxMs}}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]sui.CheckpointTime, error) {
			// cache already checked as missing above, query from clickhouse
			result, err := s.storageJSONRPC.QueryCheckpointTime(ctx, checkpointSequenceNumber.Uint64())
			if err != nil {
				return nil, err
			}
			return []sui.CheckpointTime{result}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.Errorf("checkpoint %d not found", checkpointSequenceNumber.Uint64())
	}
	ct := results[0]
	// update cache regardless of whether the result came from the slot cache or clickhouse;
	// checkpoint time is immutable, so it is always safe to persist.
	if setErr := s.cachedCheckpointTime.Set(ctx, map[string]sui.CheckpointTime{key: ct}); setErr != nil {
		logger.Warne(setErr, "update cached checkpoint time failed")
	}
	return []uint64{ct.CheckpointTime, ct.MinTxnTime, ct.MaxTxnTime}, nil
}

func (s *SuperService) GetTransactions(
	ctx context.Context,
	network string,
	query *sui.TransactionQuery,
) ([]types.TransactionResponseV1, error) {
	if err := s.requireJSONRPC(); err != nil {
		return nil, err
	}
	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber),
		s.slotCache,
		func(slot *sui.Slot) ([]types.TransactionResponseV1, error) {
			var resp []types.TransactionResponseV1
			for i := range slot.Transactions {
				tx := slot.Transactions[i] // tx is a copy, so we can change tx.Events tx.Effects tx.Transaction below
				if !query.CheckAndTrim(&tx) {
					continue
				}
				resp = append(resp, tx)
			}
			return resp, nil
		},
		func(ctx context.Context, queryRange rg.Range) (txs []types.TransactionResponseV1, err error) {
			storeQuery := *query
			storeQuery.FromSequenceNumber = queryRange.Start
			storeQuery.ToSequenceNumber = *queryRange.End
			return s.storageJSONRPC.QueryTransactions(ctx, &storeQuery)
		},
	)
}

func (s *SuperService) GetTransactionsV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
) ([]types.TransactionResponseV1, error) {
	if err := s.requireJSONRPC(); err != nil {
		return nil, err
	}
	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(slot *sui.Slot) ([]types.TransactionResponseV1, error) {
			var resp []types.TransactionResponseV1
			for i := range slot.Transactions {
				tx := slot.Transactions[i] // tx is a copy, so we can change tx.Events tx.Effects tx.Transaction below
				if !filter.Check(tx) {
					continue
				}
				resp = append(resp, fetchConfig.PruneTransaction(tx, filter.EventFilters))
			}
			return resp, nil
		},
		func(ctx context.Context, queryRange rg.Range) (txs []types.TransactionResponseV1, err error) {
			return s.storageJSONRPC.QueryTransactionsV2(ctx, queryRange.Start, *queryRange.End, filter, fetchConfig)
		},
	)
}

// GetGrpcTransactions is a grpc data format interface,
// kind in filter.FunctionFilters should use TransactionKind_Kind values:
//   - PROGRAMMABLE_TRANSACTION
//   - CHANGE_EPOCH
//   - GENESIS
//   - CONSENSUS_COMMIT_PROLOGUE_V1
//   - AUTHENTICATOR_STATE_UPDATE
//   - END_OF_EPOCH
//   - RANDOMNESS_STATE_UPDATE
//   - CONSENSUS_COMMIT_PROLOGUE_V2
//   - CONSENSUS_COMMIT_PROLOGUE_V3
//   - CONSENSUS_COMMIT_PROLOGUE_V4
//   - PROGRAMMABLE_SYSTEM_TRANSACTION
func (s *SuperService) GetGrpcTransactions(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
) ([]*sui.ExtendedGrpcTransaction, error) {
	if err := s.requireGRPC(); err != nil {
		return nil, err
	}
	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(slot *sui.Slot) ([]*sui.ExtendedGrpcTransaction, error) {
			if slot.GrpcCheckpoint == nil {
				return nil, errors.Errorf("checkpoint %d miss grpc data", slot.GetNumber())
			}
			var resp []*sui.ExtendedGrpcTransaction
			for txIndex, tx := range slot.GrpcCheckpoint.GetTransactions() {
				if !filter.CheckGrpcTx(tx) {
					continue
				}
				etx := &sui.ExtendedGrpcTransaction{
					Checkpoint:          slot.SequenceNumber,
					CheckpointDigest:    slot.Digest,
					TimestampMs:         slot.TimestampMs.Uint64(),
					Epoch:               slot.GrpcCheckpoint.GetSummary().GetEpoch(),
					TxIndex:             uint64(txIndex),
					ExecutedTransaction: tx,
				}
				resp = append(resp, fetchConfig.PruneGrpcTransaction(etx, filter.EventFilters))
			}
			return resp, nil
		},
		func(ctx context.Context, queryRange rg.Range) (txs []*sui.ExtendedGrpcTransaction, err error) {
			return s.storageGRPC.QueryTransactions(ctx, queryRange.Start, *queryRange.End, filter, fetchConfig)
		},
	)
}

func (s *SuperService) FilterObjectChanges(
	ctx context.Context,
	query *sui.ObjectChangeQuery,
) ([]types.ObjectChangeExtend, error) {
	if err := s.requireJSONRPC(); err != nil {
		return nil, err
	}
	result, err := chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber),
		s.slotCache,
		func(slot *sui.Slot) ([]types.ObjectChangeExtend, error) {
			var result []types.ObjectChangeExtend
			for txIndex, tx := range slot.Transactions {
				for _, oc := range tx.ObjectChanges {
					result = append(result, types.ObjectChangeExtend{
						Checkpoint:       types.Uint64ToNumber(slot.SequenceNumber),
						CheckpointDigest: types.StrToDigestMust(slot.Digest),
						TxIndex:          txIndex,
						TxDigest:         tx.Digest,
						ObjectChange:     oc,
					})
				}
			}
			return query.Filter(result), nil
		},
		func(ctx context.Context, queryRange rg.Range) (objs []types.ObjectChangeExtend, err error) {
			storeQuery := *query
			storeQuery.FromSequenceNumber = queryRange.Start
			storeQuery.ToSequenceNumber = *queryRange.End
			return s.storageJSONRPC.QueryObjectChanges(ctx, &storeQuery)
		},
	)
	if err != nil {
		return nil, err
	}
	if query.OnlyLastVersion {
		set := make(map[string]types.ObjectChangeExtend)
		for _, oc := range result {
			set[oc.GetObjectID()] = oc
		}
		result = utils.GetMapValues(set)
	}
	return result, nil
}

func (s *SuperService) FilterObjectChangesV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
) ([]types.ObjectChangeExtend, error) {
	if err := s.requireJSONRPC(); err != nil {
		return nil, err
	}
	checker := filter.Checker()
	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(slot *sui.Slot) (result []types.ObjectChangeExtend, err error) {
			checkpoint := types.Uint64ToNumber(slot.SequenceNumber)
			checkpointDigest := types.StrToDigestMust(slot.Digest)
			for txIndex, tx := range slot.Transactions {
				for _, oc := range tx.ObjectChanges {
					oce := types.ObjectChangeExtend{
						Checkpoint:       checkpoint,
						CheckpointDigest: checkpointDigest,
						TxIndex:          txIndex,
						TxDigest:         tx.Digest,
						ObjectChange:     oc,
					}
					if checker(oce) {
						result = append(result, oce)
					}
				}
			}
			return result, nil
		},
		func(ctx context.Context, queryRange rg.Range) (objs []types.ObjectChangeExtend, err error) {
			return s.storageJSONRPC.QueryObjectChangesV2(ctx, queryRange.Start, *queryRange.End, filter)
		},
	)
}

// FilterGrpcChangedObjects is a grpc data format interface,
// ownerType in filter should use Owner_OwnerKind values:
//   - ADDRESS
//   - OBJECT
//   - SHARED
//   - IMMUTABLE
//   - CONSENSUS_ADDRESS
func (s *SuperService) FilterGrpcChangedObjects(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
) ([]*sui.ExtendedGrpcChangedObject, error) {
	if err := s.requireGRPC(); err != nil {
		return nil, err
	}
	checker := filter.CheckerGrpc()
	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(slot *sui.Slot) (result []*sui.ExtendedGrpcChangedObject, err error) {
			if slot.GrpcCheckpoint == nil {
				return nil, errors.Errorf("checkpoint %d miss grpc data", slot.GetNumber())
			}
			for i, tx := range slot.GrpcCheckpoint.GetTransactions() {
				for _, co := range tx.GetEffects().GetChangedObjects() {
					if !checker(co) {
						continue
					}
					result = append(result, &sui.ExtendedGrpcChangedObject{
						Checkpoint:       slot.SequenceNumber,
						CheckpointDigest: slot.Digest,
						TimestampMs:      slot.TimestampMs.Uint64(),
						Epoch:            slot.GrpcCheckpoint.GetSummary().GetEpoch(),
						TxIndex:          uint64(i),
						TxDigest:         tx.GetDigest(),
						ChangedObject:    co,
					})
				}
			}
			return result, nil
		},
		func(ctx context.Context, queryRange rg.Range) (objs []*sui.ExtendedGrpcChangedObject, err error) {
			return s.storageGRPC.QueryObjectChanges(ctx, queryRange.Start, *queryRange.End, filter)
		},
	)
}

func (s *SuperService) GetGrpcObjects(
	ctx context.Context,
	reqs []*rpcv2.GetObjectRequest,
	concurrency int,
	batchSize int,
) ([]*rpcv2.GetObjectResult, error) {
	if err := s.requireGRPC(); err != nil {
		return nil, err
	}
	const theme = "proxy.GetGrpcObjects.grpc_BatchGetObjects"
	concurrency = min(concurrency, 10)
	batchSize = min(batchSize, 50)
	return s.client.GetGrpcObjectsByPage(ctx, theme, theme, concurrency, batchSize, reqs)
}

func (s *SuperService) GetObjectCreation(ctx context.Context, objectID string) (*sui.ObjectCreation, error) {
	_, logger := log.FromContext(ctx, "objectID", objectID)
	cached, getCacheErr := s.cachedObjectCreation.Get(ctx, objectID)
	if getCacheErr != nil {
		logger.Errorfe(getCacheErr, "get object creation from cache failed")
		return nil, getCacheErr
	}
	if oc, has := cached[objectID]; has {
		return &oc, nil
	}
	stat, err := s.GetObjectStat(ctx, 0, math.MaxUint64, objectID)
	if err != nil {
		return nil, err
	}
	if stat.Count == 0 {
		return nil, nil
	}
	creation := sui.ObjectCreation{
		ObjectVersion: stat.MinObjectVersion,
		Checkpoint:    stat.MinCheckpoint,
	}
	if err = s.cachedObjectCreation.Set(ctx, map[string]sui.ObjectCreation{objectID: creation}); err != nil {
		logger.Warne(err, "update cached object creation failed")
	}
	return &creation, nil
}

func (s *SuperService) GetObjectStat(
	ctx context.Context,
	startCheckpoint uint64,
	endCheckpoint uint64,
	objectID string,
) (sui.ObjectStat, error) {
	stats, err := s.GetObjectsStat(ctx, startCheckpoint, endCheckpoint, []string{objectID})
	if err != nil {
		return sui.ObjectStat{}, err
	}
	// stats[objectID] is the zero ObjectStat (Count 0) when the object is not found
	return stats[objectID], nil
}

func (s *SuperService) GetObjectsStat(
	ctx context.Context,
	startCheckpoint uint64,
	endCheckpoint uint64,
	objectIDList []string,
) (map[string]sui.ObjectStat, error) {
	const maxObjectIDLen = 200
	if len(objectIDList) > maxObjectIDLen {
		return nil, errors.Errorf("too many object ids, should <= %d", maxObjectIDLen)
	}
	objectIDSet := set.New[string](objectIDList...)
	ss, err := chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(startCheckpoint, endCheckpoint),
		s.slotCache,
		func(st *sui.Slot) ([]map[string]sui.ObjectStat, error) {
			result := make(map[string]sui.ObjectStat)
			merge := func(objID string, version uint64) {
				result[objID] = result[objID].Merge(sui.ObjectStat{
					Count:            1,
					MinObjectVersion: version,
					MinCheckpoint:    st.GetNumber(),
					MaxObjectVersion: version,
					MaxCheckpoint:    st.GetNumber(),
				})
			}
			// prefer grpc data; fall back to json-rpc when the slot has no grpc data (e.g. iota)
			if st.GrpcCheckpoint != nil {
				st.IterGrpcObjectChanges(func(objID string, version uint64) {
					if objectIDSet.Contains(objID) {
						merge(objID, version)
					}
				})
			} else {
				for _, tx := range st.Transactions {
					for _, oc := range tx.ObjectChanges {
						if objID := oc.GetObjectID(); objectIDSet.Contains(objID) {
							merge(objID, oc.Version.Uint64())
						}
					}
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return []map[string]sui.ObjectStat{result}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]map[string]sui.ObjectStat, error) {
			r, err := s.storageShared.QueryObjectsStat(ctx, queryRange.Start, *queryRange.End, objectIDList)
			if err != nil {
				return nil, err
			}
			if len(r) == 0 {
				return nil, nil
			}
			return []map[string]sui.ObjectStat{r}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return utils.Reduce(ss, func(a, b map[string]sui.ObjectStat) map[string]sui.ObjectStat {
		r := make(map[string]sui.ObjectStat)
		for k, v := range a {
			r[k] = v
		}
		for k, v := range b {
			r[k] = r[k].Merge(v)
		}
		return r
	}), nil
}
