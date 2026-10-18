package game

import "strings"

// The level catalogue spells a line break in a title or description as the two characters backslash and n
// ("Bonus Chapter\nWestern: Level 6-1"); the screens that draw them break the line there.

// levelTextLines returns text with the catalogue's backslash-n breaks turned into real line breaks.
func levelTextLines(text string) string { return strings.ReplaceAll(text, "\\n", "\n") }

// levelTextFlat returns text on one line, the break replaced by sep (for titles, presence and labels).
func levelTextFlat(text, sep string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\\n", sep), "\n", sep)
}
