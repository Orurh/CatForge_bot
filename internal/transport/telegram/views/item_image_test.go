package views

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"catforge/internal/assets"
	"catforge/internal/gamedata"
)

func TestEveryCatalogItemHasValidImage(t *testing.T) {
	items, err := gamedata.Items()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		t.Run(item.ID, func(t *testing.T) {
			url := ItemImageURL(item.ID)
			if url == "" {
				t.Fatal("missing item image")
			}
			data, err := assets.FS.ReadFile(strings.TrimPrefix(url, "/"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != cfg.Height || cfg.Width < 128 {
				t.Fatalf("invalid icon size: %+v", cfg)
			}
		})
	}
	for _, id := range []string{"", "missing", "../cats/bengal"} {
		if ItemImageURL(id) != "" {
			t.Fatalf("unexpected image for %q", id)
		}
	}
}
