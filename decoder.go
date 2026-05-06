package main

import (
	"regexp"
	"strings"
)

// tableIntroducers matches SQL keywords that introduce a table name.
var tableIntroducers = regexp.MustCompile(
	`(?i)\b(FROM|JOIN|INNER\s+JOIN|LEFT\s+JOIN|RIGHT\s+JOIN|FULL\s+JOIN|CROSS\s+JOIN|UPDATE|INTO)\s+([a-zA-Z_][a-zA-Z0-9_]*)`,
)

// decodeSQL replaces known Sage X3 table abbreviations in query with their
// human-readable names. It returns the rewritten query and the distinct set of
// tables that were matched, in first-seen order.
func decodeSQL(query string, tables map[string]tableInfo) (string, []tableInfo) {
	seen := map[string]bool{}
	var found []tableInfo

	decoded := tableIntroducers.ReplaceAllStringFunc(query, func(match string) string {
		parts := tableIntroducers.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		keyword := parts[1]
		abbr := strings.ToUpper(parts[2])

		info, ok := tables[abbr]
		if !ok {
			return match
		}

		if !seen[abbr] {
			seen[abbr] = true
			found = append(found, info)
		}

		// Rewrite as "Full Name [ABBR]" — readable yet structurally intact.
		return keyword + " " + info.fullName + " [" + abbr + "]"
	})

	return decoded, found
}
