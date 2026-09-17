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
		"writer:{Username:sentio Password:***(6) Database:default}]"
	if got != want {
		t.Errorf("printed credentials = %q, want %q", got, want)
	}
}

func TestLogSerializationMasksThePrivateKey(t *testing.T) {
	const privateKeyHex = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"
	var options Options
	ConnectWithPrivateKey(privateKeyHex)(&options)

	// The cache key keeps the raw key: two connections signing with different keys are different.
	if !strings.Contains(options.Serialization(), privateKeyHex) {
		t.Errorf("Serialization dropped the private key: %s", options.Serialization())
	}
	logged := options.LogSerialization()
	if strings.Contains(logged, privateKeyHex) {
		t.Errorf("LogSerialization leaks the private key: %s", logged)
	}
	if want := "private_key=b71c71***a3f291(64)"; !strings.Contains(logged, want) {
		t.Errorf("LogSerialization = %q, want it to contain %q", logged, want)
	}
}
