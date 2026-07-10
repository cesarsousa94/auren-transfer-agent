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
		"docker/entrypoint.sh",
		"deploy/linux/install.sh",
		"deploy/systemd/auren-transfer-agent.service",
		"deploy/systemd/auren-transfer-agent.env.example",
		"deploy/debian/DEBIAN/postinst",
		"deploy/debian/DEBIAN/prerm",
		"deploy/debian/DEBIAN/postrm",
		"deploy/debian/DEBIAN/conffiles",
		"deploy/kubernetes/auren-transfer-agent.yaml",
		".github/workflows/ci.yml",
		"scripts/release.sh",
		"scripts/build-deb.sh",
		"scripts/build-apt-repo.sh",
		"scripts/publish-apt-s3.sh",
		"scripts/export-apt-gpg-key.sh",
		"scripts/generate-install-command.sh",
		"docs/deployment/production.md",
		"docs/deployment/linux-package-bootstrap.md",
		"docs/deployment/apt-repository.md",
		"docs/deployment/media-hub-install-command.md",
		"docs/deployment/docker-auto-bootstrap.md",
		"docs/deployment/docker-permission-hotfix.md",
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
		"docs/deployment/linux-package-bootstrap.md",
		"docs/deployment/apt-repository.md",
		"docs/deployment/media-hub-install-command.md",
		"docs/deployment/docker-auto-bootstrap.md",
		"docs/deployment/docker-permission-hotfix.md",
		"scripts/release.sh",
		"scripts/build-deb.sh",
		"scripts/export-apt-gpg-key.sh",
		"scripts/generate-install-command.sh",
		"docker/entrypoint.sh",
	}

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(content), "v1.13.1") && !strings.Contains(string(content), "1.13.1") {
			t.Fatalf("expected %s to reference v1.13.1", file)
		}
	}
}

func TestProductionScriptsAreExecutable(t *testing.T) {
	files := []string{
		"deploy/linux/install.sh",
		"scripts/release.sh",
		"scripts/build-deb.sh",
		"scripts/build-apt-repo.sh",
		"scripts/publish-apt-s3.sh",
		"scripts/export-apt-gpg-key.sh",
		"scripts/generate-install-command.sh",
		"docker/entrypoint.sh",
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

func TestSystemdRunsServeSubcommand(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "deploy/systemd/auren-transfer-agent.service"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "/usr/bin/auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml") {
		t.Fatalf("systemd unit must run the serve subcommand")
	}
	if !strings.Contains(text, "User=auren-agent") {
		t.Fatalf("systemd unit must use the canonical auren-agent user")
	}
}

func TestDebianMaintainerScriptsArePackagerNormalized(t *testing.T) {
	files := []string{
		"deploy/debian/DEBIAN/postinst",
		"deploy/debian/DEBIAN/prerm",
		"deploy/debian/DEBIAN/postrm",
	}

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.HasPrefix(string(content), "#!/") {
			t.Fatalf("expected %s to have a shebang", file)
		}
	}

	buildDeb, err := os.ReadFile(filepath.Join("..", "scripts/build-deb.sh"))
	if err != nil {
		t.Fatalf("read build-deb.sh: %v", err)
	}
	text := string(buildDeb)
	for _, script := range []string{"postinst", "prerm", "postrm"} {
		if !strings.Contains(text, "${ROOT}/DEBIAN/"+script) || !strings.Contains(text, "chmod 0755") {
			t.Fatalf("build-deb.sh must normalize executable mode for %s", script)
		}
	}
}

func TestDockerEntrypointAutoBootstrapContract(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "docker/entrypoint.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"AUREN_DOCKER_AUTO_BOOTSTRAP",
		"AUREN_DOCKER_FORCE_BOOTSTRAP",
		"AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT",
		"/api/internal/nodes/bootstrap-token",
		"node.json",
		"auren-transfer-agent bootstrap",
		"auren-transfer-agent serve",
		"--log-dir",
		"AUREN_AGENT_LOG_DIR",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("docker entrypoint must include %s", expected)
		}
	}
}

func TestDockerComposeUsesRootEnvFileAndAutoBootstrap(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "docker", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"env_file:",
		"../.env",
		"AUREN_DOCKER_AUTO_BOOTSTRAP",
		"AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT",
		"auren-transfer-agent-data",
		"AUREN_AGENT_LOG_DIR",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("docker compose must include %s", expected)
		}
	}
}
