package sui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/compress"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type ObjectDictSetManager struct {
	mu                sync.RWMutex
	data              map[string]ObjectDict
	cachedData        string
	cachedBlockNumber *uint64
}

func (m *ObjectDictSetManager) Get(key string) *ObjectDict {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if od, has := m.data[key]; has {
		return &od
	}
	return nil
}

func (m *ObjectDictSetManager) Put(key string, value ObjectDict) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

const CheckpointDataKey = "ObjectDictSetManager"

func (m *ObjectDictSetManager) Load(checkpoint *controller.Checkpoint) error {
	var raw string
	if checkpoint != nil {
		raw = checkpoint.Data[CheckpointDataKey]
	}
	return m.load(raw)
}

// GetData use c.cachedData so the unsaved checkpoint will not dup the same data, can save memory
func (m *ObjectDictSetManager) GetData() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.data) == 0 {
		return ""
	}
	var bn uint64
	for _, dict := range m.data {
		bn = max(bn, dict.BlockNumber)
	}
	if m.cachedBlockNumber == nil || *m.cachedBlockNumber < bn {
		m.cachedBlockNumber, m.cachedData = &bn, m.dump()
	}
	return m.cachedData
}

func (m *ObjectDictSetManager) dump() string {
	var buf bytes.Buffer
	for key, dict := range m.data {
		if buf.Len() > 0 {
			buf.WriteRune('\n')
		}
		buf.WriteString(key)
		buf.WriteRune('@')
		buf.WriteString(dict.Dump())
	}
	b, _ := compress.Dump(buf.String())
	return string(b)
}

func (m *ObjectDictSetManager) load(data string) error {
	var raw string
	if err := compress.Load([]byte(data), &raw); err != nil {
		return errors.Wrapf(err, "decompress data failed")
	}
	result := make(map[string]ObjectDict)
	var line string
	var blockNumber uint64
	for len(raw) > 0 {
		line, raw, _ = strings.Cut(raw, "\n")
		key, value, _ := strings.Cut(line, "@")
		var dict ObjectDict
		if err := dict.Load(value); err != nil {
			return errors.Wrapf(err, "load object dictionary for %s failed", key)
		}
		blockNumber = max(blockNumber, dict.BlockNumber)
		result[key] = dict
	}
	m.mu.Lock()
	m.data = result
	if len(result) > 0 {
		m.cachedBlockNumber, m.cachedData = &blockNumber, data
	} else {
		m.cachedBlockNumber, m.cachedData = nil, ""
	}
	m.mu.Unlock()
	return nil
}

var objectDictPreviewCount = envconf.LoadUInt64("SENTIO_SUI_INTERVAL_HANDLER_OBJECT_DICT_PREVIEW_COUNT", 100)

func (m *ObjectDictSetManager) Snapshot() any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dataPreview := make(map[string]any)
	for key, dict := range m.data {
		dataPreview[key] = dict.Snapshot(int(objectDictPreviewCount))
	}
	return dataPreview
}

type ObjectDict struct {
	BlockNumber         uint64
	ObjectLatestVersion map[string]uint64
}

func (d *ObjectDict) Snapshot(previewCount int) any {
	preview := make(map[string]uint64)
	for objectID, latestVersion := range d.ObjectLatestVersion {
		preview[objectID] = latestVersion
		if len(preview) >= previewCount {
			break
		}
	}
	return map[string]any{
		"blockNumber": d.BlockNumber,
		"size":        len(d.ObjectLatestVersion),
		"preview":     preview,
	}
}

func (d *ObjectDict) Dump() string {
	var buf bytes.Buffer
	buf.WriteString(strconv.FormatUint(d.BlockNumber, 16))
	for _, objectID := range utils.GetOrderedMapKeys(d.ObjectLatestVersion) {
		buf.WriteRune('#')
		buf.WriteString(objectID)
		buf.WriteRune(':')
		buf.WriteString(strconv.FormatUint(d.ObjectLatestVersion[objectID], 16))
	}
	return buf.String()
}

