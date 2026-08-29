package views

import (
	"strings"
	"testing"

	"catforge/internal/domain"
)

func TestCatAvatarUsesExistingBaseAssetAtEveryLevel(t *testing.T) {
	t.Parallel()

	for _, level := range []int{1, 5, 10, 20, 100} {
		cat := &domain.Cat{Breed: domain.BreedBengal, Level: level}
		got := CatAvatarURL("https://example.test", cat)
		if !strings.Contains(got, "/bengal_base.png") {
			t.Fatalf("level %d URL = %q, want base asset", level, got)
		}
	}
}
