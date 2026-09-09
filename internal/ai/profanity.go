package ai

import (
	"regexp"
	"strings"
)

var speechWord = regexp.MustCompile(`[\p{L}]+`)

// Match common Russian profanity stems within whole words, including inflections.
// Keep ordinary words such as «страхуй», «рубля» and «употреблять» intact.
var profanityWord = regexp.MustCompile(`^(?:(?:на|по|за|ни|до|о|а|не)?ху[йеяиюё]|(?:по|на|за|вы|до|от|раз|под|при|про|пере|с|у|об|вз)?[её]б|(?:вы|на|за|рас|от|до|при)?пизд|бляд|блят|сука$|суки$|суку$|сукой$|сучар|сучк|мудак|мудач|мудил|гандон|говн|дерьм)`)

func censorProfanity(text string) string {
	return speechWord.ReplaceAllStringFunc(text, func(word string) string {
		lower := strings.ToLower(word)
		if lower != "бля" && !profanityWord.MatchString(lower) {
			return word
		}
		return string([]rune(word)[:1]) + "***"
	})
}
