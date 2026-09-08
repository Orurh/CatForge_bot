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
		"fight.title",
		"fight.turn.attack",
		"fight.turn.crit",
		"fight.turn.finish",
		"fight.result.close",
		"fight.queue.waiting",
		"fight.queue.canceled",
		"fight.result.stats",
		"fight.revenge.available",
		"fight.revenge.accepted",
		"fight.revenge.expired",
		"button.fight.revenge",
		"yard_event.fish_truck.success",
		"yard_event.fish_truck.card.rules",
		"yard_event.big_dog.card.rules",
		"yard_event.big_box.card.rules",
		"yard_event.big_dog.result.headline.success",
		"yard_event.big_dog.result.headline.partial",
		"yard_event.big_dog.result.headline.exceptional",
		"yard_event.big_box.result.headline.success",
		"yard_event.result.participant",
		"yard_event.result.reward_note",
		"fallback.first_line.bully",
		"fallback.arena_banter.first",
		"fallback.arena_banter.revenge",
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
