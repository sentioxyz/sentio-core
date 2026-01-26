package ckhmanager

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

type JWSHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type JWSPayload struct {
	Iat       int64  `json:"iat"`
	QueryHash string `json:"qhash"`
}

var (
	jwsHeaderV1     = JWSHeader{Alg: "ES256K", Typ: "JWS"}
	jwsHeaderBytes  []byte
	jwsHeaderBase64 string
)

func init() {
	jwsHeaderBytes, _ = json.Marshal(jwsHeaderV1)
	jwsHeaderBase64 = base64.RawURLEncoding.EncodeToString(jwsHeaderBytes)
}

func keccak256Hex(data []byte) string {
	return "0x" + hex.EncodeToString(crypto.Keccak256(data))
}

const (
	EthereumRecoveryIDOffset = 27
	EthereumSignatureLength  = 65
)

func createJWSToken(privateKey *ecdsa.PrivateKey, query string) (string, error) {
	payloadBytes, _ := json.Marshal(JWSPayload{
		Iat:       time.Now().Unix(),
		QueryHash: keccak256Hex([]byte(query)),
	})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := jwsHeaderBase64 + "." + payloadB64
	msgHash := crypto.Keccak256([]byte(signingInput))

	sig, err := crypto.Sign(msgHash, privateKey)
	if err != nil {
		return "", err
	}

	// Validate signature format
	if len(sig) != EthereumSignatureLength {
		return "", fmt.Errorf("invalid signature length: expected %d, got %d", EthereumSignatureLength, len(sig))
	}

	// Validate and convert recovery ID
	recoveryID := sig[64]
	if recoveryID > 1 {
		return "", fmt.Errorf("invalid recovery ID: expected 0 or 1, got %d", recoveryID)
	}

	// Create a copy to avoid modifying the original signature
	ethSig := make([]byte, EthereumSignatureLength)
	copy(ethSig, sig)
	ethSig[64] = recoveryID + EthereumRecoveryIDOffset

	sigB64 := base64.RawURLEncoding.EncodeToString(ethSig)
	return signingInput + "." + sigB64, nil
}
