package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sha256Hello = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

func TestSHA256ReaderComputesDigest(t *testing.T) {
	result, err := SHA256Reader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("sha256 reader: %v", err)
	}
	if result.Hex != sha256Hello || result.Bytes != 5 || result.Algorithm != SHA256ChecksumName {
		t.Fatalf("result = %+v", result)
	}
}

func TestSHA256FileAndVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := VerifySHA256File(path, sha256Hello)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Path != path || result.Hex != sha256Hello {
		t.Fatalf("result = %+v", result)
	}
}

func TestVerifySHA256FileRejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := VerifySHA256File(path, strings.Repeat("0", 64))
	if err == nil {
		t.Fatalf("expected mismatch")
	}
}

func TestValidateSHA256Hex(t *testing.T) {
	if !IsSHA256Hex(strings.ToUpper(sha256Hello)) {
		t.Fatalf("expected uppercase digest to validate")
	}
	if IsSHA256Hex("not-a-digest") {
		t.Fatalf("expected invalid digest")
	}
}
