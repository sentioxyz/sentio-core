package clientpool

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
)

func BuildHTTPRequest(
	ctx context.Context,
	method string,
	baseURL string,
	path string,
	params url.Values,
	headers http.Header,
	body []byte,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err // baseURL is invalid
	}
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	req.URL = req.URL.JoinPath(path)
	if params != nil && len(params) > 0 {
		req.URL.RawQuery = url.Values(utils.MergeMap(req.URL.Query(), params)).Encode()
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	return req, nil
}

var invalidRequestStatusCode = set.New[int](
	http.StatusBadRequest,
	http.StatusNotFound,
	http.StatusMethodNotAllowed,
	http.StatusNotAcceptable,
	http.StatusConflict,
	http.StatusGone,
	http.StatusLengthRequired,
	http.StatusPreconditionFailed,
	http.StatusRequestEntityTooLarge,
	http.StatusRequestURITooLong,
	http.StatusUnsupportedMediaType,
	http.StatusUnprocessableEntity,
	http.StatusUpgradeRequired,
	http.StatusPreconditionRequired,
	http.StatusRequestHeaderFieldsTooLarge,
	http.StatusUnavailableForLegalReasons,
)

func SendHTTP(
	client *http.Client,
	req *http.Request,
	result any,
) (resp *http.Response, body []byte, r Result) {
	resp, r.Err = client.Do(req)
	if r.Err != nil {
		r.Broken = !errors.Is(r.Err, context.Canceled) && !errors.Is(r.Err, context.DeadlineExceeded)
		return resp, nil, r
	}
	body, r.Err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if r.Err != nil {
		r.Broken = !errors.Is(r.Err, context.Canceled) && !errors.Is(r.Err, context.DeadlineExceeded)
		r.Err = errors.Wrapf(r.Err, "read response body failed")
		return resp, nil, r
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// status code not in [200,300)
		r.Err = errors.Errorf("[StatusCode:%d] %s", resp.StatusCode, string(body))
		r.Broken = !invalidRequestStatusCode.Contains(resp.StatusCode)
		return resp, body, r
	}
	if result != nil {
		if r.Err = json.Unmarshal(body, result); r.Err != nil {
			// result type is invalid
			r.Err = errors.Wrapf(r.Err, "unmarshal response body to %T failed", result)
			return resp, body, r
		}
	}
	return resp, body, r
}
