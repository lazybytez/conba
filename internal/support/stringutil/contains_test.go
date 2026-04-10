package stringutil_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/support/stringutil"
)

func TestContainsAny_Match(t *testing.T) {
	t.Parallel()

	if !stringutil.ContainsAny("hello world", "world", "foo") {
		t.Error("want true when one substring matches")
	}
}

func TestContainsAny_NoMatch(t *testing.T) {
	t.Parallel()

	if stringutil.ContainsAny("hello world", "foo", "bar") {
		t.Error("want false when no substring matches")
	}
}

func TestContainsAny_Empty(t *testing.T) {
	t.Parallel()

	if stringutil.ContainsAny("hello world") {
		t.Error("want false with no substrings")
	}
}

func TestContainsAny_EmptyString(t *testing.T) {
	t.Parallel()

	if stringutil.ContainsAny("", "foo") {
		t.Error("want false when input is empty")
	}
}
