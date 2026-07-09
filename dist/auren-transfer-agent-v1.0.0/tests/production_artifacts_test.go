package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionArtifactsExist(t *testing.T) {
	files := []string{
		"docker/Dockerfile",
		"docker/docker-compose.yml",
		"deploy/linux/install.sh",
		"deploy/systemd/auren-transfer-agent.service",
		"deploy/systemd/auren-transfer-agent.env.example",
		"deploy/kubernetes/auren-transfer-agent.yaml",
		".github/workflows/ci.yml",
		"scripts/release.sh",
		"docs/deployment/production.md",
	}

	for _, file := range files {
		path := filepath.Join("..", file)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected production artifact %s: %v", file, err)
		}
		if info.IsDir() {
			t.Fatalf("expected production artifact %s to be a file", file)
		}
	}
}

func TestProductionArtifactsReferenceVersion(t *testing.T) {
	files := []string{
		"docker/Dockerfile",
		"docker/docker-compose.yml",
		"deploy/kubernetes/auren-transfer-agent.yaml",
		".github/workflows/ci.yml",
		"docs/deployment/production.md",
	}

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(content), "v1.0.0") {
			t.Fatalf("expected %s to reference v1.0.0", file)
		}
	}
}

func TestProductionScriptsAreExecutable(t *testing.T) {
	files := []string{
		"deploy/linux/install.sh",
		"scripts/release.sh",
	}

	for _, file := range files {
		info, err := os.Stat(filepath.Join("..", file))
		if err != nil {
			t.Fatalf("stat %s: %v", file, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("expected %s to be executable", file)
		}
	}
}
