package ckhmanager

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseDSNErrorHidesPassword(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{name: "invalid option", dsn: "clickhouse://sentio:s3cr3t@ch-0:9000/default?dial_timeout=nope"},
		// The driver reports a malformed DSN as a *url.Error carrying the whole DSN.
		{name: "malformed dsn", dsn: "clickhouse://sentio:s3cr3t@ch-0:9000/%zz"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseDSN(test.dsn)
			if err == nil {
				t.Fatal("ParseDSN succeeded, want an error")
			}
			if strings.Contains(err.Error(), "s3cr3t") {
				t.Errorf("ParseDSN error leaks the password: %v", err)
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
