package plugins

import "testing"

func TestNormalizeManifest(t *testing.T) {
	manifest, err := NormalizeManifest(Manifest{Name: " local ", Kind: " UPLOADER ", Version: " 1.0.0 ", Capabilities: []string{"upload"}, Metadata: map[string]string{"driver": "local"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest.Name != "local" || manifest.Kind != KindUploader || manifest.Version != "1.0.0" {
		t.Fatalf("manifest was not normalized: %+v", manifest)
	}
	manifest.Capabilities[0] = "changed"
	manifest.Metadata["driver"] = "changed"
	clone := manifest.Clone()
	clone.Capabilities[0] = "clone"
	clone.Metadata["driver"] = "clone"
	if manifest.Capabilities[0] != "changed" || manifest.Metadata["driver"] != "changed" {
		t.Fatalf("clone mutated source")
	}
}

func TestValidateManifestRejectsInvalidKind(t *testing.T) {
	if err := ValidateManifest(Manifest{Name: "x", Kind: "bad", Version: "1"}); err == nil {
		t.Fatal("expected invalid kind error")
	}
}
