package models

import (
	commonProtos "sentioxyz/sentio-core/service/common/protos"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

type RequestLog struct {
	ID             string
	CreatedAt      time.Time
	EndpointID     *string
	ProjectID      string
	StatusCode     uint32
	RequestBody    string
	ResponseBody   string
	RequestHeader  datatypes.JSON
	ResponseHeader datatypes.JSON
	QueryTime      *uint64
	Duration       *uint64
	Method         string
	URL            string
	RPCNodeID      string
	Caller         string
	Slug           string
	EndpointType   string
	ChainID        string
	TargetURL      string
}

func (r *RequestLog) ToPB() *commonProtos.RequestLog {
	ret := &commonProtos.RequestLog{
		RequestId:    r.ID,
		CreatedAt:    timestamppb.New(r.CreatedAt),
		StatusCode:   r.StatusCode,
		RequestBody:  []byte(r.RequestBody),
		ResponseBody: []byte(r.ResponseBody),
		Duration:     *r.Duration, RequestHeader: &structpb.Struct{Fields: make(map[string]*structpb.Value)},
		Method:       r.Method,
		OriginUrl:    r.URL,
		Slug:         r.Slug,
		EndpointType: r.EndpointType,
		ChainId:      r.ChainID,
	}
	if r.QueryTime != nil {
		ret.QueryDuration = *r.QueryTime
	}
	if r.EndpointID != nil {
		ret.EndpointId = *r.EndpointID
	}
	if len(r.RequestHeader) > 0 {
		ret.RequestHeader, _ = structpb.NewStruct(nil)
		_ = ret.RequestHeader.UnmarshalJSON(r.RequestHeader)
		ret.RequestHeader = filterBlacklistedHeader(ret.RequestHeader)
	}

	if len(r.ResponseHeader) > 0 {
		ret.ResponseHeader, _ = structpb.NewStruct(nil)
		_ = ret.ResponseHeader.UnmarshalJSON(r.ResponseHeader)
		ret.ResponseHeader = filterBlacklistedHeader(ret.ResponseHeader)
	}
	if r.RPCNodeID != "" {
		ret.RpcNodeId = r.RPCNodeID
	}

	return ret
}

var BlacklistedHeadersPrefix = []string{
	"X-Envoy-",
	"X-Forwarded-For",
}

func filterBlacklistedHeader(originHeaders *structpb.Struct) *structpb.Struct {
	ret := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
	for key, value := range originHeaders.Fields {
		blacklisted := false
		for _, prefix := range BlacklistedHeadersPrefix {
			if strings.HasPrefix(key, prefix) {
				blacklisted = true
				break
			}
		}
		if !blacklisted {
			ret.Fields[key] = value
		}
	}
	return ret
}
