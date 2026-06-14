// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import "unicode"

// validSlug reports whether s is a valid slug: non-empty, ≤100 chars, contains
// only [A-Za-z0-9._-], and does not start with '-' or '.'.
func validSlug(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for i, r := range s {
		if i == 0 && (r == '-' || r == '.') {
			return false
		}
		if r != '-' && r != '.' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
