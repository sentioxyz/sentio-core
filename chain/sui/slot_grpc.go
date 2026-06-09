package sui

import (
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/utils"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
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

func FromChangeType(changeType types.ObjectChangeType) (
	rpcv2.ChangedObject_InputObjectState,
	rpcv2.ChangedObject_OutputObjectState,
	rpcv2.ChangedObject_IdOperation,
) {
	switch changeType {
	case types.ObjectChangeTypeAccumulatorWrite:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_ACCUMULATOR_WRITE,
			rpcv2.ChangedObject_NONE
	case types.ObjectChangeTypePublished:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_PACKAGE_WRITE,
			rpcv2.ChangedObject_CREATED
	case types.ObjectChangeTypeCreated:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE,
			rpcv2.ChangedObject_CREATED
	case types.ObjectChangeTypeUnwrapped:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE,
			rpcv2.ChangedObject_NONE
	case types.ObjectChangeTypeMutated:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE,
			rpcv2.ChangedObject_NONE
	case types.ObjectChangeTypeDeleted:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_DELETED
	case types.ObjectChangeTypeWrapped:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_NONE
	case types.ObjectChangeTypeUnwrappedThenDeleted:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_DOES_NOT_EXIST,
			rpcv2.ChangedObject_DELETED
	default:
		return rpcv2.ChangedObject_INPUT_OBJECT_STATE_UNKNOWN,
			rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_UNKNOWN,
			rpcv2.ChangedObject_NONE
	}
}

type ObjectSet map[string]map[uint64]*rpcv2.Object

func (s ObjectSet) Get(id string, version uint64) (*rpcv2.Object, bool) {
	return utils.GetFromK2Map(s, id, version)
}

func (s ObjectSet) Put(objs ...*rpcv2.Object) {
	for _, obj := range objs {
		utils.PutIntoK2Map(s, obj.GetObjectId(), obj.GetVersion(), obj)
	}
}
