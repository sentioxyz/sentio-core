package jsonrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/goccy/go-json"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
)

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  interface{}     `json:"result"            msgpack:"result,omitempty"`
}

func (m JsonrpcMessage) MarshalJSON() ([]byte, error) {
	if m.Error == nil {
		// always include result
		return json.Marshal(struct {
			Version string          `json:"jsonrpc,omitempty"`
			ID      json.RawMessage `json:"id,omitempty"`
			Method  string          `json:"method,omitempty"`
			Params  json.RawMessage `json:"params,omitempty"`
			Error   *jsonError      `json:"error,omitempty"`
			Result  interface{}     `json:"result"`
		}{
			Version: m.Version,
			ID:      m.ID,
			Method:  m.Method,
			Params:  m.Params,
			Error:   m.Error,
			Result:  m.Result,
		})
	}
	// no result field if error not nil and result is nil
	if utils.IsNil(m.Result) {
		// m.Result may be a typed nil like json.RawMessage(nil), it will not be ignored,
		// so set m.Result to no type nil here to make sure no result field
		m.Result = nil
	}
	return json.Marshal(struct {
		Version string          `json:"jsonrpc,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Method  string          `json:"method,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
		Error   *jsonError      `json:"error,omitempty"`
		Result  interface{}     `json:"result,omitempty"`
	}{
		Version: m.Version,
		ID:      m.ID,
		Method:  m.Method,
		Params:  m.Params,
		Error:   m.Error,
		Result:  m.Result,
	})
}

const (
	vsn                     = "2.0"
	defaultErrorCode        = -32000
	MaxRequestContentLength = 1024 * 1024 * 5
	contentType             = "application/json"
)

var acceptedContentTypes = []string{contentType, "application/json-rpc", "application/jsonrequest",
	"application/msgpack"}

func ValidateRPCRequest(r *http.Request) (int, error) {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}
	if r.ContentLength > MaxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, MaxRequestContentLength)
		return http.StatusRequestEntityTooLarge, err
	}
	// Allow OPTIONS (regardless of content-type)
	if r.Method == http.MethodOptions {
		return 0, nil
	}
	// Check content-type
	if mt, _, err := mime.ParseMediaType(r.Header.Get("content-type")); err == nil {
		for _, accepted := range acceptedContentTypes {
			if accepted == mt {
				return 0, nil
			}
		}
	}
	// Invalid content-type
	err := fmt.Errorf("invalid content type, only %s is supported", contentType)
	return http.StatusUnsupportedMediaType, err
}

// isBatch returns true when the first non-whitespace characters is '['
func isBatch(raw json.RawMessage) bool {
	for _, c := range raw {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
}

func ParseRPCMessage(ctx context.Context, raw json.RawMessage) ([]*JsonrpcMessage, bool) {
	logger := log.WithContext(ctx)
	var err error
	if !isBatch(raw) {
		msgs := []*JsonrpcMessage{{}}
		if err = json.Unmarshal(raw, &msgs[0]); err != nil {
			logger.Errore(err)
		}
		return msgs, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err = dec.Token(); err != nil { // skip '['
		logger.Errore(err)
	}
	var msgs []*JsonrpcMessage
	for dec.More() {
		msgs = append(msgs, new(JsonrpcMessage))
		if err = dec.Decode(&msgs[len(msgs)-1]); err != nil {
			logger.Errore(err)
		}
	}
	return msgs, true
}

func JSONErrorResponse(msg *JsonrpcMessage, result any, err error) *JsonrpcMessage {
	respErr := jsonError{
		Code:    defaultErrorCode,
		Message: err.Error(),
	}
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		respErr.Code = rpcErr.ErrorCode()
	}
	var dataErr rpc.DataError
	if errors.As(err, &dataErr) {
		respErr.Data = dataErr.ErrorData()
	}
	return &JsonrpcMessage{
		Version: vsn,
		ID:      msg.ID,
		Result:  result,
		Error:   &respErr,
	}
}

func JSONResponse(msg *JsonrpcMessage, result interface{}) *JsonrpcMessage {
	return &JsonrpcMessage{Version: vsn, ID: msg.ID, Result: result}
}

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	var args []reflect.Value
	tok, err := dec.Token()
	switch {
	case errors.Is(err, io.EOF) || tok == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case tok == json.Delim('['):
		// Read argument array.
		if args, err = parseArgumentArray(dec, types); err != nil {
			return nil, err
		}
	default:
		// Only one argument
		switch len(types) {
		case 1:
			// rebuild decoder to get back the first character shaved off by dec.Token
			dec = json.NewDecoder(bytes.NewReader(rawArgs))
			arg, err := parseArgument(dec, 0, types[0])
			if err != nil {
				return nil, err
			}
			return []reflect.Value{arg}, nil
		case 0:
			return nil, errors.New("do not need argument")
		default:
			return nil, errors.New("non-array args")
		}
	}
	// Set any missing args to zero value.
	for i := len(args); i < len(types); i++ {
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return args, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		arg, err := parseArgument(dec, i, types[i])
		if err != nil {
			return args, err
		}
		args = append(args, arg)
	}
	// Read end of args array.
	_, err := dec.Token()
	return args, err
}

func parseArgument(dec *json.Decoder, index int, argType reflect.Type) (val reflect.Value, err error) {
	argVal := reflect.New(argType)
	if err = dec.Decode(argVal.Interface()); err != nil {
		return val, fmt.Errorf("invalid argument %d: %w", index, err)
	}
	if argVal.IsNil() && argType.Kind() != reflect.Ptr {
		return val, fmt.Errorf("missing value for required argument %d", index)
	}
	return argVal.Elem(), nil
}
