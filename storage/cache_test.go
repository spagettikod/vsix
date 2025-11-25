package storage

import "testing"

func TestFtsArgs(t *testing.T) {
	tests := []string{
		"",
		" ",
		"abc",
		"lang ms-",
		"golang.go",
	}
	expected := []string{
		"",
		"",
		"abc*",
		"lang* \"ms-*\"",
		"\"golang.go*\"",
	}

	for i, test := range tests {
		actual := ftsArgs(test)
		if actual != expected[i] {
			t.Errorf("expected %s but got %s", expected[i], actual)
		}
	}
}
