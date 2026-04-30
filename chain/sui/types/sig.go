package types

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"sentioxyz/sentio-core/chain/sui/types/serde"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	SchemeEd25519 uint8 = iota
	SchemeSecp256k1
	SchemeSecp256r1
)

type CompressedSignature struct {
	ED25519   [64]byte
	Secp256k1 [64]byte
	Secp256r1 [64]byte
}

func (s CompressedSignature) IsBcsEnum() {}

type PublicKey struct {
	ED25519   [32]byte
	Secp256k1 [33]byte
	Secp256r1 [33]byte
}

func (s PublicKey) IsBcsEnum() {}

type MultiSigPkMap struct {
	PubKey PublicKey
	Weight uint8
}

type MultiSigPublicKey struct {
	PkMap     []MultiSigPkMap
	Threshold uint16
}

type MultiSig struct {
	Sigs      []CompressedSignature
	Bitmap    uint16
	PublicKey MultiSigPublicKey
}

func (s MultiSig) DebugString() string {
	sb := strings.Builder{}
	sb.WriteString("MultiSig {\n")
	sb.WriteString("  Sigs: [\n")
	for _, sig := range s.Sigs {
		sb.WriteString("    ")
		switch {
		case sig.ED25519 != [64]byte{}:
			sb.WriteString("ED25519(")
			sb.WriteString(hexutil.Encode(sig.ED25519[:]))
		case sig.Secp256k1 != [64]byte{}:
			sb.WriteString("Secp256k1(")
			sb.WriteString(hexutil.Encode(sig.Secp256k1[:]))
		case sig.Secp256r1 != [64]byte{}:
			sb.WriteString("Secp256r1(")
			sb.WriteString(hexutil.Encode(sig.Secp256r1[:]))
		}
		sb.WriteString("),\n")
	}
	sb.WriteString("  ],\n")
	sb.WriteString("  Bitmap: ")
	sb.WriteString(fmt.Sprintf("%d", s.Bitmap))
	sb.WriteString(",\n")
	sb.WriteString("  PublicKey: {\n")
	sb.WriteString("    PkMap: [\n")
	for _, pkMap := range s.PublicKey.PkMap {
		sb.WriteString("      {\n")
		sb.WriteString("        PubKey: ")
		switch {
		case pkMap.PubKey.ED25519 != [32]byte{}:
			sb.WriteString("ED25519(")
			sb.WriteString(hexutil.Encode(pkMap.PubKey.ED25519[:]))
		case pkMap.PubKey.Secp256k1 != [33]byte{}:
			sb.WriteString("Secp256k1(")
			sb.WriteString(hexutil.Encode(pkMap.PubKey.Secp256k1[:]))
		case pkMap.PubKey.Secp256r1 != [33]byte{}:
			sb.WriteString("Secp256r1(")
			sb.WriteString(hexutil.Encode(pkMap.PubKey.Secp256r1[:]))
		}
		sb.WriteString("),\n")
		sb.WriteString("        Weight: ")
		sb.WriteString(fmt.Sprintf("%d", pkMap.Weight))
		sb.WriteString(",\n")
		sb.WriteString("      },\n")
	}
	sb.WriteString("    ],\n")
	sb.WriteString("    Threshold: ")
	sb.WriteString(fmt.Sprintf("%d", s.PublicKey.Threshold))
	sb.WriteString(",\n")
	sb.WriteString("  },\n")
	sb.WriteString("}\n")
	return sb.String()
}

var ErrNotMultiSig = fmt.Errorf("not a multi sig")

func DecodeMultiSig(s string) (*MultiSig, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return DecodeMultiSigBytes(b)
}

func DecodeMultiSigBytes(b []byte) (*MultiSig, error) {
	if len(b) < 1 || b[0] != 0x03 {
		return nil, ErrNotMultiSig
	}
	sig := MultiSig{}
	r := bytes.NewReader(b[1:])
	if err := serde.Decode(r, &sig); err != nil {
		return nil, err
	}
	return &sig, nil
}

func IsMultiSigBase64(s string) bool {
	if len(s) < 2 {
		return false
	}
	prefix := s[:2]
	return prefix[0] == 'A' && ( // The second rune represents a 11xxxx
	(prefix[1] >= 'w' && prefix[1] <= 'z') ||
		(prefix[1] >= '0' && prefix[1] <= '9') ||
		prefix[1] == '+' || prefix[1] == '/')
}

func IsMultiSigBytes(s []byte) bool {
	return len(s) > 0 && s[0] == 0x03
}

func IsZkLoginSigBytes(s []byte) bool {
	return len(s) > 0 && s[0] == 0x05
}
