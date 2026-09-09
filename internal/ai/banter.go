package ai

import (
	"strings"
	"unicode/utf8"
)

const maxBanterLineRunes = 220

func ParseArenaBanterLines(value string) ([2]string, bool) {
	var result [2]string
	raw := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	lines := make([]string, 0, 2)
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > maxBanterLineRunes {
			return result, false
		}
		lines = append(lines, line)
	}
	if len(lines) != 2 {
		return result, false
	}
	copy(result[:], lines)
	return result, true
}
