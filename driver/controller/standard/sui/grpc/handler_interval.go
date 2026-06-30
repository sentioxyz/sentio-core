package grpc

import (
	"context"
	"fmt"
	"math"
	"time"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/log"
	cprotojson "sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HandlerAgentInterval struct {
	suihandler.HandlerAgentInterval
}

const (
	grpcObjectsConcurrency = 10
	grpcObjectsBatchSize   = 50
)

// objectReadMask asks the super node for the fields the binding needs (the default mask is only
// object_id,version,digest). Unlike the json-rpc handler — which binds just the parsed content
// (SuiParsedData) — the grpc handler binds the whole rpcv2.Object (see RawSelf below), so the mask
// must include every field that object carries: object_type and has_public_transfer (NOT present in
// the rendered json), owner, and json itself. object_type additionally drives the dynamic-object check.
// It is passed as the batch-level read mask to GetGrpcObjects (the only mask the upstream node honors).
var objectReadMask = &fieldmaskpb.FieldMask{
	Paths: []string{"object_id", "version", "digest", "object_type", "has_public_transfer", "owner", "json"},
}

var ignoreNotExistObject = envconf.LoadBool("SENTIO_SUI_INTERVAL_HANDLER_IGNORE_NON_EXISTENT_OBJECTS", true)

var dynamicType = types.TypeTagFromStringMust(
	"0x2::dynamic_field::Field<0x2::dynamic_object_field::Wrapper<any>, 0x2::object::ID>")

type grpcObject struct {
	Version uint64
	Digest  string
	Content string // protojson of rpcv2.Object — the grpc-format object json
}

// PushObjectLatestVersion overrides the embedded json-rpc implementation to track the latest object
// versions from grpc object changes (sui_filterGrpcChangedObjects) instead of the json-rpc
// sui_filterObjectChangesV2, keeping the interval handler on the grpc data source end to end (it later
// fetches those versions via GetGrpcObjects). The version-tracking logic is otherwise identical.
func (a HandlerAgentInterval) PushObjectLatestVersion(
	ctx context.Context,
	blockNumber uint64,
	dict *suihandler.ObjectDict,
) (suihandler.ObjectDict, error) {
	_, logger := log.FromContext(ctx, "handler", a.HandlerID.String(), "filter", utils.MustJSONMarshal(a.Filter))
	if dict != nil && dict.BlockNumber == blockNumber { // may be retried, so dict.BlockNumber may be equal to blockNumber
		return *dict, nil
	}
	var from uint64
	result := suihandler.ObjectDict{
		BlockNumber:         blockNumber,
		ObjectLatestVersion: make(map[string]uint64),
	}
	if dict != nil {
		from = dict.BlockNumber + 1
		result.ObjectLatestVersion = utils.CopyMap(dict.ObjectLatestVersion)
	}
	logger.Infof("will push grpc object latest version dict in [%d,%d]", from, blockNumber)
	for from <= blockNumber {
		startAt := time.Now()
		end := min(blockNumber, from+suihandler.MaxQueryObjectChangeRangeSize-1)
		changes, err := a.Client.GetGrpcObjectChanges(ctx, from, end, a.Filter)
		if err != nil {
			return suihandler.ObjectDict{}, err
		}
		beforeSize := len(result.ObjectLatestVersion)
		var deleteCount, updateCount, createCount int
		for _, bn := range utils.GetOrderedMapKeys(changes) {
			for _, oc := range changes[bn] {
				objectID, objectVersion := oc.GetObjectId(), oc.GetOutputVersion()
				if chainsui.GetChangeType(oc.ChangedObject).IsDeleted() {
					delete(result.ObjectLatestVersion, objectID)
					deleteCount++
				} else if _, has := result.ObjectLatestVersion[objectID]; has {
					result.ObjectLatestVersion[objectID] = objectVersion
					updateCount++
				} else {
					result.ObjectLatestVersion[objectID] = objectVersion
					createCount++
				}
			}
		}
		logger.With("used", time.Since(startAt).String()).
			Infof("pushed grpc object latest version dict in [%d,%d], size %d => %d, created %d and updated %d and deleted %d",
				from, end, beforeSize, len(result.ObjectLatestVersion), createCount, updateCount, deleteCount)
		from = end + 1
	}
	if size := len(result.ObjectLatestVersion); size > suihandler.MaxObjectDictLen {
		err := errors.Errorf("object latest version dict size for handler %s with filter %s is too big: %d > %d",
			a.HandlerID.String(), utils.MustJSONMarshal(a.Filter), size, suihandler.MaxObjectDictLen)
		return suihandler.ObjectDict{}, fetcher.Permanent(err)
	}
	return result, nil
}

