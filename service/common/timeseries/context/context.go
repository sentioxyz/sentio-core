package context

import (
	"context"

	commonmodels "sentioxyz/sentio-core/service/common/models"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/bytedance/sonic"
)

type ClickhouseCtxDataKey struct{}
type ClickhouseCtxQueryIdKey struct{}
type ClickhouseCtxSettingsKey struct{}

type ClickhouseCtxData struct {
	ProcessorID      string
	ProcessorVersion int
	ProjectID        string
	ProjectName      string
	UserID           string
	APIKeyID         string
	AsyncQueryID     string
	AsyncExecutionID string
	Method           string
}

func NewClickhouseCtxData(processor *processormodels.Processor, project *commonmodels.Project,
	identity *commonmodels.Identity, userID, queryID, executionID, method string) *ClickhouseCtxData {
	c := &ClickhouseCtxData{
		ProcessorID:      processor.ID,
		ProcessorVersion: int(processor.Version),
		ProjectID:        project.ID,
		ProjectName:      project.FullName(),
		Method:           method,
	}
	if identity != nil {
		c.UserID = identity.GetUserID()
		if identity.IsAuthByAPIKey() && identity.APIKey != nil {
			c.APIKeyID = identity.APIKey.ID
		}
	} else if userID != "" {
		c.UserID = userID
		c.AsyncQueryID = queryID
		c.AsyncExecutionID = executionID
	}
	return c
}

func (c *ClickhouseCtxData) CallSign() string {
	type StructLogComment struct {
		ProcessorID      string `json:"processor_id,omitempty"`
		ProcessorVersion int    `json:"processor_version,omitempty"`
		ProjectID        string `json:"project_id,omitempty"`
		ProjectName      string `json:"project_name,omitempty"`
		Method           string `json:"method,omitempty"`

		UserID           string `json:"user_id,omitempty"`
		APIKeyID         string `json:"api_key_id,omitempty"`
		AsyncQueryID     string `json:"async_query_id,omitempty"`
		AsyncExecutionID string `json:"async_execution_id,omitempty"`
	}

	var structLog = StructLogComment{
		ProjectID:        c.ProjectID,
		ProjectName:      c.ProjectName,
		ProcessorID:      c.ProcessorID,
		ProcessorVersion: c.ProcessorVersion,
		UserID:           c.UserID,
		APIKeyID:         c.APIKeyID,
		AsyncQueryID:     c.AsyncQueryID,
		AsyncExecutionID: c.AsyncExecutionID,
		Method:           c.Method,
	}

	res, err := sonic.Marshal(structLog)
	if err != nil {
		return ""
	}
	return string(res)
}

func SetClickhouseCtxData(ctx context.Context, data *ClickhouseCtxData) context.Context {
	return context.WithValue(ctx, ClickhouseCtxDataKey{}, data)
}

func GetClickhouseCtxDataCallSign(ctx context.Context) string {
	data, ok := ctx.Value(ClickhouseCtxDataKey{}).(*ClickhouseCtxData)
	if !ok {
		return ""
	}

	return data.CallSign()
}

func SetClickhouseCtxQueryId(ctx context.Context, queryId string) context.Context {
	return context.WithValue(ctx, ClickhouseCtxQueryIdKey{}, queryId)
}

func GetClickhouseCtxQueryId(ctx context.Context) string {
	queryId, ok := ctx.Value(ClickhouseCtxQueryIdKey{}).(string)
	if !ok {
		return ""
	}

	return queryId
}

func SetClickhouseCtxSettings(ctx context.Context, settings map[string]any) context.Context {
	exists, ok := ctx.Value(ClickhouseCtxSettingsKey{}).(map[string]any)
	if !ok {
		exists = make(map[string]any)
	}
	for k, v := range settings {
		exists[k] = v
	}
	return context.WithValue(ctx, ClickhouseCtxSettingsKey{}, exists)
}

func GetClickhouseCtxSettings(ctx context.Context) map[string]any {
	settings, ok := ctx.Value(ClickhouseCtxSettingsKey{}).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return settings
}
