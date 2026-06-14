// SPDX-License-Identifier: AGPL-3.0-or-later

// Package id generates opaque random identifiers used for primary keys.
package id

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a 128-bit random identifier as a hex string.
func New() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
