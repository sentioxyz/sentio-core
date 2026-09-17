package ckhmanager

import (
	"fmt"
	"strings"
	"testing"
)

func TestMaskDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "userinfo password",
			dsn:  "clickhouse://sentio:s3cr3t@ch-0.sentio.xyz:9000/default",
			want: "clickhouse://sentio:xxxxx@ch-0.sentio.xyz:9000/default",
		},
		{
			name: "multiple hosts",
			dsn:  "clickhouse://sentio:s3cr3t@ch-0:9000,ch-1:9000/default",
			want: "clickhouse://sentio:xxxxx@ch-0:9000,ch-1:9000/default",
		},
		{
			name: "password query parameter",
			dsn:  "tcp://ch-0:9000/default?username=sentio&password=s3cr3t",
			want: "tcp://ch-0:9000/default?password=xxxxx&username=sentio",
		},
		{
			name: "both places",
			dsn:  "https://sentio:s3cr3t@ch-0:8443/default?password=other",
			want: "https://sentio:xxxxx@ch-0:8443/default?password=xxxxx",
		},
		{
			name: "no password",
			dsn:  "clickhouse://sentio@ch-0:9000/default?secure=true",
			want: "clickhouse://sentio@ch-0:9000/default?secure=true",
		},
		{
			name: "empty dsn",
			dsn:  "",
			want: "",
		},
		{
			name: "unparsable dsn is not echoed back",
			dsn:  "clickhouse://sentio:s3cr3t@ch-0:9000/%zz",
			want: "<unparsable dsn>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MaskDSN(test.dsn); got != test.want {
				t.Errorf("MaskDSN(%q) = %q, want %q", test.dsn, got, test.want)
			}
		})
	}
}

func TestCredentialStringMasksPassword(t *testing.T) {
	credentials := map[string]Credential{
		"writer": {Username: "sentio", Password: "s3cr3t", Database: "default"},
		"reader": {Username: "reader", Database: "default"},
	}
	got := fmt.Sprintf("%v", credentials)
	want := "map[reader:{Username:reader Password: Database:default} " +
		"writer:{Username:sentio Password:xxxxx Database:default}]"
	if got != want {
		t.Errorf("printed credentials = %q, want %q", got, want)
	}
}

func TestParseDSNErrorHidesPassword(t *testing.T) {
	dsn := "clickhouse://sentio:s3cr3t@ch-0:9000/default?dial_timeout=nope"
	if _, err := ParseDSN(dsn); err == nil {
		t.Fatal("ParseDSN succeeded, want an error")
	} else if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("ParseDSN error leaks the password: %v", err)
	}

	// The driver reports a malformed DSN as a *url.Error carrying the whole DSN.
	dsn = "clickhouse://sentio:s3cr3t@ch-0:9000/%zz"
	if _, err := ParseDSN(dsn); err == nil {
		t.Fatal("ParseDSN succeeded, want an error")
	} else if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("ParseDSN error leaks the password: %v", err)
	}
}
