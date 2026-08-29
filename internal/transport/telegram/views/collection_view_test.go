package views

import (
	"strings"
	"testing"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/gamedata"
)

func TestCollectionAndItemViews(t *testing.T) {
	t.Parallel()
	definition, ok := gamedata.ItemByID("rat_tooth")
	if !ok {
		t.Fatal("rat_tooth missing from catalog")
	}
	entry := app.CollectionEntry{Definition: definition, Owned: domain.OwnedItem{ItemID: definition.ID, Level: 2, Fragments: 3, Equipped: true}}
	collection := FormatCollection([]app.CollectionEntry{entry})
	detail := FormatItem(entry, "")
	profile := FormatProfileEquipment([]app.CollectionEntry{entry})
	for text, want := range map[string]string{collection: "✅ Крысиный зуб", detail: "ATK +4", profile: "Крысиный зуб"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}
