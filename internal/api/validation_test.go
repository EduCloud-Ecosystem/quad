// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"strings"
	"testing"
)

func TestValidSlug(t *testing.T) {
	ok := []string{
		"cs101", "hw-1", "hw.1", "hw_1", "a", "A1", "foo-bar",
		strings.Repeat("a", 100), // exactly 100 chars
	}
	bad := []string{
		"",
		"-leading-dash",
		".leading-dot",
		"has space",
		"has<angle>",
		"<operator-username>",    // literal placeholder
		strings.Repeat("a", 101), // 101 chars
	}
	for _, s := range ok {
		if !validSlug(s) {
			t.Errorf("validSlug(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if validSlug(s) {
			t.Errorf("validSlug(%q) = true, want false", s)
		}
	}
}