func (a HandlerAgentInterval) BuildBindingDataList(
	ctx context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	if !data.ContainsInterval(bd.mainData.Intervals, a.IntervalConfig) {
		return
	}
	dict := bd.objMgr.Get(a.ObjMgrKey())
	if dict == nil {
		return
	}

	_, logger := log.FromContext(ctx)
	var requests []*rpcv2.GetObjectRequest
	for objectID, version := range dict.ObjectLatestVersion {
		requests = append(requests, newObjectRequest(objectID, version))
	}
	contents := make(map[string]grpcObject)
	for len(requests) > 0 {
		var resp []*rpcv2.GetObjectResult
		if resp, err = a.Client.GetGrpcObjects(ctx, requests, objectReadMask, grpcObjectsConcurrency, grpcObjectsBatchSize); err != nil {
			return
		}
		var wrappedObjectIDList []string
		for i, res := range resp {
			req := requests[i]
			obj := res.GetObject()
			if obj == nil {
				// deleted / not found — GetObjectResult carries an error instead of an object
				message := fmt.Sprintf("object %s version %d not returned: %s",
					req.GetObjectId(), req.GetVersion(), res.GetError().GetMessage())
				if !ignoreNotExistObject {
					return nil, errors.Errorf(message)
				}
				logger.Warnf("%s, will be ignored", message)
				continue
			}
			objectType, _ := types.TypeTagFromString(obj.GetObjectType())
			if a.UnwrapDynamicObject && objectType != nil && dynamicType.Include(*objectType) {
				wrappedObjectIDList = append(wrappedObjectIDList, wrappedObjectID(obj.GetJson()))
			} else {
				var content []byte
				// Bind the WHOLE rpcv2.Object, intentionally diverging from the json-rpc handler (which
				// binds only the parsed content, SuiParsedData). The grpc object's rendered value
				// (obj.GetJson()) is leaner than SuiParsedData: it flattens the move struct's fields to
				// the top level and drops object_type / has_public_transfer (those live as sibling
				// fields on rpcv2.Object, not inside json). Marshaling the whole object preserves that
				// info for the processor. Example — sui-mainnet object
				//   0xc061d544681939544136efac81d212de377e2ff13eb07ef9079404ebd57cad5d version 309855314
				// grpc json (fields flattened, no object_type / has_public_transfer):
				//   {
				//     "id": "0xc061d544681939544136efac81d212de377e2ff13eb07ef9079404ebd57cad5d",
				//     "name": {"name": "0xe859a7ebc84e7573d1e81ef99946f8821aeb0ff67454e579a32dd216da239621"},
				//     "value": "0xc0254d60d00d9215c3a878ad2ea020aeebfb336eb143e5919a8209e71db998a0"
				//   }
				// whereas json-rpc content wraps the fields and adds type info:
				//   {
				//     "dataType": "moveObject",
				//     "type": "...",
				//     "hasPublicTransfer": false,
				//     "fields": {
				//       "id": {"id": "0xc061d544681939544136efac81d212de377e2ff13eb07ef9079404ebd57cad5d"},
				//       "name": {...},
				//       "value": "0xc0254d60d00d9215c3a878ad2ea020aeebfb336eb143e5919a8209e71db998a0"
				//     }
				//   }
				if content, err = cprotojson.Marshal(obj); err != nil {
					return nil, errors.Wrapf(err, "marshal grpc object %s failed", obj.GetObjectId())
				}
				contents[obj.GetObjectId()] = grpcObject{
					Version: obj.GetVersion(),
					Digest:  obj.GetDigest(),
					Content: string(content),
				}
			}
		}
		requests = requests[:0]
		if len(wrappedObjectIDList) > 0 {
			var stat []chainsui.ObjectStat
			if stat, err = a.Client.GetObjectsStat(ctx, 0, bd.GetBlockNumber(), wrappedObjectIDList); err != nil {
				return nil, err
			}
			for i, objectID := range wrappedObjectIDList {
				if stat[i].Count > 0 {
					requests = append(requests, newObjectRequest(objectID, stat[i].MaxObjectVersion))
				} else {
					logger.Warnf("Object %s has no history", objectID)
				}
			}
		}
	}

	if a.Filter.TypePattern != nil {
		for objectID, obj := range contents {
			result = append(result, standard.BindingDataInner{
				HandlerType: protos.HandlerType_SUI_OBJECT,
				Data: &protos.Data{
					Value: &protos.Data_SuiObject_{
						SuiObject: &protos.Data_SuiObject{
							RawSelf:       utils.WrapPointer(obj.Content),
							ObjectId:      objectID,
							ObjectVersion: obj.Version,
							ObjectDigest:  obj.Digest,
							Timestamp:     timestamppb.New(bd.GetBlockTime()),
							Slot:          bd.GetBlockNumber(),
						},
					},
				},
				DataSize: len(obj.Content),
			})
		}
	}
	if a.Filter.OwnerFilter != nil {
		var dataSize int
		var owned []string
		var self *grpcObject
		for objectID, obj := range contents {
			if utils.IndexOf(a.Filter.OwnerFilter.OwnerID, objectID) < 0 {
				owned = append(owned, obj.Content)
				dataSize += len(obj.Content)
			} else {
				self = utils.WrapPointer(contents[objectID])
			}
		}
		var selfContent *string
		var selfVersion uint64
		var selfDigest string
		if self != nil {
			selfContent = utils.WrapPointer(self.Content)
			selfVersion = self.Version
			selfDigest = self.Digest
			dataSize += len(self.Content)
		} else if a.NeedSelf {
			return
		}
		result = append(result, standard.BindingDataInner{
			HandlerType: protos.HandlerType_SUI_OBJECT,
			TxIndex:     math.MaxInt,
			Data: &protos.Data{
				Value: &protos.Data_SuiObject_{
					SuiObject: &protos.Data_SuiObject{
						RawSelf:       selfContent,
						ObjectId:      a.Filter.OwnerFilter.OwnerID[0],
						ObjectVersion: selfVersion,
						ObjectDigest:  selfDigest,
						RawObjects:    owned,
						Timestamp:     timestamppb.New(bd.GetBlockTime()),
						Slot:          bd.GetBlockNumber(),
					},
				},
			},
			DataSize: dataSize,
		})
	}
	return
}

