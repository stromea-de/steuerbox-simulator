package inspection

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// Review 037d/038 P2: a real report with an unconfirmed failsafe return must
// satisfy docs/schema/pruefbericht-v1.json (limitChange has additionalProperties:false).
func Test038UnconfirmedLimitReportMatchesSchema(t *testing.T) {
	f := &onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{100, 100}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	unconfirmed := false
	for _, c := range r.Limits {
		unconfirmed = unconfirmed || c.Unconfirmed
	}
	if !unconfirmed {
		t.Fatal("fixture must produce an unconfirmed return")
	}
	raw, err := os.ReadFile("../../docs/schema/pruefbericht-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties           map[string]any `json:"properties"`
			Required             []string       `json:"required"`
			AdditionalProperties *bool          `json:"additionalProperties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	def := schema.Defs["limitChange"]
	data, _ := json.Marshal(r)
	var report struct {
		Limits []map[string]any `json:"limits"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for i, entry := range report.Limits {
		for key, value := range entry {
			prop, ok := def.Properties[key].(map[string]any)
			if !ok {
				t.Fatalf("limits[%d].%s not allowed by schema", i, key)
			}
			if prop["type"] == "boolean" {
				if _, isBool := value.(bool); !isBool {
					t.Fatalf("limits[%d].%s must be boolean", i, key)
				}
			}
		}
		for _, key := range def.Required {
			if _, ok := entry[key]; !ok {
				t.Fatalf("limits[%d] lacks required %s", i, key)
			}
		}
	}
}
