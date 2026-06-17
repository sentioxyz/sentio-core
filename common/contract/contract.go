package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/https"
)

type TokenResult struct {
	Decimals *int    `json:"decimals"`
	Logo     *string `json:"logo"`
	Name     string  `json:"name"`
	Symbol   string  `json:"symbol"`
}

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
	Result  *TokenResult    `json:"result,omitempty"`
}

// IsERC20 refers from https://docs.alchemy.com/reference/alchemy-gettokenmetadata
func IsERC20(ctx context.Context, apiEndpoint string, tokenAddress string) (bool, error) {
	payload := strings.NewReader(
		fmt.Sprintf(
			"{\"id\":1,\"jsonrpc\":\"2.0\",\"method\":\"alchemy_getTokenMetadata\",\"params\":[\"%s\"]}",
			tokenAddress,
		),
	)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiEndpoint, payload)

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")

	res, err := https.DefaultClient.Do(req)

	if err != nil {
		return false, errors.WithStack(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return false, errors.New("HTTP request return" + res.Status)
	}

	body, _ := io.ReadAll(res.Body)
	//bodyString := string(body)

	var jsonrpcRes JsonrpcMessage
	err = json.Unmarshal(body, &jsonrpcRes)
	if err != nil {
		return false, err
	}

	if jsonrpcRes.Result != nil {
		if jsonrpcRes.Result.Decimals != nil {
			return true, nil
		}
	}

	return false, nil
}

// IsERC20New refers from https://docs.moralis.io/reference/gettokenmetadata
func IsERC20New(ctx context.Context, apiKey string, chainID string, tokenAddress string) (bool, error) {
	info, err := getERC20Info(ctx, apiKey, chainID, []string{tokenAddress})
	if err != nil {
		return false, err
	}
	return info[0].Symbol != "" && info[0].Decimals != "", nil
}

type ERC20Token struct {
	Address     string `json:"address"`
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Decimals    string `json:"decimals"`
	Logo        string `json:"logo,omitempty"`
	LogoHash    string `json:"logo_hash,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	BlockNumber string `json:"block_number,omitempty"`
	//Validated   string `json:"validated,omitempty"`
}

// https://docs.moralis.io/reference/gettokenmetadata
func getERC20Info(ctx context.Context, apiKey string, chainID string, tokenAddress []string) ([]ERC20Token, error) {
	var chain string
	switch chainID {
	case "1":
		chain = "eth"
	case "5":
		chain = "goerli"
	case "56":
		chain = "bsc"
	// TDDO add more mappings
	default:
		return nil, errors.New("chainID not supported")
	}

	var addresses = ""
	for _, address := range tokenAddress {
		addresses += "&addresses=" + address
	}

	url := "https://deep-index.moralis.io/api/v2/erc20/metadata?&chain=" + chain + addresses

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("X-API-Key", apiKey)

	res, err := https.DefaultClient.Do(req)

	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP request return" + res.Status)
	}

	body, _ := io.ReadAll(res.Body)

	var tokenInfo []ERC20Token
	err = json.Unmarshal(body, &tokenInfo)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return tokenInfo, nil
}
