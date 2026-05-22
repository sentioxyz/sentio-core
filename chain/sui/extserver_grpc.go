package sui

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/concurrency"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type ExtServerGrpcDimension struct {
	client *ClientPool

	asSyncerSource         bool
	loadObjectsBatchSize   int
	loadObjectsConcurrency int

	*chain.ExtServerDimension[*Slot]
}

func NewExtServerGrpcDimension(
	client *ClientPool,
	loadConcurrency uint,
	loadRetry int,
	asSyncerSource bool,
	loadObjectsBatchSize int,
	loadObjectsConcurrency int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerGrpcDimension {
	dim := &ExtServerGrpcDimension{
		client:                 client,
		asSyncerSource:         asSyncerSource,
		loadObjectsBatchSize:   loadObjectsBatchSize,
		loadObjectsConcurrency: loadObjectsConcurrency,
	}
	// loadBatchSize more than 1 is meaningless
	dim.ExtServerDimension = chain.NewExtServerDimension[*Slot](
		client,
		loadConcurrency,
		1,
		loadRetry,
		validRange,
		fallBehind,
		dim)
	return dim
}

func (d *ExtServerGrpcDimension) getCheckpoint(ctx context.Context, sn uint64) (*rpcv2.Checkpoint, error) {
	var resp *rpcv2.GetCheckpointResponse
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.MainPart.grpc_GetCheckpoint/%d", sn),
		func(ctx context.Context, cli *Client) clientpool.Result {
			return cli.UseGRPCConnection(ctx, "ext.GetSlot.MainPart.grpc_GetCheckpoint",
				func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
					req := &rpcv2.GetCheckpointRequest{
						CheckpointId: &rpcv2.GetCheckpointRequest_SequenceNumber{SequenceNumber: sn},
						ReadMask:     &fieldmaskpb.FieldMask{Paths: []string{"*"}},
					}
					var err error
					resp, err = rpcv2.NewLedgerServiceClient(conn).GetCheckpoint(ctx, req)
					return clientpool.Result{
						Err:           err,
						BrokenForTask: err != nil, // always retry using other client
					}
				},
			)
		},
		clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
	)
	if r.Err != nil {
		return nil, r.Err
	}
	return resp.GetCheckpoint(), nil
}

func (d *ExtServerGrpcDimension) GetSlot(ctx context.Context, sn uint64) (*Slot, error) {
	ck, err := d.getCheckpoint(ctx, sn)
	if err != nil {
		return nil, err
	}
	s := &Slot{GrpcCheckpoint: ck}
	s.loadCheckpointInfo()
	if d.asSyncerSource {
		// although ck.GetObjects().GetObjects() have all related objects, but ck.GetObjects().GetObjects()[*].GetJson()
		// will be empty, and it is needed for syncer, so have to use grpc_getObjects here to reset all related objects.
		var objReqs []*rpcv2.GetObjectRequest
		for _, obj := range ck.GetObjects().GetObjects() {
			objReqs = append(objReqs, &rpcv2.GetObjectRequest{
				ObjectId: obj.ObjectId,
				Version:  obj.Version,
			})
		}
		var objects []*rpcv2.Object
		objects, err = concurrency.TraverseByPage(
			ctx,
			d.loadObjectsConcurrency,
			d.loadObjectsBatchSize,
			objReqs,
			func(ctx context.Context, page concurrency.Page, reqs []*rpcv2.GetObjectRequest) ([]*rpcv2.Object, error) {
				var resp *rpcv2.BatchGetObjectsResponse
				r := d.client.UseClient(
					ctx,
					fmt.Sprintf("ext.GetSlot.ObjectsPart.grpc_BatchGetObjects/%d/%d#%d-%d", sn, page.Num, page.Start, page.End),
					func(ctx context.Context, cli *Client) clientpool.Result {
						return cli.UseGRPCConnection(ctx, "ext.GetSlot.ObjectsPart.grpc_BatchGetObjects",
							func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
								req := &rpcv2.BatchGetObjectsRequest{
									Requests: reqs,
									ReadMask: &fieldmaskpb.FieldMask{Paths: []string{"*"}},
								}
								var getErr error
								resp, getErr = rpcv2.NewLedgerServiceClient(conn).BatchGetObjects(ctx, req)
								return clientpool.Result{
									Err:           getErr,
									BrokenForTask: getErr != nil, // always retry using other client
								}
							},
						)
					},
					clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
				)
				if r.Err != nil {
					return nil, errors.Wrapf(r.Err, "load objects %s failed", utils.MustJSONMarshal(reqs))
				}
				if len(resp.GetObjects()) != len(reqs) {
					return nil, errors.Errorf("should get %d objects but got %d", len(reqs), len(resp.GetObjects()))
				}
				result := make([]*rpcv2.Object, len(resp.GetObjects()))
				for i, obj := range resp.GetObjects() {
					if obj.GetError() != nil {
						return nil, errors.Errorf("load object %s/%d failed: %s",
							reqs[i].GetObjectId(), reqs[i].GetVersion(), utils.MustJSONMarshal(obj.GetError()))
					}
					result[i] = obj.GetObject()
				}
				return result, nil
			},
		)
		if err != nil {
			return nil, err
		}
		ck.Objects = &rpcv2.ObjectSet{Objects: objects}
	} else {
		if err = s.loadTransactions(); err != nil {
			return nil, err
		}
	}
	s.removeBcs()
	return s, nil
}

func (d *ExtServerGrpcDimension) GetSlots(ctx context.Context, sr rg.Range) ([]*Slot, error) {
	slots := make([]*Slot, 0, *sr.Size())
	for sn := sr.Start; sn <= *sr.End; sn++ {
		st, err := d.GetSlot(ctx, sn)
		if err != nil {
			return nil, errors.Wrapf(err, "get slot %d failed", sn)
		}
		slots = append(slots, st)
	}
	return slots, nil
}

func (d *ExtServerGrpcDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	return &Slot{SlotCheckpointInfo: SlotCheckpointInfo{SequenceNumber: sn}}, nil
}

func (d *ExtServerGrpcDimension) Snapshot() any {
	sn := d.ExtServerDimension.Snapshot().(map[string]any)
	sn["kind"] = "grpc"
	sn["asSyncerSource"] = d.asSyncerSource
	sn["loadObjectsBatchSize"] = d.loadObjectsBatchSize
	sn["loadObjectsConcurrency"] = d.loadObjectsConcurrency
	return sn
}
