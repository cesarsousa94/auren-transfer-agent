package mediahub

import "testing"

func TestParseRegistrationResultAcceptsNestedMediaHubShapes(t *testing.T) {
	result, err := ParseRegistrationResult(map[string]any{
		"success": true,
		"node": map[string]any{
			"uuid":           "node-uuid",
			"agent_secret":   "node-secret",
			"config_version": "cfg-2",
		},
	})
	if err != nil {
		t.Fatalf("ParseRegistrationResult() error = %v", err)
	}
	if result.NodeUUID != "node-uuid" || result.NodeSecret != "node-secret" || result.ConfigVersion != "cfg-2" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
