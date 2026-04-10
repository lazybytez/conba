// Package stringutil provides string matching utilities.
package stringutil

import "strings"

// ContainsAny reports whether s contains any of the given substrings.
func ContainsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}
