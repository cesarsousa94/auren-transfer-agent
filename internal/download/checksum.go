package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// SHA256ChecksumName is the canonical checksum algorithm used by the download engine.
	SHA256ChecksumName = "sha256"
	sha256HexLength    = 64
)

// SHA256Result describes a computed SHA-256 checksum.
type SHA256Result struct {
	Algorithm string `json:"algorithm"`
	Hex       string `json:"hex"`
	Bytes     int64  `json:"bytes"`
	Path      string `json:"path,omitempty"`
}

// SHA256Reader streams reader into a SHA-256 digest without buffering the full payload.
func SHA256Reader(reader io.Reader) (SHA256Result, error) {
	if reader == nil {
		return SHA256Result{}, fmt.Errorf("sha256 reader is required")
	}
	hash := sha256.New()
	written, err := io.Copy(hash, reader)
	if err != nil {
		return SHA256Result{}, err
	}
	return SHA256Result{Algorithm: SHA256ChecksumName, Hex: hex.EncodeToString(hash.Sum(nil)), Bytes: written}, nil
}

// SHA256File computes the SHA-256 checksum for a local file.
func SHA256File(path string) (SHA256Result, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return SHA256Result{}, fmt.Errorf("sha256 file path is required")
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return SHA256Result{}, err
	}
	defer file.Close()
	result, err := SHA256Reader(file)
	if err != nil {
		return SHA256Result{}, err
	}
	result.Path = trimmed
	return result, nil
}

// ValidateSHA256Hex validates a canonical SHA-256 hex digest.
func ValidateSHA256Hex(value string) error {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if len(trimmed) != sha256HexLength {
		return fmt.Errorf("sha256 hex digest must have %d characters", sha256HexLength)
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return fmt.Errorf("sha256 hex digest is invalid")
	}
	if len(decoded) != sha256.Size {
		return fmt.Errorf("sha256 hex digest has invalid size")
	}
	return nil
}

// VerifySHA256File computes a file digest and compares it with the expected digest.
func VerifySHA256File(path string, expected string) (SHA256Result, error) {
	if err := ValidateSHA256Hex(expected); err != nil {
		return SHA256Result{}, err
	}
	result, err := SHA256File(path)
	if err != nil {
		return SHA256Result{}, err
	}
	if result.Hex != strings.ToLower(strings.TrimSpace(expected)) {
		return result, fmt.Errorf("sha256 mismatch: expected %s got %s", strings.ToLower(strings.TrimSpace(expected)), result.Hex)
	}
	return result, nil
}

// IsSHA256Hex returns true when value is a valid SHA-256 hex digest.
func IsSHA256Hex(value string) bool {
	return ValidateSHA256Hex(value) == nil
}
