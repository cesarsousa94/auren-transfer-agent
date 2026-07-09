package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// IntegrityValidatorName is the canonical upload integrity validator name.
	IntegrityValidatorName = "integrity_validation"

	// SHA256Algorithm is the supported foundation integrity algorithm.
	SHA256Algorithm = "sha256"
)

// IntegrityResult describes the mechanical comparison between source and destination.
type IntegrityResult struct {
	Algorithm           string `json:"algorithm"`
	SourcePath          string `json:"source_path"`
	DestinationPath     string `json:"destination_path"`
	SourceSize          int64  `json:"source_size"`
	DestinationSize     int64  `json:"destination_size"`
	SourceChecksum      string `json:"source_checksum"`
	DestinationChecksum string `json:"destination_checksum"`
	SizeMatch           bool   `json:"size_match"`
	ChecksumMatch       bool   `json:"checksum_match"`
	Valid               bool   `json:"valid"`
}

// ValidateIntegrity compares source and destination size and SHA-256 checksum.
func ValidateIntegrity(ctx context.Context, sourcePath string, destinationPath string) (IntegrityResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(sourcePath) == "" {
		return IntegrityResult{}, fmt.Errorf("integrity source path is required")
	}
	if strings.TrimSpace(destinationPath) == "" {
		return IntegrityResult{}, fmt.Errorf("integrity destination path is required")
	}
	sourceSize, sourceChecksum, err := checksumFile(ctx, sourcePath)
	if err != nil {
		return IntegrityResult{}, err
	}
	destinationSize, destinationChecksum, err := checksumFile(ctx, destinationPath)
	if err != nil {
		return IntegrityResult{}, err
	}
	result := IntegrityResult{Algorithm: SHA256Algorithm, SourcePath: sourcePath, DestinationPath: destinationPath, SourceSize: sourceSize, DestinationSize: destinationSize, SourceChecksum: sourceChecksum, DestinationChecksum: destinationChecksum}
	result.SizeMatch = result.SourceSize == result.DestinationSize
	result.ChecksumMatch = result.SourceChecksum == result.DestinationChecksum
	result.Valid = result.SizeMatch && result.ChecksumMatch
	return result, nil
}

// ValidateResultIntegrity validates the files referenced by an upload result.
func ValidateResultIntegrity(ctx context.Context, result Result) (IntegrityResult, error) {
	return ValidateIntegrity(ctx, result.SourcePath, result.DestinationPath)
}

func checksumFile(ctx context.Context, path string) (int64, string, error) {
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	if info.IsDir() {
		return 0, "", fmt.Errorf("integrity path must be a file")
	}
	hash := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := hash.Write(buffer[:read]); err != nil {
				return 0, "", err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return 0, "", readErr
		}
	}
	return info.Size(), hex.EncodeToString(hash.Sum(nil)), nil
}
