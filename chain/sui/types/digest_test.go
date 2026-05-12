package types

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestDecodeDigest(t *testing.T) {
	d, err := StrToDigest("13PrnWn4KTma3AyMATzP255eKu8XZkkm4v1nGMGtWV5G")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(d.String(), hex.EncodeToString(d[:]))
}
