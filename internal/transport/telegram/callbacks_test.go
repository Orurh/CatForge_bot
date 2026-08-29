package telegram

import "testing"

func TestPersonalCallbackRoundTrip(t *testing.T) {
	encoded := PersonalCallback(182, CBStarterPrefix+"bengal")
	owner, action, ok := ParsePersonalCallback(encoded)
	if !ok || owner != 182 || action != "starter:bengal" {
		t.Fatalf("ParsePersonalCallback(%q) = %d, %q, %v", encoded, owner, action, ok)
	}
}

func TestParsePersonalCallbackRejectsInvalidOwner(t *testing.T) {
	for _, value := range []string{"starter:bengal", "catui:0:menu:cat", "catui:nope:menu:cat", "catui:12:"} {
		if _, _, ok := ParsePersonalCallback(value); ok {
			t.Errorf("ParsePersonalCallback(%q) unexpectedly succeeded", value)
		}
	}
}

func TestActiveKeyboardsUseOwnerBoundCallbacks(t *testing.T) {
	keyboards := []map[string]any{
		StarterBreedKeyboard(42), MainMenuKeyboard(42), ProfileKeyboard(42), ResetConfirmKeyboard(42), TrainingKeyboard(42, true),
	}
	for _, keyboard := range keyboards {
		rows := keyboard["inline_keyboard"].([][]map[string]any)
		for _, row := range rows {
			for _, button := range row {
				callback, _ := button["callback_data"].(string)
				owner, _, ok := ParsePersonalCallback(callback)
				if !ok || owner != 42 {
					t.Errorf("active callback %q is not owner-bound", callback)
				}
			}
		}
	}
}
