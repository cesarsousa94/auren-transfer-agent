package mediahub

import "testing"

func TestParseGatewayResolveResultAcceptsNestedShapes(t *testing.T) {
	result, err := ParseGatewayResolveResult(map[string]any{
		"success": true,
		"data": map[string]any{
			"delivery_mode": "redirect",
			"url":           "https://cdn.example.test/a.ts",
			"session_uuid":  "sess_1",
		},
	})
	if err != nil {
		t.Fatalf("ParseGatewayResolveResult() error = %v", err)
	}
	if result.NormalizedMode() != "redirect" || result.UpstreamURL != "https://cdn.example.test/a.ts" || result.SessionID != "sess_1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGatewayResolveValidateRejectsMissingUpstream(t *testing.T) {
	if err := (GatewayResolveResult{Mode: "proxy"}).Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing upstream error")
	}
}
