package standard

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/processor/protos"
)

func TestValidateNoResponse(t *testing.T) {
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		Op: &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: "E", Id: "1"}},
	}), "a plain read is fine")
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Upsert{Upsert: &protos.DBRequest_DBUpsert{}},
	}), "fire-and-forget upsert")
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Delete{Delete: &protos.DBRequest_DBDelete{}},
	}), "fire-and-forget delete")
	require.Error(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: "E", Id: "1"}},
	}), "a read without a waiter would hang the processor")
	require.Error(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_List{List: &protos.DBRequest_DBList{Entity: "E"}},
	}), "a list without a waiter would hang the processor")
}

// fakeStream records what the task sends and whether it closed the stream.
type fakeStream struct {
	grpc.ClientStream
	sent   []*protos.ProcessStreamRequest
	closed bool
}

func (f *fakeStream) Send(req *protos.ProcessStreamRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

func (f *fakeStream) Recv() (*protos.ProcessStreamResponseV3, error) {
	return nil, io.EOF
}

func (f *fakeStream) CloseSend() error {
	f.closed = true
	return nil
}

func newStreamTestTask(fs *fakeStream) *task {
	_, logger := log.FromContext(context.Background())
	return &task{
		sp:     make(streamPool, 1),
		stream: fs,
		timer:  timer.NewTimer(),
		logger: logger,
	}
}

func TestReleaseStream(t *testing.T) {
	fs := &fakeStream{}
	b := newStreamTestTask(fs)
	b.releaseStream(nil)
	require.Len(t, b.sp, 1, "a successful task hands its stream back")
	require.False(t, fs.closed)

	b.stream = <-b.sp
	b.releaseStream(controller.NewExternalError(controller.ErrCodeSystem, errors.New("boom")))
	require.Empty(t, b.sp, "a failed task's stream never goes back to the pool")
	require.True(t, fs.closed, "it is closed instead")
	require.Nil(t, b.stream)
}

func TestReportFailedWrite(t *testing.T) {
	failure := controller.NewExternalError(controller.ErrCodeSystem, errors.New("boom"))

	fs := &fakeStream{}
	b := newStreamTestTask(fs)
	b.executing = &protos.DBRequest{
		OpId:       7,
		NoResponse: true,
		Op: &protos.DBRequest_Upsert{Upsert: &protos.DBRequest_DBUpsert{
			Entity: []string{"E"}, Id: []string{"1"},
		}},
	}
	b.reportFailedWrite(context.Background(), failure)
	require.Len(t, fs.sent, 1, "a failed no_response write is answered")
	require.Equal(t, uint64(7), fs.sent[0].GetDbResult().GetOpId())
	require.Contains(t, fs.sent[0].GetDbResult().GetError(), "boom")

	fs = &fakeStream{}
	b = newStreamTestTask(fs)
	b.executing = &protos.DBRequest{
		OpId: 8,
		Op:   &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: "E", Id: "1"}},
	}
	b.reportFailedWrite(context.Background(), failure)
	require.Empty(t, fs.sent, "an acknowledged op gets no answer on failure, as before")

	b.executing = nil
	b.reportFailedWrite(context.Background(), failure)
	require.Empty(t, fs.sent, "nothing in flight, nothing to answer")
}