func (d *ObjectDict) Load(raw string) (err error) {
	var part string
	part, raw, _ = strings.Cut(raw, "#")
	if d.BlockNumber, err = strconv.ParseUint(part, 16, 64); err != nil {
		return errors.Wrapf(err, "parse block number from %q failed", part)
	}
	d.ObjectLatestVersion = make(map[string]uint64)
	for len(raw) > 0 {
		part, raw, _ = strings.Cut(raw, "#")
		objectID, ver, _ := strings.Cut(part, ":")
		var version uint64
		if version, err = strconv.ParseUint(ver, 16, 64); err != nil {
			return errors.Wrapf(err, "parse version for objectID %s from %q failed", objectID, ver)
		}
		d.ObjectLatestVersion[objectID] = version
	}
	return nil
}

type HandlerAgentInterval struct {
	controller.BaseHandlerAgent

	Client sui.Client `json:"-"` // used to check address is a ERC20 address

	IntervalConfig      data.IntervalConfig
	Filter              chainsui.ObjectChangeFilter
	NeedSelf            bool
	UnwrapDynamicObject bool
}

const (
	MaxObjectDictLen              = 100000
	MaxQueryObjectChangeRangeSize = 10000
)

func (a HandlerAgentInterval) PushObjectLatestVersion(
	ctx context.Context,
	blockNumber uint64,
	dict *ObjectDict,
) (ObjectDict, error) {
	_, logger := log.FromContext(ctx, "handler", a.HandlerID.String(), "filter", utils.MustJSONMarshal(a.Filter))
	if dict != nil && dict.BlockNumber == blockNumber { // may be retried, so dict.BlockNumber may be equal to blockNumber
		return *dict, nil
	}
	var from uint64
	result := ObjectDict{
		BlockNumber:         blockNumber,
		ObjectLatestVersion: make(map[string]uint64),
	}
	if dict != nil {
		from = dict.BlockNumber + 1
		result.ObjectLatestVersion = utils.CopyMap(dict.ObjectLatestVersion)
	}
	logger.Infof("will push object latest version dict in [%d,%d]", from, blockNumber)
	for from <= blockNumber {
		startAt := time.Now()
		end := min(blockNumber, from+MaxQueryObjectChangeRangeSize-1)
		changes, err := a.Client.GetObjectChanges(ctx, from, end, a.Filter)
		if err != nil {
			return ObjectDict{}, err
		}
		beforeSize := len(result.ObjectLatestVersion)
		var deleteCount int
		var updateCount int
		var createCount int
		for _, bn := range utils.GetOrderedMapKeys(changes) {
			for _, oc := range changes[bn] {
				objectID, objectVersion := oc.ObjectID.String(), oc.Version.Uint64()
				if oc.Type.IsDeleted() {
					logger.Debugf("pushing and deleted object %s/%d", objectID, objectVersion)
					delete(result.ObjectLatestVersion, objectID)
					deleteCount++
				} else if preVersion, has := result.ObjectLatestVersion[objectID]; has {
					logger.Debugf("pushing and updated object %s/%d=>%d", objectID, preVersion, objectVersion)
					result.ObjectLatestVersion[objectID] = objectVersion
					updateCount++
				} else {
					logger.Debugf("pushing and created object %s/%d", objectID, objectVersion)
					result.ObjectLatestVersion[objectID] = objectVersion
					createCount++
				}
			}
		}
		logger.With("used", time.Since(startAt).String()).
			Infof("pushed object latest version dict in [%d,%d], size %d => %d, created %d and updated %d and deleted %d",
				from, end, beforeSize, len(result.ObjectLatestVersion), createCount, updateCount, deleteCount)
		from = end + 1
	}
	if size := len(result.ObjectLatestVersion); size > MaxObjectDictLen {
		err := errors.Errorf("object latest version dict size for handler %s with filter %s is too big: %d > %d",
			a.HandlerID.String(), utils.MustJSONMarshal(a.Filter), size, MaxObjectDictLen)
		return ObjectDict{}, fetcher.Permanent(err)
	}
	return result, nil
}

type objectDetails struct {
	Version string          `json:"version"`
	Digest  string          `json:"digest"`
	Content json.RawMessage `json:"content"`
	Type    string          `json:"type"`
}

