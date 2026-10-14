package types

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"sentioxyz/sentio-core/chain/sui/types/serde"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
)

const (
	SchemeEd25519 uint8 = iota
	SchemeSecp256k1
	SchemeSecp256r1
)

// CompressedSignature and PublicKey follow the variant order of the upstream sui-types enums
// (crypto.rs). ZkLogin and Passkey carry a length-prefixed payload; MLDSA65 (protocol version 139)
// is a raw fixed-size array like the classic schemes.
type CompressedSignature struct {
	ED25519   [64]byte
	Secp256k1 [64]byte
	Secp256r1 [64]byte
	ZkLogin   []byte
	Passkey   []byte
	MLDSA65   [3309]byte
}

func (s CompressedSignature) IsBcsEnum() {}

func (s CompressedSignature) variant() (string, []byte) {
	switch {
	case s.ED25519 != [64]byte{}:
		return "ED25519", s.ED25519[:]
	case s.Secp256k1 != [64]byte{}:
		return "Secp256k1", s.Secp256k1[:]
	case s.Secp256r1 != [64]byte{}:
		return "Secp256r1", s.Secp256r1[:]
	case s.ZkLogin != nil:
		return "ZkLogin", s.ZkLogin
	case s.Passkey != nil:
		return "Passkey", s.Passkey
	case s.MLDSA65 != [3309]byte{}:
		return "MLDSA65", s.MLDSA65[:]
	}
	return "", nil
}

type PublicKey struct {
	ED25519   [32]byte
	Secp256k1 [33]byte
	Secp256r1 [33]byte
	ZkLogin   []byte
	Passkey   [33]byte
	MLDSA65   [1952]byte
}

func (s PublicKey) IsBcsEnum() {}

func (s PublicKey) variant() (string, []byte) {
	switch {
	case s.ED25519 != [32]byte{}:
		return "ED25519", s.ED25519[:]
	case s.Secp256k1 != [33]byte{}:
		return "Secp256k1", s.Secp256k1[:]
	case s.Secp256r1 != [33]byte{}:
		return "Secp256r1", s.Secp256r1[:]
	case s.ZkLogin != nil:
		return "ZkLogin", s.ZkLogin
	case s.Passkey != [33]byte{}:
		return "Passkey", s.Passkey[:]
	case s.MLDSA65 != [1952]byte{}:
		return "MLDSA65", s.MLDSA65[:]
	}
	return "", nil
}

// Bytes returns the raw key of whichever scheme is set, or nil if none is.
func (s PublicKey) Bytes() []byte {
	_, b := s.variant()
	return b
}

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
		name, b := sig.variant()
		sb.WriteString("    " + name + "(" + hexutil.Encode(b) + "),\n")
	}
	sb.WriteString("  ],\n")
	sb.WriteString("  Bitmap: ")
	sb.WriteString(fmt.Sprintf("%d", s.Bitmap))
	sb.WriteString(",\n")
	sb.WriteString("  PublicKey: {\n")
	sb.WriteString("    PkMap: [\n")
	for _, pkMap := range s.PublicKey.PkMap {
		sb.WriteString("      {\n")
		name, b := pkMap.PubKey.variant()
		sb.WriteString("        PubKey: " + name + "(" + hexutil.Encode(b) + "),\n")
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

var ErrNotMultiSig = errors.Errorf("not a multi sig")

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
