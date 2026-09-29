package docs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSpecBuilds(t *testing.T) {
	doc := Spec()
	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI = %q, want 3.1.0", doc.OpenAPI)
	}
	n := doc.Paths.Len()
	if n == 0 {
		t.Fatal("no paths")
	}
	t.Logf("paths = %d", n)
	for _, k := range doc.Paths.Keys() {
		t.Logf("  %s", k)
	}
	raw := JSON()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(string(raw), `"openapi":"3.1.0"`) {
		t.Fatalf("raw json missing 3.1.0 marker; head=%s", string(raw[:200]))
	}
	t.Logf("spec bytes = %d, schemas = %d", len(raw), len(doc.Components.Schemas))
}