type object struct {
	Version uint64
	Digest  string
	Content json.RawMessage
}

var ignoreNotExistObject = envconf.LoadBool("SENTIO_SUI_INTERVAL_HANDLER_IGNORE_NON_EXISTENT_OBJECTS", true)

var dynamicType = types.TypeTagFromStringMust(
	"0x2::dynamic_field::Field<0x2::dynamic_object_field::Wrapper<any>, 0x2::object::ID>")

func (a HandlerAgentInterval) ObjMgrKey() string {
	return utils.MustJSONMarshal(a.Filter)
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
	var opts = types.SuiObjectDataOptions{
		ShowContent: true,
		ShowType:    true,
	}
	var requests []types.SuiGetPastObjectRequest
	for objectID, version := range dict.ObjectLatestVersion {
		requests = append(requests, types.SuiGetPastObjectRequest{
			ObjectID: types.StrToObjectIDMust(objectID),
			Version:  types.Uint64ToNumber(version),
		})
	}
	contents := make(map[string]object)
	for len(requests) > 0 {
		var resp []types.SuiPastObjectResponse
		if resp, err = a.Client.TryMultiGetPastObjects(ctx, requests, opts); err != nil {
			return
		}
		var wrappedObjectIDList []string
		for i, obj := range resp {
			req := requests[i]
			switch obj.Status {
			case types.SuiPastObjectStatusVersionFound:
				var details objectDetails
				if err = json.Unmarshal(obj.Details, &details); err != nil {
					return nil, errors.Wrapf(err, "object %s version %d unmarshal details failed",
						req.ObjectID, req.Version.Uint64())
				}
				objectType, _ := types.TypeTagFromString(details.Type)
				if a.UnwrapDynamicObject && objectType != nil && dynamicType.Include(*objectType) {
					var moveObj struct {
						Fields struct {
							Value string `json:"value"`
						} `json:"fields"`
					}
					if err = json.Unmarshal(details.Content, &moveObj); err != nil {
						return nil, errors.Wrapf(err, "object %s version %d unmarshal move object failed",
							req.ObjectID, req.Version.Uint64())
					}
					wrappedObjectIDList = append(wrappedObjectIDList, moveObj.Fields.Value)
				} else {
					contents[req.ObjectID.String()] = object{
						Version: req.Version.Uint64(),
						Digest:  details.Digest,
						Content: details.Content,
					}
				}
			case types.SuiPastObjectStatusObjectDeleted:
			default:
				message := fmt.Sprintf("object %s version %d has unexpected status %s",
					req.ObjectID, req.Version.Uint64(), obj.Status)
				if !ignoreNotExistObject {
					return nil, errors.Errorf(message)
				}
				logger.Warnf("%s, will be ignored", message)
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
					requests = append(requests, types.SuiGetPastObjectRequest{
						ObjectID: types.StrToObjectIDMust(objectID),
						Version:  types.Uint64ToNumber(stat[i].MaxObjectVersion),
					})
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
							RawSelf:       utils.WrapPointer(string(obj.Content)),
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
		var self *object
		for objectID, obj := range contents {
			if utils.IndexOf(a.Filter.OwnerFilter.OwnerID, objectID) < 0 {
				owned = append(owned, string(obj.Content))
				dataSize += len(obj.Content)
			} else {
				self = utils.WrapPointer(contents[objectID])
			}
		}
		var selfContent *string
		var selfVersion uint64
		var selfDigest string
		if self != nil {
			selfContent = utils.WrapPointer(string(self.Content))
			selfVersion = self.Version
			selfDigest = self.Digest
			dataSize += len(self.Content)
		} else if a.NeedSelf {
			// need self but self not exist
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

func (a HandlerAgentInterval) Snapshot() any {
	return map[string]any{
		"HandlerID":           a.HandlerID,
		"Range":               a.Range.String(),
		"IntervalConfig":      a.IntervalConfig,
		"Filter":              a.Filter,
		"NeedSelf":            a.NeedSelf,
		"UnwrapDynamicObject": a.UnwrapDynamicObject,
	}
}
