package gamecontent

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"text/template"
)

//go:embed texts/*.json
var embeddedTexts embed.FS

type Catalog struct {
	variants map[string][]*template.Template
}

var defaultCatalog = mustLoad(embeddedTexts, "texts/*.json")

// Render picks a deterministic variant from the embedded Russian content and
// renders its optional Go-template placeholders with data.
func Render(key string, selector uint64, data any) string {
	return defaultCatalog.MustRender(key, selector, data)
}

// Has reports whether the default catalog contains a content key.
func Has(key string) bool {
	_, ok := defaultCatalog.variants[key]
	return ok
}

// Keys returns all default catalog keys in stable order. It is useful for
// content linting and editor tooling.
func Keys() []string {
	keys := make([]string, 0, len(defaultCatalog.variants))
	for key := range defaultCatalog.variants {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func Load(source fs.FS, pattern string) (*Catalog, error) {
	paths, err := fs.Glob(source, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob game content: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no game content files match %q", pattern)
	}
	sort.Strings(paths)

	catalog := &Catalog{variants: make(map[string][]*template.Template)}
	for _, path := range paths {
		payload, readErr := fs.ReadFile(source, path)
		if readErr != nil {
			return nil, fmt.Errorf("read game content %s: %w", path, readErr)
		}
		var groups map[string][]string
		if decodeErr := json.Unmarshal(payload, &groups); decodeErr != nil {
			return nil, fmt.Errorf("decode game content %s: %w", path, decodeErr)
		}
		for key, lines := range groups {
			key = strings.TrimSpace(key)
			if key == "" {
				return nil, fmt.Errorf("game content %s contains an empty key", path)
			}
			if _, duplicate := catalog.variants[key]; duplicate {
				return nil, fmt.Errorf("duplicate game content key %q in %s", key, path)
			}
			if len(lines) == 0 {
				return nil, fmt.Errorf("game content key %q has no variants", key)
			}
			compiled := make([]*template.Template, 0, len(lines))
			for index, line := range lines {
				if strings.TrimSpace(line) == "" {
					return nil, fmt.Errorf("game content key %q variant %d is empty", key, index)
				}
				tmpl, parseErr := template.New(key).Option("missingkey=error").Parse(line)
				if parseErr != nil {
					return nil, fmt.Errorf("parse game content key %q variant %d: %w", key, index, parseErr)
				}
				compiled = append(compiled, tmpl)
			}
			catalog.variants[key] = compiled
		}
	}
	return catalog, nil
}

func (c *Catalog) MustRender(key string, selector uint64, data any) string {
	variants, ok := c.variants[key]
	if !ok || len(variants) == 0 {
		panic("missing game content key: " + key)
	}
	selected := variants[selector%uint64(len(variants))]
	var rendered bytes.Buffer
	if err := selected.Execute(&rendered, data); err != nil {
		panic(fmt.Sprintf("render game content key %q: %v", key, err))
	}
	return rendered.String()
}

func mustLoad(source fs.FS, pattern string) *Catalog {
	catalog, err := Load(source, pattern)
	if err != nil {
		panic(err)
	}
	return catalog
}