// newObjectRequest builds a by-id+version object request. The read mask is set once at
// the batch level by GetGrpcObjects (objectReadMask), since the upstream node ignores
// per-request masks, so it is intentionally omitted here.
func newObjectRequest(objectID string, version uint64) *rpcv2.GetObjectRequest {
	return &rpcv2.GetObjectRequest{
		ObjectId: utils.WrapPointer(objectID),
		Version:  utils.WrapPointer(version),
	}
}

// wrappedObjectID extracts the wrapped object id from a dynamic-object-field object's grpc json. The
// json-rpc handler reads content.fields.value, but the grpc rendered json flattens the move struct's
// fields to the top level, so "value" sits directly under the root. Verified against sui-mainnet object
// 0xc061d544681939544136efac81d212de377e2ff13eb07ef9079404ebd57cad5d version 309855314 (a
// 0x2::dynamic_field::Field<Wrapper<address>, object::ID>), whose grpc json is
//
//	{
//	  "id": "0xc061d544681939544136efac81d212de377e2ff13eb07ef9079404ebd57cad5d",
//	  "name": {"name": "0xe859a7ebc84e7573d1e81ef99946f8821aeb0ff67454e579a32dd216da239621"},
//	  "value": "0xc0254d60d00d9215c3a878ad2ea020aeebfb336eb143e5919a8209e71db998a0"
//	}
//
// → value = 0xc0254d60d00d9215c3a878ad2ea020aeebfb336eb143e5919a8209e71db998a0 (the wrapped object id).
func wrappedObjectID(v *structpb.Value) string {
	return v.GetStructValue().GetFields()["value"].GetStringValue()
}
