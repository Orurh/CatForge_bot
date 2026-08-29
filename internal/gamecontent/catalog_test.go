package gamecontent

import (
	"testing"
	"testing/fstest"
)

func TestDefaultCatalogLoadsRequiredProceduralGroups(t *testing.T) {
	required := []string{
		"training.intro",
		"training.encounter.big_rat",
		"fight.start",
		"fight.result.close",
		"fight.queue.waiting",
		"fight.queue.canceled",
		"fight.result.stats",
		"yard_event.fish_truck.success",
		"fallback.first_line.bully",
		"legacy_expedition.outcome.victory",
	}
	for _, key := range required {
		if !Has(key) {
			t.Errorf("missing required content key %q", key)
		}
	}
}

func TestRenderIsDeterministicAndSupportsTemplates(t *testing.T) {
	first := Render("fight.start", 7, map[string]string{"AttackerName": "Барсик", "DefenderName": "Батон"})
	second := Render("fight.start", 7, map[string]string{"AttackerName": "Барсик", "DefenderName": "Батон"})
	if first != second {
		t.Fatalf("same selector rendered different variants: %q != %q", first, second)
	}
	if first == "" {
		t.Fatal("rendered fight line is empty")
	}
}

func TestLoadRejectsDuplicateKeysAcrossFiles(t *testing.T) {
	files := fstest.MapFS{
		"a.json": {Data: []byte(`{"same":["one"]}`)},
		"b.json": {Data: []byte(`{"same":["two"]}`)},
	}
	if _, err := Load(files, "*.json"); err == nil {
		t.Fatal("Load() accepted a duplicate key")
	}
}
