package cache

import (
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/rpccache/compression"

	"github.com/go-faster/errors"
	"github.com/vmihailenco/msgpack/v5"
)

type ctxKey struct{}

var refreshBgKey = ctxKey{}

type cachedResponse[Y Response] struct {
	Body      string
	Timestamp int64
}

func unmarshalCachedResponse[Y Response](data []byte) (*cachedResponse[Y], error) {
	resp := &cachedResponse[Y]{}
	if err := msgpack.Unmarshal(data, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func newCachedResponse[Y Response](logger *log.SentioLogger, response Y) (*cachedResponse[Y], error) {
	bytes, err := compression.Encode[Y](&response)
	if err != nil {
		logger.Warnf("rpc cache response encode failed: %v", err)
		return nil, err
	}
	return &cachedResponse[Y]{Body: bytes, Timestamp: time.Now().UnixMilli()}, nil
}

func (r *cachedResponse[Y]) Response() (resp *Y, err error) {
	if r == nil {
		return nil, errors.Errorf("nil cached response")
	}
	resp, err = compression.Decode[Y](r.Body)
	return
}

func (r *cachedResponse[Y]) Marshal() ([]byte, error) {
	return msgpack.Marshal(r)
}

func (r *cachedResponse[Y]) Newer(respData []byte) bool {
	resp := &cachedResponse[Y]{}
	if err := msgpack.Unmarshal(respData, resp); err != nil {
		return true
	}
	return r.Timestamp > resp.Timestamp
}
