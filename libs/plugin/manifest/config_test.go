package manifest

import (
	"encoding/json"
	"testing"
	"uuid"
)

func TestGenerateIDs(t *testing.T) {
	m := valid()
	m.SetConfigSchema(`{"type":"object","properties":{"indexers":{"type":"array","items":{"type":"object","properties":{` +
		`"id":{"type":"string","format":"uuid","readOnly":true},"name":{"type":"string"},"seed_ratio":{"type":"number"}}}},` +
		`"other":{"type":"string","format":"uuid"}}}`)
	got, err := GenerateIDs(m, `{"indexers":[{"name":"a","seed_ratio":1.50},{"id":"","name":"b"},{"id":"kept","name":"c"}],"other":""}`)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Indexers []struct {
			ID        string      `json:"id"`
			SeedRatio json.Number `json:"seed_ratio"`
		} `json:"indexers"`
		Other string `json:"other"`
	}
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatal(err)
	}
	for i, ix := range doc.Indexers[:2] {
		if _, err := uuid.Parse(ix.ID); err != nil {
			t.Errorf("indexers[%d].id = %q, want = a UUID", i, ix.ID)
		}
	}
	if doc.Indexers[0].ID == doc.Indexers[1].ID {
		t.Errorf("generated IDs are equal: %q", doc.Indexers[0].ID)
	}
	if doc.Indexers[2].ID != "kept" {
		t.Errorf("indexers[2].id = %q, want = kept", doc.Indexers[2].ID)
	}
	if doc.Indexers[0].SeedRatio != "1.50" {
		t.Errorf("seed_ratio = %s, want = 1.50", doc.Indexers[0].SeedRatio)
	}
	if doc.Other != "" {
		t.Errorf("other = %q, want it left empty: it is not readOnly", doc.Other)
	}

	const unchanged = `{"indexers": [{"id": "x"}]}`
	if got, err := GenerateIDs(m, unchanged); err != nil || got != unchanged {
		t.Errorf("GenerateIDs() = %s, %v, want = the configuration as it is", got, err)
	}
	if _, err := GenerateIDs(m, `{`); err == nil {
		t.Error("GenerateIDs() of invalid JSON err = nil, want = an error")
	}
}
