package authsvc

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// KeyType is the PEM block type for RSA private keys.
const KeyType = "RSA PRIVATE KEY"

// DefaultKeySize is the default RSA key size in bits.
const DefaultKeySize = 2048

// DecodePrivateKey reads and decodes a PEM-encoded RSA private key.
// Returns an error if the key cannot be read or is not a valid RSA private key.
func DecodePrivateKey(key io.Reader) (*rsa.PrivateKey, error) {
	buf, err := io.ReadAll(key)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	block, _ := pem.Decode(buf)
	if block == nil {
		return nil, fmt.Errorf("decode key: %w", err)
	} else if block.Type != KeyType {
		return nil, fmt.Errorf("decode key: %w", domain.ErrInvalidAuthToken)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	return privateKey, nil
}

// GeneratePrivateKey creates a new RSA private key with the specified bit size.
// Returns an error if key generation fails.
func GeneratePrivateKey(bits int) (*rsa.PrivateKey, error) {
	signingKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	return signingKey, nil
}

// EncodePrivateKey encodes an RSA private key in PEM format.
// Returns the PEM-encoded key bytes or an error if encoding fails.
func EncodePrivateKey(signingKey *rsa.PrivateKey) ([]byte, error) {
	keyBytes := x509.MarshalPKCS1PrivateKey(signingKey)
	//nolint:exhaustruct
	pemBlock := &pem.Block{
		Type:  KeyType,
		Bytes: keyBytes,
	}

	var (
		buf bytes.Buffer
		out = io.Writer(&buf)
	)

	if err := pem.Encode(out, pemBlock); err != nil {
		return nil, fmt.Errorf("encode key: %w", err)
	}

	return buf.Bytes(), nil
}

// GetPrivateKey loads or creates an RSA private key from the specified file path.
// If the file exists, it loads and decodes the key.
// If the file doesn't exist, it generates a new key and saves it to the file.
// Returns an error if any operation fails.
func GetPrivateKey(path string) (*rsa.PrivateKey, error) {
	// Try decode existing key
	keyFile, err := os.Open(path)
	if err == nil {
		signingKey, err := DecodePrivateKey(keyFile)
		if err != nil {
			return nil, fmt.Errorf("decode private key: %w", err)
		}

		return signingKey, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open key file: %w", err)
	}

	// Generate new key
	signingKey, err := GeneratePrivateKey(DefaultKeySize)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	// Encode key
	keyBytes, err := EncodePrivateKey(signingKey)
	if err != nil {
		return nil, fmt.Errorf("encode private key: %w", err)
	}

	// Write key to file
	keyFile, err = os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create key file: %w", err)
	}
	defer keyFile.Close()

	if _, err := keyFile.Write(keyBytes); err != nil {
		return nil, fmt.Errorf("write key file: %w", err)
	}

	return signingKey, nil
}
