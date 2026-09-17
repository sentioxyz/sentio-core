package utils

import (
	"fmt"
	"net/url"
	"strings"
)

func mosaic(ch byte, len int) string {
	s := make([]byte, len)
	for i := range s {
		s[i] = ch
	}
	return string(s)
}

// AddSecretMosaic replaces a secret with a hint: how long it is, plus at most a fifth of its
// characters, the first and last len/10 of them. A secret shorter than ten characters is hidden
// entirely. The hint is there so that an operator reading a log can tell a misconfigured value from
// the intended one, without the log handing out the secret itself.
func AddSecretMosaic(secret string) string {
	runes := []rune(secret)
	visible := len(runes) / 10
	if visible == 0 {
		return fmt.Sprintf("***(%d)", len(runes))
	}
	return fmt.Sprintf("%s***%s(%d)", string(runes[:visible]), string(runes[len(runes)-visible:]), len(runes))
}

// AddURLMosaic hides the credentials of a URL so that it can safely be logged, replacing them with
// the hint AddSecretMosaic produces. Masked are the userinfo password ("scheme://user:password@host")
// and the "password" query parameter, the two places a ClickHouse DSN carries one. A URL that cannot
// be parsed is reported as a placeholder rather than returned as is: it may still hold a password,
// and there is no reliable way to locate it.
//
// The secrets are cut out of the URL as it was configured rather than re-rendered from the parsed
// form, so that what reaches the log is the URL an operator can compare against the configuration.
func AddURLMosaic(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if _, err := url.Parse(rawURL); err != nil {
		return "<unparsable url>"
	}
	head, query, hasQuery := strings.Cut(rawURL, "?")
	masked := addUserinfoMosaic(head)
	if hasQuery {
		masked += "?" + addPasswordParamMosaic(query)
	}
	return masked
}

// addUserinfoMosaic masks the password of "scheme://user:password@host/path". The authority ends at
// the first "/", and its userinfo at the last "@", the way url.Parse splits them.
func addUserinfoMosaic(head string) string {
	scheme, rest, found := strings.Cut(head, "://")
	if !found {
		return head
	}
	authority, path, hasPath := strings.Cut(rest, "/")
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return head
	}
	userinfo, host := authority[:at], authority[at:]
	if username, password, found := strings.Cut(userinfo, ":"); found && password != "" {
		// The userinfo is percent-encoded; describe the configured secret, not its encoding.
		decoded, err := url.PathUnescape(password)
		if err != nil {
			decoded = password
		}
		authority = username + ":" + AddSecretMosaic(decoded) + host
	}
	masked := scheme + "://" + authority
	if hasPath {
		masked += "/" + path
	}
	return masked
}

// addPasswordParamMosaic masks the "password" parameter of a query string, which is the other place
// clickhouse.ParseDSN reads the credentials from.
func addPasswordParamMosaic(query string) string {
	params := strings.Split(query, "&")
	for i, param := range params {
		key, value, found := strings.Cut(param, "=")
		if found && key == "password" && value != "" {
			decoded, err := url.QueryUnescape(value)
			if err != nil {
				decoded = value
			}
			params[i] = key + "=" + AddSecretMosaic(decoded)
		}
	}
	return strings.Join(params, "&")
}

func AddOwnerNameMosaic(raw string) string {
	rl := len(raw)
	if rl <= 2 {
		return raw
	}
	vl := min(3, rl/3)
	return raw[:vl] + mosaic('*', rl-vl*2) + raw[rl-vl:]
}
